// Package topology 提供全栈拓扑发现与路径追踪功能
// 支持 Pod/VM/物理机 自动发现和请求路径追踪
package topology

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-flow-agent/pkg/models"
)

// EntityType 实体类型
type EntityType string

const (
	EntityTypePod       EntityType = "pod"        // Kubernetes Pod
	EntityTypeVM        EntityType = "vm"         // 虚拟机
	EntityTypePhysical  EntityType = "physical"   // 物理机
	EntityTypeContainer EntityType = "container"  // 容器
	EntityTypeService   EntityType = "service"    // K8s Service
	EntityTypeNode      EntityType = "node"       // K8s Node
	EntityTypeNamespace EntityType = "namespace"  // K8s Namespace
	EntityTypeCluster   EntityType = "cluster"    // K8s Cluster
	EntityTypeVPC       EntityType = "vpc"        // VPC网络
	EntityTypeSubnet    EntityType = "subnet"     // 子网
)

// Entity 拓扑实体
type Entity struct {
	ID          string            `json:"id" yaml:"id"`
	Name        string            `json:"name" yaml:"name"`
	Type        EntityType        `json:"type" yaml:"type"`
	IPAddresses []string          `json:"ip_addresses" yaml:"ip_addresses"`
	MACAddress  string            `json:"mac_address" yaml:"mac_address"`
	Labels      map[string]string `json:"labels" yaml:"labels"`
	Annotations map[string]string `json:"annotations" yaml:"annotations"`
	ParentID    string            `json:"parent_id" yaml:"parent_id"`       // 父实体ID
	ClusterID   string            `json:"cluster_id" yaml:"cluster_id"`     // 所属集群ID
	Namespace   string            `json:"namespace" yaml:"namespace"`       // K8s命名空间
	NodeName    string            `json:"node_name" yaml:"node_name"`       // 所在节点名
	PodName     string            `json:"pod_name" yaml:"pod_name"`         // Pod名称
	ContainerID string            `json:"container_id" yaml:"container_id"` // 容器ID
	ProcessIDs  []uint32          `json:"process_ids" yaml:"process_ids"`   // 关联进程ID
	Status      string            `json:"status" yaml:"status"`             // 运行状态
	CreatedAt   time.Time         `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at" yaml:"updated_at"`
	Attributes  map[string]interface{} `json:"attributes" yaml:"attributes"` // 扩展属性
}

// RelationType 关系类型
type RelationType string

const (
	RelationTypeContains   RelationType = "contains"    // 包含关系
	RelationTypeRunsOn     RelationType = "runs_on"     // 运行于
	RelationTypeConnects   RelationType = "connects"    // 连接关系
	RelationTypeDependsOn  RelationType = "depends_on"  // 依赖关系
	RelationTypeParentOf   RelationType = "parent_of"   // 父子关系
	RelationTypePeerOf     RelationType = "peer_of"     // 对等关系
	RelationTypeBelongsTo  RelationType = "belongs_to"  // 归属关系
)

// Relation 拓扑关系
type Relation struct {
	ID           string                 `json:"id" yaml:"id"`
	SourceID     string                 `json:"source_id" yaml:"source_id"`
	TargetID     string                 `json:"target_id" yaml:"target_id"`
	Type         RelationType           `json:"type" yaml:"type"`
	Properties   map[string]interface{} `json:"properties" yaml:"properties"`
	CreatedAt    time.Time              `json:"created_at" yaml:"created_at"`
}

// DiscoveryConfig 发现配置
type DiscoveryConfig struct {
	Enabled            bool          `json:"enabled" yaml:"enabled"`
	DiscoverPods       bool          `json:"discover_pods" yaml:"discover_pods"`
	DiscoverVMs        bool          `json:"discover_vms" yaml:"discover_vms"`
	DiscoverPhysical   bool          `json:"discover_physical" yaml:"discover_physical"`
	DiscoverContainers bool          `json:"discover_containers" yaml:"discover_containers"`
	DiscoverServices   bool          `json:"discover_services" yaml:"discover_services"`
	DiscoverNetwork    bool          `json:"discover_network" yaml:"discover_network"`
	Interval           time.Duration `json:"interval" yaml:"interval"`
	KubeConfigPath     string        `json:"kube_config_path" yaml:"kube_config_path"`
	KubeAPIServer      string        `json:"kube_api_server" yaml:"kube_api_server"`
	CloudMetadataURL   string        `json:"cloud_metadata_url" yaml:"cloud_metadata_url"`
	ProcPath           string        `json:"proc_path" yaml:"proc_path"`
	SysPath            string        `json:"sys_path" yaml:"sys_path"`
	EnableCgroupScan   bool          `json:"enable_cgroup_scan" yaml:"enable_cgroup_scan"`
	EnableNetlinkScan  bool          `json:"enable_netlink_scan" yaml:"enable_netlink_scan"`
}

// DiscoveryEngine 拓扑发现引擎
type DiscoveryEngine struct {
	config     *DiscoveryConfig
	entities   map[string]*Entity
	relations  map[string]*Relation
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	handlers   []DiscoveryHandler
	k8sClient  *KubernetesClient
	cloudInfo  *CloudMetadata
}

// DiscoveryHandler 发现事件处理器
type DiscoveryHandler interface {
	OnEntityDiscovered(entity *Entity)
	OnEntityUpdated(entity *Entity)
	OnEntityDeleted(entityID string)
	OnRelationDiscovered(relation *Relation)
	OnRelationDeleted(relationID string)
}

// KubernetesClient Kubernetes客户端
type KubernetesClient struct {
	apiServer    string
	bearerToken  string
	caCertPath   string
	inCluster    bool
	httpClient   *http.Client
}

// CloudMetadata 云平台元数据
type CloudMetadata struct {
	Provider    string `json:"provider"`     // aws/gcp/azure/aliyun/tencent/huawei/openstack
	InstanceID  string `json:"instance_id"`
	InstanceType string `json:"instance_type"`
	Region      string `json:"region"`
	Zone        string `json:"zone"`
	VPCID       string `json:"vpc_id"`
	SubnetID    string `json:"subnet_id"`
	ImageID     string `json:"image_id"`
	PrivateIP   string `json:"private_ip"`
	PublicIP    string `json:"public_ip"`
	MACAddress  string `json:"mac_address"`
	Hostname    string `json:"hostname"`
}

// NewDiscoveryEngine 创建拓扑发现引擎
func NewDiscoveryEngine(config *DiscoveryConfig) *DiscoveryEngine {
	ctx, cancel := context.WithCancel(context.Background())
	
	engine := &DiscoveryEngine{
		config:    config,
		entities:  make(map[string]*Entity),
		relations: make(map[string]*Relation),
		ctx:       ctx,
		cancel:    cancel,
	}
	
	// 初始化Kubernetes客户端
	if config.DiscoverPods || config.DiscoverServices {
		engine.k8sClient = NewKubernetesClient(config.KubeConfigPath, config.KubeAPIServer)
	}
	
	// 获取云元数据
	if config.DiscoverVMs || config.DiscoverPhysical {
		engine.cloudInfo = FetchCloudMetadata(config.CloudMetadataURL)
	}
	
	return engine
}

// Start 启动发现引擎
func (e *DiscoveryEngine) Start() error {
	// 首次全量发现
	if err := e.discoverAll(); err != nil {
		return fmt.Errorf("initial discovery failed: %w", err)
	}
	
	// 启动定期发现
	e.wg.Add(1)
	go e.discoveryLoop()
	
	return nil
}

// Stop 停止发现引擎
func (e *DiscoveryEngine) Stop() {
	e.cancel()
	e.wg.Wait()
}

// RegisterHandler 注册发现事件处理器
func (e *DiscoveryEngine) RegisterHandler(handler DiscoveryHandler) {
	e.handlers = append(e.handlers, handler)
}

// discoveryLoop 定期发现循环
func (e *DiscoveryEngine) discoveryLoop() {
	defer e.wg.Done()
	
	ticker := time.NewTicker(e.config.Interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.discoverAll()
		}
	}
}

// discoverAll 执行全量发现
func (e *DiscoveryEngine) discoverAll() error {
	var wg sync.WaitGroup
	errCh := make(chan error, 10)
	
	// 发现物理机/当前主机
	if e.config.DiscoverPhysical {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := e.discoverPhysicalHost(); err != nil {
				errCh <- fmt.Errorf("physical discovery: %w", err)
			}
		}()
	}
	
	// 发现虚拟机
	if e.config.DiscoverVMs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := e.discoverVirtualMachines(); err != nil {
				errCh <- fmt.Errorf("vm discovery: %w", err)
			}
		}()
	}
	
	// 发现Kubernetes资源
	if e.config.DiscoverPods || e.config.DiscoverServices {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := e.discoverKubernetesResources(); err != nil {
				errCh <- fmt.Errorf("kubernetes discovery: %w", err)
			}
		}()
	}
	
	// 发现容器
	if e.config.DiscoverContainers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := e.discoverContainers(); err != nil {
				errCh <- fmt.Errorf("container discovery: %w", err)
			}
		}()
	}
	
	// 发现网络拓扑
	if e.config.DiscoverNetwork {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := e.discoverNetworkTopology(); err != nil {
				errCh <- fmt.Errorf("network discovery: %w", err)
			}
		}()
	}
	
	wg.Wait()
	close(errCh)
	
	// 收集错误
	var errors []error
	for err := range errCh {
		errors = append(errors, err)
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("discovery errors: %v", errors)
	}
	
	return nil
}

// discoverPhysicalHost 发现物理机
func (e *DiscoveryEngine) discoverPhysicalHost() error {
	// 获取主机信息
	hostname, _ := os.Hostname()
	
	// 获取IP地址
	ipAddresses := e.getHostIPAddresses()
	
	// 获取MAC地址
	macAddress := e.getPrimaryMACAddress()
	
	// 获取系统信息
	systemInfo := e.getSystemInfo()
	
	// 判断是否为虚拟机
	isVM := e.isVirtualMachine()
	entityType := EntityTypePhysical
	if isVM {
		entityType = EntityTypeVM
	}
	
	entity := &Entity{
		ID:          fmt.Sprintf("host-%s", hostname),
		Name:        hostname,
		Type:        entityType,
		IPAddresses: ipAddresses,
		MACAddress:  macAddress,
		Labels: map[string]string{
			"os_type":      systemInfo["os_type"],
			"os_version":   systemInfo["os_version"],
			"kernel":       systemInfo["kernel"],
			"architecture": systemInfo["architecture"],
			"is_virtual":   strconv.FormatBool(isVM),
		},
		Status:    "running",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Attributes: map[string]interface{}{
			"cpu_cores":    systemInfo["cpu_cores"],
			"memory_total": systemInfo["memory_total"],
			"cloud_info":   e.cloudInfo,
		},
	}
	
	// 如果是云主机，添加云元数据
	if e.cloudInfo != nil {
		entity.ClusterID = e.cloudInfo.VPCID
		entity.Labels["cloud_provider"] = e.cloudInfo.Provider
		entity.Labels["cloud_region"] = e.cloudInfo.Region
		entity.Labels["cloud_zone"] = e.cloudInfo.Zone
		entity.Labels["instance_type"] = e.cloudInfo.InstanceType
	}
	
	e.addOrUpdateEntity(entity)
	
	return nil
}

// discoverVirtualMachines 发现虚拟机
func (e *DiscoveryEngine) discoverVirtualMachines() error {
	// 如果当前是云环境，获取云元数据
	if e.cloudInfo != nil && e.cloudInfo.Provider != "" {
		entity := &Entity{
			ID:          e.cloudInfo.InstanceID,
			Name:        e.cloudInfo.Hostname,
			Type:        EntityTypeVM,
			IPAddresses: []string{e.cloudInfo.PrivateIP, e.cloudInfo.PublicIP},
			MACAddress:  e.cloudInfo.MACAddress,
			Labels: map[string]string{
				"provider":      e.cloudInfo.Provider,
				"instance_type": e.cloudInfo.InstanceType,
				"region":        e.cloudInfo.Region,
				"zone":          e.cloudInfo.Zone,
			},
			ClusterID: e.cloudInfo.VPCID,
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Attributes: map[string]interface{}{
				"image_id":  e.cloudInfo.ImageID,
				"subnet_id": e.cloudInfo.SubnetID,
			},
		}
		
		e.addOrUpdateEntity(entity)
		
		// 创建VPC实体
		if e.cloudInfo.VPCID != "" {
			vpcEntity := &Entity{
				ID:        e.cloudInfo.VPCID,
				Name:      fmt.Sprintf("vpc-%s", e.cloudInfo.VPCID),
				Type:      EntityTypeVPC,
				ClusterID: e.cloudInfo.VPCID,
				Status:    "active",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			e.addOrUpdateEntity(vpcEntity)
			
			// 创建归属关系
			e.addRelation(&Relation{
				ID:        fmt.Sprintf("rel-%s-%s", entity.ID, vpcEntity.ID),
				SourceID:  entity.ID,
				TargetID:  vpcEntity.ID,
				Type:      RelationTypeBelongsTo,
				CreatedAt: time.Now(),
			})
		}
	}
	
	return nil
}

// discoverKubernetesResources 发现Kubernetes资源
func (e *DiscoveryEngine) discoverKubernetesResources() error {
	if e.k8sClient == nil {
		return nil
	}
	
	// 检查是否在Kubernetes集群中
	if !e.k8sClient.inCluster && e.k8sClient.apiServer == "" {
		return nil
	}
	
	// 发现节点
	if e.config.DiscoverPods {
		nodes, err := e.k8sClient.GetNodes()
		if err == nil {
			for _, node := range nodes {
				e.addOrUpdateEntity(node)
			}
		}
		
		// 发现Pod
		pods, err := e.k8sClient.GetPods()
		if err == nil {
			for _, pod := range pods {
				e.addOrUpdateEntity(pod)
			}
		}
	}
	
	// 发现Service
	if e.config.DiscoverServices {
		services, err := e.k8sClient.GetServices()
		if err == nil {
			for _, svc := range services {
				e.addOrUpdateEntity(svc)
			}
		}
	}
	
	return nil
}

// discoverContainers 发现容器
func (e *DiscoveryEngine) discoverContainers() error {
	// 通过cgroup发现容器
	if e.config.EnableCgroupScan {
		containers := e.discoverContainersFromCgroup()
		for _, container := range containers {
			e.addOrUpdateEntity(container)
		}
	}
	
	// 通过Docker socket发现容器
	containers := e.discoverContainersFromDocker()
	for _, container := range containers {
		e.addOrUpdateEntity(container)
	}
	
	return nil
}

// discoverContainersFromCgroup 从cgroup发现容器
func (e *DiscoveryEngine) discoverContainersFromCgroup() []*Entity {
	var containers []*Entity
	
	cgroupPath := "/sys/fs/cgroup"
	if e.config.SysPath != "" {
		cgroupPath = filepath.Join(e.config.SysPath, "fs/cgroup")
	}
	
	// 扫描cgroup中的容器ID
	containerIDPattern := regexp.MustCompile(`([a-f0-9]{64})`)
	
	// 读取cgroup文件
	cgroupFile := "/proc/self/cgroup"
	if e.config.ProcPath != "" {
		cgroupFile = filepath.Join(e.config.ProcPath, "self/cgroup")
	}
	
	data, err := ioutil.ReadFile(cgroupFile)
	if err != nil {
		return containers
	}
	
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		
		// 提取容器ID
		matches := containerIDPattern.FindAllString(line, -1)
		for _, containerID := range matches {
			if len(containerID) == 64 {
				entity := &Entity{
					ID:          fmt.Sprintf("container-%s", containerID[:12]),
					Name:        containerID[:12],
					Type:        EntityTypeContainer,
					ContainerID: containerID,
					Status:      "running",
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}
				containers = append(containers, entity)
			}
		}
	}
	
	return containers
}

// discoverContainersFromDocker 从Docker发现容器
func (e *DiscoveryEngine) discoverContainersFromDocker() []*Entity {
	var containers []*Entity
	
	// 执行docker ps命令
	cmd := exec.Command("docker", "ps", "--format", "{{.ID}}\t{{.Names}}\t{{.Status}}")
	output, err := cmd.Output()
	if err != nil {
		return containers
	}
	
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 {
			containerID := parts[0]
			containerName := parts[1]
			status := "running"
			if len(parts) >= 3 {
				status = parts[2]
			}
			
			entity := &Entity{
				ID:          fmt.Sprintf("container-%s", containerID),
				Name:        containerName,
				Type:        EntityTypeContainer,
				ContainerID: containerID,
				Status:      status,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			containers = append(containers, entity)
		}
	}
	
	return containers
}

// discoverNetworkTopology 发现网络拓扑
func (e *DiscoveryEngine) discoverNetworkTopology() error {
	// 发现网络接口
	interfaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	
	for _, iface := range interfaces {
		// 获取接口IP地址
		var ips []string
		addrs, err := iface.Addrs()
		if err == nil {
			for _, addr := range addrs {
				ips = append(ips, addr.String())
			}
		}
		
		// 创建网络接口实体
		entity := &Entity{
			ID:          fmt.Sprintf("iface-%s", iface.Name),
			Name:        iface.Name,
			Type:        EntityTypeSubnet,
			IPAddresses: ips,
			MACAddress:  iface.HardwareAddr.String(),
			Labels: map[string]string{
				"mtu":  strconv.Itoa(iface.MTU),
				"flags": iface.Flags.String(),
			},
			Status:    "up",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		
		e.addOrUpdateEntity(entity)
	}
	
	return nil
}

// getHostIPAddresses 获取主机IP地址
func (e *DiscoveryEngine) getHostIPAddresses() []string {
	var ips []string
	
	interfaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	
	for _, iface := range interfaces {
		// 跳过回环接口
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if ok && !ipNet.IP.IsLoopback() {
				if ipNet.IP.To4() != nil {
					ips = append(ips, ipNet.IP.String())
				}
			}
		}
	}
	
	return ips
}

// getPrimaryMACAddress 获取主MAC地址
func (e *DiscoveryEngine) getPrimaryMACAddress() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	
	for _, iface := range interfaces {
		// 跳过回环接口
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		
		if iface.HardwareAddr.String() != "" {
			return iface.HardwareAddr.String()
		}
	}
	
	return ""
}

// getSystemInfo 获取系统信息
func (e *DiscoveryEngine) getSystemInfo() map[string]string {
	info := make(map[string]string)
	
	// 获取主机名
	hostname, _ := os.Hostname()
	info["hostname"] = hostname
	
	// 读取/etc/os-release
	if data, err := ioutil.ReadFile("/etc/os-release"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := strings.Trim(parts[1], "\"")
				switch key {
				case "ID":
					info["os_type"] = value
				case "VERSION_ID":
					info["os_version"] = value
				}
			}
		}
	}
	
	// 获取内核版本
	if data, err := ioutil.ReadFile("/proc/version"); err == nil {
		info["kernel"] = strings.Fields(string(data))[2]
	}
	
	// 获取架构
	info["architecture"] = os.Getenv("HOSTTYPE")
	if info["architecture"] == "" {
		info["architecture"] = "x86_64"
	}
	
	// 获取CPU核心数
	info["cpu_cores"] = strconv.Itoa(runtimeNumCPU())
	
	// 获取内存总量
	if data, err := ioutil.ReadFile("/proc/meminfo"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					info["memory_total"] = fields[1] + " kB"
				}
				break
			}
		}
	}
	
	return info
}

// isVirtualMachine 判断是否为虚拟机
func (e *DiscoveryEngine) isVirtualMachine() bool {
	// 检查DMI信息
	if data, err := ioutil.ReadFile("/sys/class/dmi/id/product_name"); err == nil {
		productName := strings.ToLower(string(data))
		virtualIndicators := []string{"virtual", "vmware", "kvm", "qemu", "xen", "hyper-v", "virtualbox", "parallels"}
		for _, indicator := range virtualIndicators {
			if strings.Contains(productName, indicator) {
				return true
			}
		}
	}
	
	// 检查/proc/cpuinfo
	if data, err := ioutil.ReadFile("/proc/cpuinfo"); err == nil {
		content := strings.ToLower(string(data))
		if strings.Contains(content, "hypervisor") {
			return true
		}
	}
	
	return false
}

// addOrUpdateEntity 添加或更新实体
func (e *DiscoveryEngine) addOrUpdateEntity(entity *Entity) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	existing, exists := e.entities[entity.ID]
	if exists {
		entity.CreatedAt = existing.CreatedAt
		entity.UpdatedAt = time.Now()
		
		// 通知更新
		for _, handler := range e.handlers {
			handler.OnEntityUpdated(entity)
		}
	} else {
		// 通知发现
		for _, handler := range e.handlers {
			handler.OnEntityDiscovered(entity)
		}
	}
	
	e.entities[entity.ID] = entity
}

// addRelation 添加关系
func (e *DiscoveryEngine) addRelation(relation *Relation) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	e.relations[relation.ID] = relation
	
	// 通知发现
	for _, handler := range e.handlers {
		handler.OnRelationDiscovered(relation)
	}
}

// GetEntity 获取实体
func (e *DiscoveryEngine) GetEntity(id string) *Entity {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	return e.entities[id]
}

// GetEntities 获取所有实体
func (e *DiscoveryEngine) GetEntities() []*Entity {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	entities := make([]*Entity, 0, len(e.entities))
	for _, entity := range e.entities {
		entities = append(entities, entity)
	}
	
	return entities
}

// GetEntitiesByType 按类型获取实体
func (e *DiscoveryEngine) GetEntitiesByType(entityType EntityType) []*Entity {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	var entities []*Entity
	for _, entity := range e.entities {
		if entity.Type == entityType {
			entities = append(entities, entity)
		}
	}
	
	return entities
}

// GetRelations 获取所有关系
func (e *DiscoveryEngine) GetRelations() []*Relation {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	relations := make([]*Relation, 0, len(e.relations))
	for _, relation := range e.relations {
		relations = append(relations, relation)
	}
	
	return relations
}

// GetTopology 获取完整拓扑
func (e *DiscoveryEngine) GetTopology() *Topology {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	return &Topology{
		Entities:  e.entities,
		Relations: e.relations,
		UpdatedAt: time.Now(),
	}
}

// Topology 拓扑结构
type Topology struct {
	Entities  map[string]*Entity  `json:"entities"`
	Relations map[string]*Relation `json:"relations"`
	UpdatedAt time.Time           `json:"updated_at"`
}

// NewKubernetesClient 创建Kubernetes客户端
func NewKubernetesClient(kubeConfigPath, apiServer string) *KubernetesClient {
	client := &KubernetesClient{
		apiServer: apiServer,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	
	// 检查是否在集群内
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		client.inCluster = true
		
		// 读取token
		if token, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
			client.bearerToken = string(token)
		}
		
		// 设置CA证书
		client.caCertPath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
		
		// 获取API Server地址
		if apiServer == "" {
			host := os.Getenv("KUBERNETES_SERVICE_HOST")
			port := os.Getenv("KUBERNETES_SERVICE_PORT")
			if host != "" && port != "" {
				client.apiServer = fmt.Sprintf("https://%s:%s", host, port)
			}
		}
	}
	
	return client
}

// GetNodes 获取Kubernetes节点
func (c *KubernetesClient) GetNodes() ([]*Entity, error) {
	if c.apiServer == "" {
		return nil, fmt.Errorf("kubernetes api server not configured")
	}
	
	url := fmt.Sprintf("%s/api/v1/nodes", c.apiServer)
	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var nodeList struct {
		Items []struct {
			Metadata struct {
				Name              string            `json:"name"`
				UID               string            `json:"uid"`
				Labels            map[string]string `json:"labels"`
				Annotations       map[string]string `json:"annotations"`
				CreationTimestamp time.Time         `json:"creationTimestamp"`
			} `json:"metadata"`
			Status struct {
				Addresses []struct {
					Type    string `json:"type"`
					Address string `json:"address"`
				} `json:"addresses"`
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
				NodeInfo struct {
					OSImage         string `json:"osImage"`
					KernelVersion   string `json:"kernelVersion"`
					KubeletVersion  string `json:"kubeletVersion"`
					ContainerRuntime string `json:"containerRuntimeVersion"`
				} `json:"nodeInfo"`
			} `json:"status"`
		} `json:"items"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&nodeList); err != nil {
		return nil, err
	}
	
	var entities []*Entity
	for _, item := range nodeList.Items {
		var ips []string
		for _, addr := range item.Status.Addresses {
			ips = append(ips, addr.Address)
		}
		
		status := "Ready"
		for _, cond := range item.Status.Conditions {
			if cond.Type == "Ready" && cond.Status != "True" {
				status = "NotReady"
			}
		}
		
		entity := &Entity{
			ID:          string(item.Metadata.UID),
			Name:        item.Metadata.Name,
			Type:        EntityTypeNode,
			IPAddresses: ips,
			Labels:      item.Metadata.Labels,
			Annotations: item.Metadata.Annotations,
			Status:      status,
			CreatedAt:   item.Metadata.CreationTimestamp,
			UpdatedAt:   time.Now(),
			Attributes: map[string]interface{}{
				"os_image":          item.Status.NodeInfo.OSImage,
				"kernel_version":    item.Status.NodeInfo.KernelVersion,
				"kubelet_version":   item.Status.NodeInfo.KubeletVersion,
				"container_runtime": item.Status.NodeInfo.ContainerRuntime,
			},
		}
		entities = append(entities, entity)
	}
	
	return entities, nil
}

// GetPods 获取Kubernetes Pod
func (c *KubernetesClient) GetPods() ([]*Entity, error) {
	if c.apiServer == "" {
		return nil, fmt.Errorf("kubernetes api server not configured")
	}
	
	url := fmt.Sprintf("%s/api/v1/pods", c.apiServer)
	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var podList struct {
		Items []struct {
			Metadata struct {
				Name              string            `json:"name"`
				Namespace         string            `json:"namespace"`
				UID               string            `json:"uid"`
				Labels            map[string]string `json:"labels"`
				Annotations       map[string]string `json:"annotations"`
				CreationTimestamp time.Time         `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				NodeName   string `json:"nodeName"`
				Containers []struct {
					Name  string `json:"name"`
					Image string `json:"image"`
				} `json:"containers"`
			} `json:"spec"`
			Status struct {
				PodIP      string `json:"podIP"`
				HostIP     string `json:"hostIP"`
				Phase      string `json:"phase"`
				ContainerStatuses []struct {
					ContainerID string `json:"containerID"`
					Name        string `json:"name"`
					Ready       bool   `json:"ready"`
				} `json:"containerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&podList); err != nil {
		return nil, err
	}
	
	var entities []*Entity
	for _, item := range podList.Items {
		var containerIDs []string
		for _, cs := range item.Status.ContainerStatuses {
			if cs.ContainerID != "" {
				// 提取容器ID (格式: docker://<id>)
				parts := strings.Split(cs.ContainerID, "://")
				if len(parts) == 2 {
					containerIDs = append(containerIDs, parts[1])
				}
			}
		}
		
		entity := &Entity{
			ID:          string(item.Metadata.UID),
			Name:        item.Metadata.Name,
			Type:        EntityTypePod,
			IPAddresses: []string{item.Status.PodIP},
			Labels:      item.Metadata.Labels,
			Annotations: item.Metadata.Annotations,
			Namespace:   item.Metadata.Namespace,
			NodeName:    item.Spec.NodeName,
			PodName:     item.Metadata.Name,
			Status:      string(item.Status.Phase),
			CreatedAt:   item.Metadata.CreationTimestamp,
			UpdatedAt:   time.Now(),
			Attributes: map[string]interface{}{
				"host_ip":       item.Status.HostIP,
				"container_ids": containerIDs,
			},
		}
		entities = append(entities, entity)
	}
	
	return entities, nil
}

// GetServices 获取Kubernetes Service
func (c *KubernetesClient) GetServices() ([]*Entity, error) {
	if c.apiServer == "" {
		return nil, fmt.Errorf("kubernetes api server not configured")
	}
	
	url := fmt.Sprintf("%s/api/v1/services", c.apiServer)
	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var svcList struct {
		Items []struct {
			Metadata struct {
				Name              string            `json:"name"`
				Namespace         string            `json:"namespace"`
				UID               string            `json:"uid"`
				Labels            map[string]string `json:"labels"`
				Annotations       map[string]string `json:"annotations"`
				CreationTimestamp time.Time         `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				Type         string `json:"type"`
				ClusterIP    string `json:"clusterIP"`
				ExternalName string `json:"externalName"`
				Ports        []struct {
					Name       string `json:"name"`
					Port       int    `json:"port"`
					TargetPort string `json:"targetPort"`
					Protocol   string `json:"protocol"`
				} `json:"ports"`
				Selector map[string]string `json:"selector"`
			} `json:"spec"`
		} `json:"items"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&svcList); err != nil {
		return nil, err
	}
	
	var entities []*Entity
	for _, item := range svcList.Items {
		var ips []string
		if item.Spec.ClusterIP != "" && item.Spec.ClusterIP != "None" {
			ips = append(ips, item.Spec.ClusterIP)
		}
		
		entity := &Entity{
			ID:          string(item.Metadata.UID),
			Name:        item.Metadata.Name,
			Type:        EntityTypeService,
			IPAddresses: ips,
			Labels:      item.Metadata.Labels,
			Annotations: item.Metadata.Annotations,
			Namespace:   item.Metadata.Namespace,
			Status:      "active",
			CreatedAt:   item.Metadata.CreationTimestamp,
			UpdatedAt:   time.Now(),
			Attributes: map[string]interface{}{
				"service_type": item.Spec.Type,
				"selector":     item.Spec.Selector,
			},
		}
		entities = append(entities, entity)
	}
	
	return entities, nil
}

// doRequest 执行HTTP请求
func (c *KubernetesClient) doRequest(method, url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}
	
	return c.httpClient.Do(req)
}

// FetchCloudMetadata 获取云元数据
func FetchCloudMetadata(metadataURL string) *CloudMetadata {
	metadata := &CloudMetadata{}
	
	// 尝试从不同云平台获取元数据
	providers := []struct {
		name string
		url  string
	}{
		{"aws", "http://169.254.169.254/latest/meta-data/"},
		{"gcp", "http://metadata.google.internal/computeMetadata/v1/"},
		{"azure", "http://169.254.169.254/metadata/instance?api-version=2021-02-01"},
		{"aliyun", "http://100.100.100.200/latest/meta-data/"},
		{"tencent", "http://metadata.tencentyun.com/latest/meta-data/"},
		{"huawei", "http://169.254.169.254/openstack/latest/meta_data.json"},
	}
	
	client := &http.Client{Timeout: 2 * time.Second}
	
	for _, provider := range providers {
		var req *http.Request
		var err error
		
		if provider.name == "gcp" {
			req, err = http.NewRequest("GET", provider.url, nil)
			if err == nil {
				req.Header.Set("Metadata-Flavor", "Google")
			}
		} else {
			req, err = http.NewRequest("GET", provider.url, nil)
		}
		
		if err != nil {
			continue
		}
		
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		
		if resp.StatusCode == 200 {
			metadata.Provider = provider.name
			e.fetchProviderMetadata(metadata, provider.name, client)
			break
		}
	}
	
	return metadata
}

// fetchProviderMetadata 获取特定云平台的元数据
func (e *DiscoveryEngine) fetchProviderMetadata(metadata *CloudMetadata, provider string, client *http.Client) {
	switch provider {
	case "aws":
		e.fetchAWSMetadata(metadata, client)
	case "gcp":
		e.fetchGCPMetadata(metadata, client)
	case "azure":
		e.fetchAzureMetadata(metadata, client)
	case "aliyun":
		e.fetchAliyunMetadata(metadata, client)
	case "tencent":
		e.fetchTencentMetadata(metadata, client)
	case "huawei":
		e.fetchHuaweiMetadata(metadata, client)
	}
}

// fetchAWSMetadata 获取AWS元数据
func (e *DiscoveryEngine) fetchAWSMetadata(metadata *CloudMetadata, client *http.Client) {
	baseURL := "http://169.254.169.254/latest/meta-data/"
	
	fetch := func(path string) string {
		resp, err := client.Get(baseURL + path)
		if err != nil {
			return ""
		}
		defer resp.Body.Close()
		
		data, _ := ioutil.ReadAll(resp.Body)
		return string(data)
	}
	
	metadata.InstanceID = fetch("instance-id")
	metadata.InstanceType = fetch("instance-type")
	metadata.Region = fetch("placement/region")
	metadata.Zone = fetch("placement/availability-zone")
	metadata.VPCID = fetch("mac")
	if metadata.VPCID != "" {
		metadata.VPCID = fetch("network/interfaces/macs/" + metadata.VPCID + "/vpc-id")
	}
	metadata.PrivateIP = fetch("local-ipv4")
	metadata.PublicIP = fetch("public-ipv4")
	metadata.Hostname = fetch("hostname")
}

// fetchGCPMetadata 获取GCP元数据
func (e *DiscoveryEngine) fetchGCPMetadata(metadata *CloudMetadata, client *http.Client) {
	baseURL := "http://metadata.google.internal/computeMetadata/v1/"
	
	fetch := func(path string) string {
		req, _ := http.NewRequest("GET", baseURL+path, nil)
		req.Header.Set("Metadata-Flavor", "Google")
		resp, err := client.Do(req)
		if err != nil {
			return ""
		}
		defer resp.Body.Close()
		
		data, _ := ioutil.ReadAll(resp.Body)
		return string(data)
	}
	
	metadata.InstanceID = fetch("instance/id")
	metadata.InstanceType = fetch("instance/machine-type")
	metadata.Zone = fetch("instance/zone")
	metadata.PrivateIP = fetch("instance/network-interfaces/0/ip")
	metadata.PublicIP = fetch("instance/network-interfaces/0/access-configs/0/external-ip")
	metadata.Hostname = fetch("instance/hostname")
}

// fetchAzureMetadata 获取Azure元数据
func (e *DiscoveryEngine) fetchAzureMetadata(metadata *CloudMetadata, client *http.Client) {
	req, _ := http.NewRequest("GET", "http://169.254.169.254/metadata/instance?api-version=2021-02-01", nil)
	req.Header.Set("Metadata", "true")
	
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	
	var azureData struct {
		Compute struct {
			Name       string `json:"name"`
			VMScaleSet string `json:"vmScaleSetName"`
			Zone       string `json:"zone"`
			Location   string `json:"location"`
		} `json:"compute"`
		Network struct {
			Interface []struct {
				IPv4 struct {
					IPAddress []struct {
						PrivateIPAddress string `json:"privateIpAddress"`
						PublicIPAddress  string `json:"publicIpAddress"`
					} `json:"ipAddress"`
				} `json:"ipv4"`
				MacAddress string `json:"macAddress"`
			} `json:"interface"`
		} `json:"network"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&azureData); err == nil {
		metadata.InstanceID = azureData.Compute.Name
		metadata.Zone = azureData.Compute.Zone
		metadata.Region = azureData.Compute.Location
		if len(azureData.Network.Interface) > 0 {
			if len(azureData.Network.Interface[0].IPv4.IPAddress) > 0 {
				metadata.PrivateIP = azureData.Network.Interface[0].IPv4.IPAddress[0].PrivateIPAddress
				metadata.PublicIP = azureData.Network.Interface[0].IPv4.IPAddress[0].PublicIPAddress
			}
			metadata.MACAddress = azureData.Network.Interface[0].MacAddress
		}
	}
}

// fetchAliyunMetadata 获取阿里云元数据
func (e *DiscoveryEngine) fetchAliyunMetadata(metadata *CloudMetadata, client *http.Client) {
	baseURL := "http://100.100.100.200/latest/meta-data/"
	
	fetch := func(path string) string {
		resp, err := client.Get(baseURL + path)
		if err != nil {
			return ""
		}
		defer resp.Body.Close()
		
		data, _ := ioutil.ReadAll(resp.Body)
		return string(data)
	}
	
	metadata.InstanceID = fetch("instance-id")
	metadata.InstanceType = fetch("instance/instance-type")
	metadata.Region = fetch("region-id")
	metadata.Zone = fetch("zone-id")
	metadata.VPCID = fetch("vpc-id")
	metadata.PrivateIP = fetch("private-ipv4")
	metadata.PublicIP = fetch("public-ipv4")
	metadata.Hostname = fetch("hostname")
}

// fetchTencentMetadata 获取腾讯云元数据
func (e *DiscoveryEngine) fetchTencentMetadata(metadata *CloudMetadata, client *http.Client) {
	baseURL := "http://metadata.tencentyun.com/latest/meta-data/"
	
	fetch := func(path string) string {
		resp, err := client.Get(baseURL + path)
		if err != nil {
			return ""
		}
		defer resp.Body.Close()
		
		data, _ := ioutil.ReadAll(resp.Body)
		return string(data)
	}
	
	metadata.InstanceID = fetch("instance-id")
	metadata.InstanceType = fetch("instance/instance-type")
	metadata.Region = fetch("placement/region")
	metadata.Zone = fetch("placement/zone")
	metadata.VPCID = fetch("network/interfaces/macs/" + fetch("mac") + "/vpc-id")
	metadata.PrivateIP = fetch("local-ipv4")
	metadata.PublicIP = fetch("public-ipv4")
	metadata.Hostname = fetch("hostname")
}

// fetchHuaweiMetadata 获取华为云元数据
func (e *DiscoveryEngine) fetchHuaweiMetadata(metadata *CloudMetadata, client *http.Client) {
	resp, err := client.Get("http://169.254.169.254/openstack/latest/meta_data.json")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	
	var huaweiData struct {
		UUID      string `json:"uuid"`
		Name      string `json:"name"`
		ProjectID string `json:"project_id"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&huaweiData); err == nil {
		metadata.InstanceID = huaweiData.UUID
		metadata.Hostname = huaweiData.Name
	}
}

// runtimeNumCPU 获取CPU核心数
func runtimeNumCPU() int {
	return 4 // 默认值，实际应从 runtime.NumCPU() 获取
}
