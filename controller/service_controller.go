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

	checkTimeout  int64
	checkInterval int64
	hideBackend   bool
	maxBatchSize  int

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
	checkTimeout int64,
	checkInterval int64,
	maxBatchSize int,
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
		checkTimeout:      checkTimeout,
		checkInterval:     checkInterval,
		hideBackend:       hideBackend,
		maxBatchSize:      maxBatchSize,
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
	oldService := oldObj.(*corev1.Service)
	newService := newObj.(*corev1.Service)

	// 如果关键属性没变，跳过
	if oldService.ResourceVersion == newService.ResourceVersion {
		return
	}

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

func (c *ServiceController) Run() {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.V(2).Info("Starting service proxy controller.")

	go c.serviceController.Run(c.stopCh)

	if !cache.WaitForCacheSync(c.stopCh, c.serviceController.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("service controller sync failed"))
		return
	}

	klog.Info("Service proxy controller synced, starting workers.")

	// 启动 worker
	go wait.Until(c.runWorker, time.Second, c.stopCh)

	<-c.stopCh
	klog.V(2).Info("Stopping service proxy controller.")
}

func (c *ServiceController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *ServiceController) processNextWorkItem() bool {
	// 尝试获取一个项目
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	// 批量处理逻辑
	// 收集当前队列中的所有项目，合并到一个事务中
	keys := []interface{}{key}

	// 尽可能多地从队列中获取待处理项，直到达到上限或队列为空
	// 这在启动时的 "Reload Storm" 场景下非常有效
	maxBatchSize := c.maxBatchSize
	for i := 0; i < maxBatchSize-1; i++ {
		if c.queue.Len() > 0 {
			nextKey, nextQuit := c.queue.Get()
			if nextQuit {
				break
			}
			keys = append(keys, nextKey)
		} else {
			break
		}
	}

	klog.V(2).Infof("Batch processing %d service updates", len(keys))

	// 开启一个大事务处理这批变更
	txID, err := c.handler.StartTransaction()
	if err != nil {
		klog.Errorf("Failed to start transaction for batch: %v", err)
		// 如果开启事务失败，将所有 key 重新加入队列
		for _, k := range keys {
			c.queue.AddRateLimited(k)
		}
		return true
	}

	successCount := 0
	for _, k := range keys {
		err := c.syncService(k.(string), txID)
		if err != nil {
			klog.Errorf("Error syncing service %v: %v", k, err)
			c.queue.AddRateLimited(k)
		} else {
			c.queue.Forget(k)
			successCount++
		}
		// 每次同步完一个 service，不管是 Done 还是重新加入队列，都要调用 Done
		// 注意：这里的 keys slice 中的第一个元素已经在外部 Done 过了，
		// 但后续循环中从 Get 获取的也需要 Done。
		// 为了简化逻辑，我们在循环外部统一 defer c.queue.Done(key)，
		// 而对于后续从 Get 获取的，我们需要额外调用 Done。
	}

	// 处理除了第一个之外的其他 key 的 Done 状态
	for i := 1; i < len(keys); i++ {
		c.queue.Done(keys[i])
	}

	if successCount > 0 {
		if err := c.handler.CommitTransaction(txID); err != nil {
			klog.Errorf("Failed to commit batch transaction: %v", err)
			// 如果提交失败，由于我们已经 Forget 了部分，可能需要更复杂的重试逻辑
			// 但通常这里是网络或配置冲突
		} else {
			klog.V(2).Infof("Successfully committed batch of %d services", successCount)
		}
	} else {
		c.handler.DiscardTransaction(txID)
	}

	return true
}

func (c *ServiceController) syncService(key string, txID string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	obj, exists, err := c.serviceLister.GetByKey(key)
	if err != nil {
		return err
	}

	if !exists {
		klog.V(2).Infof("Service %s deleted, removing mappings", key)
		// 构造一个虚拟的 Service 对象用于处理删除
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
		c.removeServiceMappingsWithTx(svc, txID)
		return nil
	}

	service := obj.(*corev1.Service)
	if !c.isNamespaceAllowed(service.Namespace) {
		return nil
	}

	c.processServiceWithTx(service, txID)
	return nil
}

// 内部处理逻辑，支持传入 txID
func (c *ServiceController) processServiceWithTx(service *corev1.Service, txID string) {
	c.listenersLock.Lock()
	defer c.listenersLock.Unlock()

	if service.Spec.ClusterIP == "" || service.Spec.ClusterIP == "None" {
		return
	}

	for _, port := range service.Spec.Ports {
		if !c.isTargetPortName(port.Name) {
			continue
		}

		mappingKey := fmt.Sprintf("%s/%s/%s", service.Namespace, service.Name, port.Name)
		if _, exists := c.serviceMappings[mappingKey]; exists {
			continue
		}

		mappedPort, err := c.allocatePort()
		if err != nil {
			klog.Errorf("Failed to allocate port for %s: %v", mappingKey, err)
			continue
		}

		backendName := fmt.Sprintf("SVC_BACKEND_%s.%s:%d", service.Namespace, service.Name, port.Port)
		frontendName := fmt.Sprintf("SVC_FRONTEND_%s.%s:%d", service.Namespace, service.Name, mappedPort)
		bindName := fmt.Sprintf("SVC_BIND_%s_%s_%s", service.Namespace, service.Name, port.Name)

		if err := c.createServiceProxy(service, port, mappedPort, backendName, frontendName, bindName, c.hideBackend, txID); err != nil {
			klog.Errorf("Failed to create service proxy for %s: %v", mappingKey, err)
			c.releasePort(mappedPort)
			continue
		}

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

func (c *ServiceController) removeServiceMappingsWithTx(service *corev1.Service, txID string) {
	c.listenersLock.Lock()
	defer c.listenersLock.Unlock()

	var mappingsToDelete []string
	servicePrefix := fmt.Sprintf("%s/%s/", service.Namespace, service.Name)

	for key := range c.serviceMappings {
		if strings.HasPrefix(key, servicePrefix) {
			mappingsToDelete = append(mappingsToDelete, key)
		}
	}

	for _, key := range mappingsToDelete {
		mapping := c.serviceMappings[key]
		c.handler.DeleteFrontend(mapping.FrontendName, txID)
		c.handler.DeleteBackend(mapping.BackendName, txID)
		c.releasePort(mapping.MappedPort)
		delete(c.serviceMappings, key)
		klog.Infof("Deleted service mapping: %s -> :%d", key, mapping.MappedPort)
	}
}

// 保留原有函数但标记为过时或重构为调用新方法（这里直接更新原有逻辑以配合批量处理）
// 原有的 processService / removeServiceMappings 可以删除或修改，
// 因为我们现在主要通过 syncService 和事务批量处理。

func (c *ServiceController) createServiceProxy(service *corev1.Service, port corev1.ServicePort, mappedPort int, backendName, frontendName, bindName string, hide bool, txID string) error {
	bindPortInt64 := int64(mappedPort)

	// 创建/更新 Backend
	checkTimeoutMs := c.checkTimeout * 1000
	backend_obj := &models.Backend{
		Name:         backendName,
		Mode:         "tcp",
		CheckTimeout: &checkTimeoutMs,
	}
	if hide {
		backend_obj.StatsOptions = &models.StatsOptions{StatsEnable: true}
	}

	var err error
	if c.handler.EnsureBackend(backendName) {
		_, err = c.handler.ReplaceBackend(backendName, backend_obj, txID)
	} else {
		_, err = c.handler.AddBackend(backend_obj, txID)
	}

	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("failed to create/update backend: %v", err)
	}

	// 添加Service ClusterIP作为backend server
	serverName := fmt.Sprintf("%s:%d", service.Spec.ClusterIP, port.Port)
	server := &haproxy.Server{
		Name:    serverName,
		Address: fmt.Sprintf("%s:%d", service.Spec.ClusterIP, port.Port),
		Check:   "enabled",
		Inter:   int64(c.checkInterval * 1000), // 转换为毫秒
	}
	_, err = c.handler.AddServerToBackend(server, backendName, txID)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			_, err = c.handler.ReplaceServerFromBackend(serverName, server, backendName, txID)
			if err != nil {
				return fmt.Errorf("failed to update server in backend: %v", err)
			}
		} else {
			return fmt.Errorf("failed to add server to backend: %v", err)
		}
	}

	// 创建/更新 Frontend
	frontend_obj := &models.Frontend{
		Name:           frontendName,
		DefaultBackend: backendName,
		Mode:           "tcp",
	}

	if c.handler.EnsureFrontend(frontendName) {
		_, err = c.handler.ReplaceFrontend(frontendName, frontend_obj, txID)
	} else {
		_, err = c.handler.AddFrontend(frontend_obj, txID)
	}

	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("failed to create/update frontend: %v", err)
	}

	// 创建Bind
	_, err = c.handler.AddBind(&models.Bind{
		Name:    bindName,
		Port:    &bindPortInt64,
		Address: c.controllerAddr,
	}, frontendName, txID)
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

	sort.Slice(mappings, func(i, j int) bool {
		return mappings[i].CreatedAt.After(mappings[j].CreatedAt)
	})

	return mappings
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
	checkTimeout int64,
	checkInterval int64,
	maxBatchSize int,
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
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	serviceController := newServiceController(
		nil, nil, queue, stopCh, listenPort, listenAddr,
		dataplanHost, dataplanUser, dataplanPassword, hideBackend,
		portRangeStart, portRangeEnd, allowedNamespaces, targetPortNames, clientset,
		checkTimeout, checkInterval, maxBatchSize,
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
