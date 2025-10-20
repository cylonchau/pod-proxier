package controller

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/haproxytech/models"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type ServiceMapping struct {
	ServiceName  string
	Namespace    string
	ServicePort  int32
	PortName     string
	MappedPort   int
	BackendName  string
	FrontendName string
	BindName     string
	ClusterIP    string
	CreatedAt    time.Time
}

type ServiceController struct {
	serviceLister     cache.Indexer
	serviceController cache.Controller

	queue         workqueue.RateLimitingInterface
	handler       haproxy.HaproxyHandle
	stopCh        chan struct{}
	listenersLock sync.RWMutex
	wg            wait.Group

	controllerAddr string
	controllerPort int

	// 端口范围管理
	portRangeStart    int
	portRangeEnd      int
	allowedNamespaces map[string]bool
	targetPortNames   map[string]bool // 要监听的端口名称

	// 服务映射管理
	serviceMappings    map[string]*ServiceMapping // key: "namespace/serviceName/portName"
	portAllocations    map[int]bool               // key: port, value: allocated
	portAllocationLock sync.RWMutex

	clientset *kubernetes.Clientset
}

func newServiceController(
	serviceLister cache.Indexer,
	serviceController cache.Controller,
	queue workqueue.RateLimitingInterface,
	stopCh chan struct{},
	listenPort int,
	listenAddr string,
	dataplanHost, dataplanUser, dataplanPassword string,
	portRangeStart, portRangeEnd int,
	allowedNamespaces []string,
	targetPortNames []string,
	clientset *kubernetes.Clientset,
) *ServiceController {

	allowedNsMap := make(map[string]bool)
	for _, ns := range allowedNamespaces {
		allowedNsMap[ns] = true
	}

	targetPortNamesMap := make(map[string]bool)
	for _, portName := range targetPortNames {
		targetPortNamesMap[portName] = true
	}

	return &ServiceController{
		serviceLister:     serviceLister,
		serviceController: serviceController,
		queue:             queue,
		handler:           haproxy.NewHaproxyHandle(dataplanUser, dataplanPassword, dataplanHost),
		controllerAddr:    listenAddr,
		controllerPort:    listenPort,
		stopCh:            stopCh,
		portRangeStart:    portRangeStart,
		portRangeEnd:      portRangeEnd,
		allowedNamespaces: allowedNsMap,
		targetPortNames:   targetPortNamesMap,
		serviceMappings:   make(map[string]*ServiceMapping),
		portAllocations:   make(map[int]bool),
		clientset:         clientset,
	}
}

// 分配可用端口
func (c *ServiceController) allocatePort() (int, error) {
	c.portAllocationLock.Lock()
	defer c.portAllocationLock.Unlock()

	for port := c.portRangeStart; port <= c.portRangeEnd; port++ {
		if !c.portAllocations[port] {
			c.portAllocations[port] = true
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports in range %d-%d", c.portRangeStart, c.portRangeEnd)
}

// 释放端口
func (c *ServiceController) releasePort(port int) {
	c.portAllocationLock.Lock()
	defer c.portAllocationLock.Unlock()
	c.portAllocations[port] = false
}

// 检查命名空间是否允许
func (c *ServiceController) isNamespaceAllowed(namespace string) bool {
	return c.allowedNamespaces[namespace]
}

// 检查端口名称是否匹配
func (c *ServiceController) isTargetPortName(portName string) bool {
	if len(c.targetPortNames) == 0 {
		return true // 如果没有指定端口名称，则匹配所有端口
	}
	return c.targetPortNames[portName]
}

// 处理Service添加事件
func (c *ServiceController) handleServiceAdd(obj interface{}) {
	service := obj.(*corev1.Service)

	if !c.isNamespaceAllowed(service.Namespace) {
		return
	}

	klog.V(2).Infof("Service added: %s/%s", service.Namespace, service.Name)
	c.processService(service)
}

// 处理Service更新事件
func (c *ServiceController) handleServiceUpdate(oldObj, newObj interface{}) {
	oldService := oldObj.(*corev1.Service)
	newService := newObj.(*corev1.Service)

	if !c.isNamespaceAllowed(newService.Namespace) {
		return
	}

	// 如果ClusterIP或端口发生变化，需要重新处理
	if oldService.Spec.ClusterIP != newService.Spec.ClusterIP ||
		!c.compareServicePorts(oldService.Spec.Ports, newService.Spec.Ports) {
		klog.V(2).Infof("Service updated: %s/%s", newService.Namespace, newService.Name)
		c.removeServiceMappings(oldService)
		c.processService(newService)
	}
}

// 处理Service删除事件
func (c *ServiceController) handleServiceDelete(obj interface{}) {
	service := obj.(*corev1.Service)

	if !c.isNamespaceAllowed(service.Namespace) {
		return
	}

	klog.V(2).Infof("Service deleted: %s/%s", service.Namespace, service.Name)
	c.removeServiceMappings(service)
}

// 比较服务端口是否相同
func (c *ServiceController) compareServicePorts(oldPorts, newPorts []corev1.ServicePort) bool {
	if len(oldPorts) != len(newPorts) {
		return false
	}

	for i, oldPort := range oldPorts {
		newPort := newPorts[i]
		if oldPort.Name != newPort.Name || oldPort.Port != newPort.Port || oldPort.Protocol != newPort.Protocol {
			return false
		}
	}
	return true
}

// 处理单个服务
func (c *ServiceController) processService(service *corev1.Service) {
	c.listenersLock.Lock()
	defer c.listenersLock.Unlock()

	// 跳过没有ClusterIP的服务
	if service.Spec.ClusterIP == "" || service.Spec.ClusterIP == "None" {
		return
	}

	for _, port := range service.Spec.Ports {
		// 检查端口名称是否匹配
		if !c.isTargetPortName(port.Name) {
			continue
		}

		mappingKey := fmt.Sprintf("%s/%s/%s", service.Namespace, service.Name, port.Name)

		// 检查是否已存在映射
		if _, exists := c.serviceMappings[mappingKey]; exists {
			continue
		}

		// 分配端口
		mappedPort, err := c.allocatePort()
		if err != nil {
			klog.Errorf("Failed to allocate port for %s: %v", mappingKey, err)
			continue
		}

		// 创建HAProxy配置
		backendName := fmt.Sprintf("SVC_BACKEND_%s.%s:%d", service.Namespace, service.Name, port.Port)
		frontendName := fmt.Sprintf("SVC_FRONTEND_%s.%s:%d", service.Namespace, service.Name, mappedPort)
		bindName := fmt.Sprintf("SVC_BIND_%s_%s_%s", service.Namespace, service.Name, port.Name)

		if err := c.createServiceProxy(service, port, mappedPort, backendName, frontendName, bindName); err != nil {
			klog.Errorf("Failed to create service proxy for %s: %v", mappingKey, err)
			c.releasePort(mappedPort)
			continue
		}

		// 保存映射信息
		mapping := &ServiceMapping{
			ServiceName:  service.Name,
			Namespace:    service.Namespace,
			ServicePort:  port.Port,
			PortName:     port.Name,
			MappedPort:   mappedPort,
			BackendName:  backendName,
			FrontendName: frontendName,
			BindName:     bindName,
			ClusterIP:    service.Spec.ClusterIP,
			CreatedAt:    time.Now(),
		}

		c.serviceMappings[mappingKey] = mapping

		klog.Infof("Created service mapping: %s:%d (%s) -> :%d", mappingKey, port.Port, port.Name, mappedPort)
	}
}

// 移除服务映射
func (c *ServiceController) removeServiceMappings(service *corev1.Service) {
	c.listenersLock.Lock()
	defer c.listenersLock.Unlock()

	var mappingsToDelete []string
	servicePrefix := fmt.Sprintf("%s/%s/", service.Namespace, service.Name)

	for key, mapping := range c.serviceMappings {
		mapping = mapping
		if strings.HasPrefix(key, servicePrefix) {
			mappingsToDelete = append(mappingsToDelete, key)
		}
	}

	for _, key := range mappingsToDelete {
		mapping := c.serviceMappings[key]

		// 删除HAProxy配置
		c.handler.DeleteFrontend(mapping.FrontendName)
		c.handler.DeleteBackend(mapping.BackendName)

		// 释放端口
		c.releasePort(mapping.MappedPort)

		// 删除映射记录
		delete(c.serviceMappings, key)

		klog.Infof("Deleted service mapping: %s -> :%d", key, mapping.MappedPort)
	}
}

// 创建服务代理配置
func (c *ServiceController) createServiceProxy(service *corev1.Service, port corev1.ServicePort, mappedPort int, backendName, frontendName, bindName string) error {
	checkTimeout := int64(15)
	bindPortInt64 := int64(mappedPort)

	// 创建Backend
	_, err := c.handler.AddBackend(&models.Backend{
		Name:         backendName,
		Mode:         "tcp",
		CheckTimeout: &checkTimeout,
	})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("failed to create backend: %v", err)
	}

	// 添加Service ClusterIP作为backend server
	serverName := fmt.Sprintf("%s:%d", service.Spec.ClusterIP, port.Port)
	server := &haproxy.Server{
		Name:    serverName,
		Address: fmt.Sprintf("%s:%d", service.Spec.ClusterIP, port.Port),
		Check:   "enabled",
	}

	_, err = c.handler.AddServerToBackend(server, backendName)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("failed to add server to backend: %v", err)
	}

	// 创建Frontend
	_, err = c.handler.AddFrontend(&models.Frontend{
		Name:           frontendName,
		DefaultBackend: backendName,
		Mode:           "tcp",
	})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("failed to create frontend: %v", err)
	}

	// 创建Bind
	_, err = c.handler.AddBind(&models.Bind{
		Name:    bindName,
		Port:    &bindPortInt64,
		Address: c.controllerAddr,
	}, frontendName)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("failed to create bind: %v", err)
	}

	return nil
}

// 列出当前映射（提供给可能的监控接口）
func (c *ServiceController) ListServiceMappings() []ServiceMapping {
	c.listenersLock.RLock()
	defer c.listenersLock.RUnlock()

	var mappings []ServiceMapping
	for _, mapping := range c.serviceMappings {
		mappings = append(mappings, ServiceMapping{
			ServiceName:  mapping.ServiceName,
			Namespace:    mapping.Namespace,
			ServicePort:  mapping.ServicePort,
			PortName:     mapping.PortName,
			MappedPort:   mapping.MappedPort,
			BackendName:  mapping.BackendName,
			FrontendName: mapping.FrontendName,
			ClusterIP:    mapping.ClusterIP,
			CreatedAt:    mapping.CreatedAt,
		})
	}

	// 按创建时间排序
	sort.Slice(mappings, func(i, j int) bool {
		return mappings[i].CreatedAt.After(mappings[j].CreatedAt)
	})

	return mappings
}

func (c *ServiceController) Run() {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	klog.V(2).Info("Starting service proxy controller.")

	go c.serviceController.Run(c.stopCh)

	if !cache.WaitForCacheSync(c.stopCh, c.serviceController.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("service controller sync failed"))
		return
	}

	klog.Info("Service proxy controller started successfully")
	<-c.stopCh
	klog.V(2).Info("Stopping service proxy controller.")
}

func RunServiceController(
	stopCh chan struct{},
	kubeconfig string,
	listenAddr string,
	listenPort int,
	dataplanHost string,
	dataplanUser string,
	dataplanPassword string,
	portRangeStart int, portRangeEnd int,
	allowedNamespaces []string,
	targetPortNames []string,
	resyncTime int,
) *ServiceController {
	var (
		restConfig *rest.Config
		err        error
	)

	if _, err := os.Stat(kubeconfig); err != nil {
		klog.V(2).Infof("%s, trying in-cluster mode.", err.Error())
	}

	if restConfig, err = rest.InClusterConfig(); err != nil {
		restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err)
		}
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		panic(err)
	}

	// 创建Service监听器
	serviceLister := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "services", metav1.NamespaceAll, fields.Everything())
	serviceQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	serviceController := newServiceController(
		nil, nil, serviceQueue, stopCh, listenPort, listenAddr,
		dataplanHost, dataplanUser, dataplanPassword,
		portRangeStart, portRangeEnd, allowedNamespaces, targetPortNames, clientset,
	)

	serviceIndexer, serviceControllerInformer := cache.NewIndexerInformer(
		serviceLister,
		&corev1.Service{},
		time.Duration(resyncTime)*time.Second,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    serviceController.handleServiceAdd,
			UpdateFunc: serviceController.handleServiceUpdate,
			DeleteFunc: serviceController.handleServiceDelete,
		}, cache.Indexers{})

	serviceController.serviceLister = serviceIndexer
	serviceController.serviceController = serviceControllerInformer

	return serviceController
}
