package controller

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	models "github.com/haproxytech/models/v2"
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

	hideBackend bool

	clientset *kubernetes.Clientset
}

func newServiceController(
	serviceLister cache.Indexer,
	serviceController cache.Controller,
	queue workqueue.RateLimitingInterface,
	stopCh chan struct{},
	listenPort int,
	listenAddr string,
	dataplanHost string,
	dataplanUser string,
	dataplanPassword string,
	hideBackend bool,
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
		hideBackend:       hideBackend,
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
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

// 处理Service更新事件
func (c *ServiceController) handleServiceUpdate(oldObj, newObj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

// 处理Service删除事件
func (c *ServiceController) handleServiceDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.queue.Add(key)
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

		// 开启事务
		txID, err := c.handler.StartTransaction()
		if err != nil {
			klog.Errorf("Failed to start transaction for %s: %v", mappingKey, err)
			continue
		}

		// 分配端口
		mappedPort, err := c.allocatePort()
		if err != nil {
			klog.Errorf("Failed to allocate port for %s: %v", mappingKey, err)
			c.handler.DiscardTransaction(txID)
			continue
		}

		// 创建HAProxy配置
		backendName := fmt.Sprintf("SVC_BACKEND_%s.%s:%d", service.Namespace, service.Name, port.Port)
		frontendName := fmt.Sprintf("SVC_FRONTEND_%s.%s:%d", service.Namespace, service.Name, mappedPort)
		bindName := fmt.Sprintf("SVC_BIND_%s_%s_%s", service.Namespace, service.Name, port.Name)

		if err := c.createServiceProxy(service, port, mappedPort, backendName, frontendName, bindName, c.hideBackend); err != nil {
			klog.Errorf("Failed to create service proxy for %s: %v", mappingKey, err)
			c.releasePort(mappedPort)
			c.handler.DiscardTransaction(txID)
			continue
		}

		// 提交事务
		if err := c.handler.CommitTransaction(txID); err != nil {
			klog.Errorf("Failed to commit transaction for %s: %v", mappingKey, err)
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

// 移除指定服务的所有映射
func (c *ServiceController) removeServiceMappingsByKey(namespace, name string) {
	c.listenersLock.Lock()
	defer c.listenersLock.Unlock()

	var mappingsToDelete []string
	servicePrefix := fmt.Sprintf("%s/%s/", namespace, name)

	for key := range c.serviceMappings {
		if strings.HasPrefix(key, servicePrefix) {
			mappingsToDelete = append(mappingsToDelete, key)
		}
	}

	if len(mappingsToDelete) == 0 {
		return
	}

	// 开启事务
	txID, err := c.handler.StartTransaction()
	if err != nil {
		klog.Errorf("Failed to start transaction for service removal: %v", err)
		return
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

	// 提交事务
	if err := c.handler.CommitTransaction(txID); err != nil {
		klog.Errorf("Failed to commit transaction for service removal: %v", err)
	}
}

func (c *ServiceController) syncService(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	// 获取现有映射
	c.listenersLock.RLock()
	existingMappings := make(map[string]*ServiceMapping)
	servicePrefix := fmt.Sprintf("%s/%s/", namespace, name)
	for k, v := range c.serviceMappings {
		if strings.HasPrefix(k, servicePrefix) {
			existingMappings[k] = v
		}
	}
	c.listenersLock.RUnlock()

	obj, exists, err := c.serviceLister.GetByKey(key)
	if err != nil {
		return err
	}

	if !exists {
		if len(existingMappings) > 0 {
			klog.V(2).Infof("Service %s deleted, cleaning up mappings", key)
			c.removeServiceMappingsByKey(namespace, name)
		}
		return nil
	}

	service := obj.(*corev1.Service)

	// 检查命名空间和 ClusterIP
	if !c.isNamespaceAllowed(service.Namespace) || service.Spec.ClusterIP == "" || service.Spec.ClusterIP == "None" {
		if len(existingMappings) > 0 {
			klog.V(2).Infof("Service %s no longer qualifies for proxy, cleaning up mappings", key)
			c.removeServiceMappingsByKey(namespace, name)
		}
		return nil
	}

	// 计算目标端口
	targetPorts := make(map[string]corev1.ServicePort)
	for _, p := range service.Spec.Ports {
		if c.isTargetPortName(p.Name) {
			targetPorts[p.Name] = p
		}
	}

	// 检查是否需要更新
	needsUpdate := false
	if len(existingMappings) != len(targetPorts) {
		needsUpdate = true
	} else {
		for portName, port := range targetPorts {
			mappingKey := fmt.Sprintf("%s/%s/%s", namespace, name, portName)
			m, ok := existingMappings[mappingKey]
			if !ok || m.ServicePort != port.Port || m.ClusterIP != service.Spec.ClusterIP {
				needsUpdate = true
				break
			}
		}
	}

	if needsUpdate {
		klog.V(2).Infof("Syncing service %s (needsUpdate=true)", key)
		c.removeServiceMappingsByKey(namespace, name)
		c.processService(service)
	} else {
		klog.V(3).Infof("Service %s matches existing mappings, skipping", key)
	}

	return nil
}

func (c *ServiceController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncService(key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("Error syncing service %v: %v", key, err))
	c.queue.AddRateLimited(key)

	return true
}

func (c *ServiceController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// 创建服务代理配置
func (c *ServiceController) createServiceProxy(service *corev1.Service, port corev1.ServicePort, mappedPort int, backendName, frontendName, bindName string, hide bool) error {
	checkTimeout := int64(15)
	bindPortInt64 := int64(mappedPort)

	// 创建Backend
	backend_obj := &models.Backend{
		Name:         backendName,
		Mode:         "tcp",
		CheckTimeout: &checkTimeout,
	}
	if hide {
		backend_obj.StatsOptions = &models.StatsOptions{StatsEnable: true}
	}
	_, err := c.handler.AddBackend(backend_obj)

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

	// 启动 worker
	go wait.Until(c.runWorker, time.Second, c.stopCh)

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
	hideBackend bool,
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
		dataplanHost, dataplanUser, dataplanPassword, hideBackend,
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
