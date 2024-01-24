package controller

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/haproxytech/models"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"pod-proxier/haproxy"
)

type proxy struct {
	backendName, frontendName, serverName, bindName string
	podAddr                                         string
	mode                                            string
}

type server struct {
	serverName string
	podAddr    string
	mode       string
}

func (p *Controller) RunAsyncProcess() {
	klog.V(2).Info("Start asynchronous processing scheduled mapping.")
	defer func() {
		p.asyncQueue.ShutDown()
	}()
	p.asyncWg.Start(p.pop)
	p.asyncWg.Wait()
}

func (p *Controller) AddAfter(t time.Duration, event Event) {
	klog.V(2).Infof("Push server %s:%d to stack.", event.Host, event.Port)
	p.asyncQueue.AddAfter(event, t)
}

func (p *Controller) pop() {
	klog.V(2).Info("Async event processor started, waitting task...")
	for {
		select {
		case <-p.asyncStopCh:
			klog.V(2).Info("Async evnet process exit.")
			return
		default:
			_, quit := p.asyncQueue.Get()
			if quit {
				return
			}
			p.DefaultMapping()
		}
	}
}

type Controller struct {
	lister     cache.Indexer
	controller cache.Controller
	// controller的队列
	queue         workqueue.RateLimitingInterface
	handler       haproxy.HaproxyHandle
	stopCh        chan struct{}
	listenersLock sync.RWMutex
	wg            wait.Group

	asyncStopCh        chan struct{}
	asyncListenersLock sync.RWMutex
	asyncWg            wait.Group
	asyncQueue         workqueue.RateLimitingInterface

	// 延迟队列
	aferqueue      workqueue.RateLimitingInterface
	controllerAddr string
	controllerPort int

	// pod spec
	DefaultMappingPort int
	JprofilerPortName  string
	MaxMappingTime     int
}

func newController(lister cache.Indexer,
	controller cache.Controller,
	queue workqueue.RateLimitingInterface,
	stopCh chan struct{},
	listenPort int,
	listenAddr string,
	dataplanHost, dataplanUser, dataplanPassword string,
	defaultMappPort int,
	jprofilerPortName string,
	maxMapTime int,
) *Controller {
	return &Controller{
		lister:             lister,
		controller:         controller,
		queue:              queue,
		handler:            haproxy.NewHaproxyHandle(dataplanUser, dataplanPassword, dataplanHost),
		aferqueue:          workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		controllerAddr:     listenAddr,
		controllerPort:     listenPort,
		stopCh:             stopCh,
		asyncStopCh:        stopCh,
		asyncQueue:         workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		DefaultMappingPort: defaultMappPort,
		JprofilerPortName:  jprofilerPortName,
		MaxMappingTime:     maxMapTime,
	}
}

func (c *Controller) defaultBackend() (encounterError error) {
	sIns := c.handler.GetServers(BACKEND_PREFIX)
	newServerIns := &haproxy.Server{
		Name:    "default",
		Address: "127.0.0.1:22",
		Check:   "enabled",
	}
	if len(sIns) > 0 {
		for _, v := range sIns {
			_, encounterError = c.handler.ReplaceServerFromBackend(v.Name, newServerIns, BACKEND_PREFIX)
		}
	} else {
		_, encounterError = c.handler.AddServerToBackend(newServerIns, BACKEND_PREFIX)

	}
	return
}

func (c *Controller) changeProxy(svrInt server) (encounterError error, b bool) {
	sIns := c.handler.GetServers(BACKEND_PREFIX)
	newServerIns := &haproxy.Server{
		Name:    svrInt.serverName,
		Address: svrInt.podAddr,
		Check:   "enabled",
	}
	if len(sIns) > 0 {
		for _, v := range sIns {
			b, encounterError = c.handler.ReplaceServerFromBackend(v.Name, newServerIns, BACKEND_PREFIX)
		}
	} else {
		b, encounterError = c.handler.AddServerToBackend(newServerIns, BACKEND_PREFIX)
	}
	return
}

func (c *Controller) createProxy() (encounterError error, b bool) {
	proxyInts := proxy{
		backendName:  BACKEND_PREFIX,
		frontendName: FRONTEND_PREFIX,
		bindName:     BIND_PREFIX,
		mode:         "tcp",
	}
	checkTimeout := int64(15)
	bindPort := int64(c.controllerPort)
	b, encounterError = c.handler.AddBackend(&models.Backend{
		Name:         proxyInts.backendName,
		Mode:         proxyInts.mode,
		CheckTimeout: &checkTimeout,
	})
	if encounterError == nil || !strings.Contains(encounterError.Error(), "already exists") {
		if encounterError == nil {
			b, encounterError = c.handler.AddFrontend(&models.Frontend{
				Name:           proxyInts.frontendName,
				DefaultBackend: proxyInts.backendName,
				Mode:           proxyInts.mode,
			})
			if encounterError == nil || !strings.Contains(encounterError.Error(), "already exists") {
				b, encounterError = c.handler.AddBind(&models.Bind{
					Name:    proxyInts.bindName,
					Port:    &bindPort,
					Address: c.controllerAddr,
				}, proxyInts.frontendName)
			}
		}
	}
	return
}

func (c *Controller) initHaproxy() (error, bool) {
	return c.createProxy()
}

func (c *Controller) QueryPodExists(name string) bool {
	_, exists, err := c.lister.GetByKey(name)
	if !exists || err != nil {
		klog.Warningf("item %v not exists in cache.", name)
		return false
	}
	return true
}

func (c *Controller) CreateMapping(name string, delayTime time.Duration) error {
	item, exists, err := c.lister.GetByKey(name)
	if err != nil {
		return err
	}
	if !exists {
		klog.Warningf("item %v not exists in cache.", name)
		return fmt.Errorf("%s not fount.", name)
	}
	pod := item.(*v1.Pod)
	var port int
	for _, container := range pod.Spec.Containers {
		for _, p := range container.Ports {
			if p.Name == c.JprofilerPortName {
				port = int(p.ContainerPort)
			}
		}
	}
	if port == 0 {
		port = c.DefaultMappingPort
	}
	srv := server{
		serverName: SERVER_PREFIX + pod.Name + "." + pod.Status.PodIP,
		podAddr:    fmt.Sprintf("%s:%d", pod.Status.PodIP, port),
	}
	error, _ := c.changeProxy(srv)
	if error != nil {
		klog.Error(error)
		return error
	}
	c.AddAfter(delayTime*time.Second, Event{
		Port: port,
		Host: name,
	})
	return nil
}

func (c *Controller) DefaultMapping() error {
	srv := server{
		serverName: APP_NAME_PREFIX + "pod-proxier",
		podAddr:    fmt.Sprintf("%s:%d", c.controllerAddr, c.controllerPort),
	}
	error, _ := c.changeProxy(srv)
	if error != nil {
		klog.Error(error)
		return error
	}
	return nil
}

func (c *Controller) Run() {

	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	klog.V(2).Infof("Starting pod proxy handler controller.")

	go c.controller.Run(c.stopCh)
	go c.RunAsyncProcess()

	if !cache.WaitForCacheSync(c.stopCh, c.controller.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("sync failed."))
		return
	}
	e, _ := c.initHaproxy()
	if e != nil {
		if strings.Contains(e.Error(), "already exists") {
			klog.Warningf("Inital failed: %s", e.Error())
		} else {
			panic(e)
		}
	}
	<-c.stopCh
	klog.V(2).Info("Stopping pod proxy handler controller.")
}

func RunController(
	stopCh chan struct{},
	kubeconfig string,
	listenAddr string,
	listenPort int,
	dataplanHost, dataplanUser, dataplanPassword string,
	defaultMappPort int,
	jprofilerPortName string,
	maxMapTime int,
	resyncTime int,
) *Controller {
	var (
		restConfig *rest.Config
		err        error
	)

	if _, err := os.Stat(kubeconfig); err != nil {
		klog.V(2).Infof("%s, tryting in-cluster mode.", err.Error())
	}

	if restConfig, err = rest.InClusterConfig(); err != nil {
		// 这里是从masterUrl 或者 kubeconfig传入集群的信息，两者选一
		restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err)
		}
	}
	restset, err := kubernetes.NewForConfig(restConfig)
	lister := cache.NewListWatchFromClient(restset.CoreV1().RESTClient(), "pods", v1.NamespaceAll, fields.Everything())
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	indexer, controller := cache.NewIndexerInformer(
		lister,
		&v1.Pod{},
		time.Duration(resyncTime)*time.Second,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) {},
			UpdateFunc: func(oldObj, newObj interface{}) {},
			DeleteFunc: func(obj interface{}) {},
		}, cache.Indexers{})
	return newController(
		indexer,
		controller,
		queue,
		stopCh,
		listenPort,
		listenAddr,
		dataplanHost, dataplanUser, dataplanPassword,
		defaultMappPort,
		jprofilerPortName,
		maxMapTime,
	)
}
