package controller

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/haproxytech/models"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

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

type Controller struct {
	lister     cache.Indexer
	controller cache.Controller
	queue      workqueue.RateLimitingInterface
	handler    haproxy.HaproxyHandle
}

func newController(lister cache.Indexer, controller cache.Controller, queue workqueue.RateLimitingInterface) *Controller {
	return &Controller{
		lister:     lister,
		controller: controller,
		queue:      queue,
		handler:    haproxy.NewHaproxyHandle("admin", "1fc917c7ad66487470e466c0ad40ddd45b9f7730a4b43e1b2542627f0596bbdc", "http://10.0.0.3:5555"),
	}
}

func (c *Controller) changeProxy(svrInt server) (encounterError error, b bool) {
	sIns := c.handler.GetServers(BACKEND_PREFIX)
	newServerIns := &haproxy.Server{
		Name:    svrInt.serverName,
		Address: svrInt.podAddr,
	}
	if len(sIns) > 0 {
		for _, v := range sIns {
			b, encounterError = c.handler.ReplaceServerFromBackend(v.Name, newServerIns, BACKEND_PREFIX)
		}
	}
	if encounterError == nil {
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
	checkTimeout := int64(1)
	bindPort := int64(8849)
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
					Address: "0.0.0.0",
				}, proxyInts.frontendName)
			}
		}
	}
	return
}

func (c *Controller) initHaproxy() (error, bool) {
	return c.createProxy()
}

func (c *Controller) CreateTask(name string) error {
	fmt.Println(name)
	item, exists, err := c.lister.GetByKey(name)
	if err != nil {
		return err
	}
	if !exists {
		klog.Info(fmt.Sprintf("item %v not exists in cache.", item))
		return fmt.Errorf("%s not fount.", name)
	}
	pod := item.(*v1.Pod)
	srv := server{
		serverName: SERVER_PREFIX + pod.Name + "." + pod.Status.PodIP,
		podAddr:    pod.Status.PodIP + ":80",
	}
	error, _ := c.changeProxy(srv)
	if error != nil {
		klog.Error(error)
		return error
	}
	return nil
}

func (c *Controller) handleError(key string) {

	if c.queue.NumRequeues(key) < 3 {
		c.queue.AddRateLimited(key)
		return
	}
	c.queue.Forget(key)
	klog.Infof("Drop Object %s in queue", key)
}

func (c *Controller) Run(threadiness int, stopCh chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	klog.Infof("Starting pod proxy handler controller.")

	go c.controller.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.controller.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("sync failed."))
		return
	}
	e, _ := c.initHaproxy()
	if e != nil && !strings.Contains(e.Error(), "already exists") {
		fmt.Println(strings.Contains("already exists", e.Error()))
		panic(e)
	}
	<-stopCh
	klog.Info("Stopping pod proxy handler controller.")
}

func RunController() *Controller {
	var (
		k8sconfig  *string //使用kubeconfig配置文件进行集群权限认证
		restConfig *rest.Config
		err        error
	)
	if home := homedir.HomeDir(); home != "" {
		k8sconfig = flag.String("kubeconfig", fmt.Sprintf("./admin.conf"), "kubernetes auth config")
	}
	k8sconfig = k8sconfig
	flag.Parse()
	if _, err := os.Stat(*k8sconfig); err != nil {
		panic(err)
	}

	if restConfig, err = rest.InClusterConfig(); err != nil {
		// 这里是从masterUrl 或者 kubeconfig传入集群的信息，两者选一
		restConfig, err = clientcmd.BuildConfigFromFlags("", *k8sconfig)
		if err != nil {
			panic(err)
		}
	}
	restset, err := kubernetes.NewForConfig(restConfig)
	lister := cache.NewListWatchFromClient(restset.CoreV1().RESTClient(), "pods", v1.NamespaceAll, fields.Everything())
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	indexer, controller := cache.NewIndexerInformer(lister, &v1.Pod{}, time.Minute, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			//fmt.Println("add ", obj.(*v1.Pod).GetName())
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}

		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			//fmt.Println("update", newObj.(*v1.Pod).GetName())
			if newObj.(*v1.Pod).Status.Conditions[0].Status == "True" {
				fmt.Println("update: the Initialized Status", newObj.(*v1.Pod).Status.Conditions[0].Status)
			} else {
				fmt.Println("update: the Initialized Status ", newObj.(*v1.Pod).Status.Conditions[0].Status)
				fmt.Println("update: the Initialized Reason ", newObj.(*v1.Pod).Status.Conditions[0].Reason)
			}

			if len(newObj.(*v1.Pod).Status.Conditions) > 1 {
				if newObj.(*v1.Pod).Status.Conditions[1].Status == "True" {
					fmt.Println("update: the Ready Status", newObj.(*v1.Pod).Status.Conditions[1].Status)
				} else {
					fmt.Println("update: the Ready Status ", newObj.(*v1.Pod).Status.Conditions[1].Status)
					fmt.Println("update: the Ready Reason ", newObj.(*v1.Pod).Status.Conditions[1].Reason)
				}

				if newObj.(*v1.Pod).Status.Conditions[2].Status == "True" {
					fmt.Println("update: the PodCondition Status", newObj.(*v1.Pod).Status.Conditions[2].Status)
				} else {
					fmt.Println("update: the PodCondition Status ", newObj.(*v1.Pod).Status.Conditions[2].Status)
					fmt.Println("update: the PodCondition Reason ", newObj.(*v1.Pod).Status.Conditions[2].Reason)
				}

				if newObj.(*v1.Pod).Status.Conditions[3].Status == "True" {
					fmt.Println("update: the PodScheduled Status", newObj.(*v1.Pod).Status.Conditions[3].Status)
				} else {
					fmt.Println("update: the PodScheduled Status ", newObj.(*v1.Pod).Status.Conditions[3].Status)
					fmt.Println("update: the PodScheduled Reason ", newObj.(*v1.Pod).Status.Conditions[3].Reason)
				}
			}

		},
		DeleteFunc: func(obj interface{}) {
			//fmt.Println("delete ", obj.(*v1.Pod).GetName(), "Status ", obj.(*v1.Pod).Status.Phase)
			// 上面是事件函数的处理，下面是对workqueue的操作
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
	}, cache.Indexers{})
	return newController(indexer, controller, queue)
}
