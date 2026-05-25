// Package huaweicloud 提供华为云Stack V8 API对接功能
// 本文件定义云资产模型（VM、VPC、子网、宿主机）
package huaweicloud

import (
	"fmt"
	"sync"
	"time"
)

// ==================== 资产类型定义 ====================

// AssetType 资产类型
type AssetType string

const (
	AssetTypeVM       AssetType = "vm"       // 虚拟机
	AssetTypeVPC      AssetType = "vpc"      // VPC
	AssetTypeSubnet   AssetType = "subnet"   // 子网
	AssetTypeHost     AssetType = "host"     // 宿主机
	AssetTypeDisk     AssetType = "disk"     // 云硬盘
	AssetTypeEIP      AssetType = "eip"      // 弹性IP
	AssetTypeSecurityGroup AssetType = "security_group" // 安全组
)

// String 返回资产类型字符串
func (t AssetType) String() string {
	return string(t)
}

// ==================== 基础资产接口 ====================

// CloudAsset 云资产接口
type CloudAsset interface {
	GetID() string
	GetName() string
	GetType() AssetType
	GetRegion() string
	GetProjectID() string
	GetTags() map[string]string
	GetMetadata() map[string]string
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
	GetStatus() string
}

// BaseAsset 基础资产
type BaseAsset struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        AssetType         `json:"type"`
	Region      string            `json:"region"`
	ProjectID   string            `json:"project_id"`
	ProjectName string            `json:"project_name"`
	DomainID    string            `json:"domain_id"`
	Tags        map[string]string `json:"tags"`
	Metadata    map[string]string `json:"metadata"`
	Status      string            `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// GetID 获取ID
func (a *BaseAsset) GetID() string {
	return a.ID
}

// GetName 获取名称
func (a *BaseAsset) GetName() string {
	return a.Name
}

// GetType 获取类型
func (a *BaseAsset) GetType() AssetType {
	return a.Type
}

// GetRegion 获取区域
func (a *BaseAsset) GetRegion() string {
	return a.Region
}

// GetProjectID 获取项目ID
func (a *BaseAsset) GetProjectID() string {
	return a.ProjectID
}

// GetTags 获取标签
func (a *BaseAsset) GetTags() map[string]string {
	return a.Tags
}

// GetMetadata 获取元数据
func (a *BaseAsset) GetMetadata() map[string]string {
	return a.Metadata
}

// GetCreatedAt 获取创建时间
func (a *BaseAsset) GetCreatedAt() time.Time {
	return a.CreatedAt
}

// GetUpdatedAt 获取更新时间
func (a *BaseAsset) GetUpdatedAt() time.Time {
	return a.UpdatedAt
}

// GetStatus 获取状态
func (a *BaseAsset) GetStatus() string {
	return a.Status
}

// ==================== 虚拟机(VM) ====================

// VM 虚拟机
type VM struct {
	BaseAsset
	
	// 计算资源
	FlavorID     string `json:"flavor_id"`     // 规格ID
	FlavorName   string `json:"flavor_name"`   // 规格名称
	vCPUs        int    `json:"vcpus"`         // CPU核数
	MemoryMB     int    `json:"memory_mb"`     // 内存(MB)
	DiskGB       int    `json:"disk_gb"`       // 系统盘(GB)
	
	// 网络配置
	VPCID        string   `json:"vpc_id"`        // VPC ID
	VPCName      string   `json:"vpc_name"`      // VPC名称
	SubnetIDs    []string `json:"subnet_ids"`    // 子网ID列表
	PrivateIPs   []string `json:"private_ips"`   // 内网IP列表
	PublicIP     string   `json:"public_ip"`     // 公网IP
	SecurityGroups []string `json:"security_groups"` // 安全组列表
	
	// 宿主机信息
	HostID       string `json:"host_id"`       // 宿主机ID
	HostName     string `json:"host_name"`     // 宿主机名称
	AZ           string `json:"az"`            // 可用区
	
	// 镜像信息
	ImageID      string `json:"image_id"`      // 镜像ID
	ImageName    string `json:"image_name"`    // 镜像名称
	OS           string `json:"os"`            // 操作系统
	
	// 其他
	KeyPair      string `json:"key_pair"`      // 密钥对
	UserData     string `json:"user_data"`     // 用户数据
	
	// 关联资源
	Volumes      []VolumeInfo `json:"volumes"`   // 挂载的云硬盘
}

// VolumeInfo 卷信息
type VolumeInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SizeGB   int    `json:"size_gb"`
	Type     string `json:"type"`
	Bootable bool   `json:"bootable"`
}

// ToLabels 转换为标签（用于注入观测数据）
func (vm *VM) ToLabels() map[string]string {
	labels := map[string]string{
		"cloud.provider":      "huaweicloud",
		"cloud.resource_type": "vm",
		"cloud.resource_id":   vm.ID,
		"cloud.vm.name":       vm.Name,
		"cloud.vm.flavor":     vm.FlavorName,
		"cloud.vm.vcpus":      fmt.Sprintf("%d", vm.vCPUs),
		"cloud.vm.memory_mb":  fmt.Sprintf("%d", vm.MemoryMB),
		"cloud.vpc.id":        vm.VPCID,
		"cloud.vpc.name":      vm.VPCName,
		"cloud.az":            vm.AZ,
		"cloud.region":        vm.Region,
		"cloud.project_id":    vm.ProjectID,
		"cloud.os":            vm.OS,
	}
	
	// 添加子网信息
	for i, subnetID := range vm.SubnetIDs {
		labels[fmt.Sprintf("cloud.subnet.%d.id", i)] = subnetID
	}
	
	// 添加IP信息
	for i, ip := range vm.PrivateIPs {
		labels[fmt.Sprintf("cloud.private_ip.%d", i)] = ip
	}
	if vm.PublicIP != "" {
		labels["cloud.public_ip"] = vm.PublicIP
	}
	
	// 添加宿主机信息
	if vm.HostID != "" {
		labels["cloud.host.id"] = vm.HostID
		labels["cloud.host.name"] = vm.HostName
	}
	
	// 合并自定义标签
	for k, v := range vm.Tags {
		labels["cloud.tag."+k] = v
	}
	
	return labels
}

// ==================== VPC ====================

// VPC 虚拟私有云
type VPC struct {
	BaseAsset
	
	CIDR        string `json:"cidr"`         // 网段
	IsDefault   bool   `json:"is_default"`   // 是否默认VPC
	
	// 子网列表
	Subnets     []SubnetInfo `json:"subnets"`
}

// SubnetInfo 子网信息
type SubnetInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	CIDR        string `json:"cidr"`
	GatewayIP   string `json:"gateway_ip"`
	AZ          string `json:"az"`
	Status      string `json:"status"`
}

// ToLabels 转换为标签
func (vpc *VPC) ToLabels() map[string]string {
	labels := map[string]string{
		"cloud.provider":      "huaweicloud",
		"cloud.resource_type": "vpc",
		"cloud.resource_id":   vpc.ID,
		"cloud.vpc.name":      vpc.Name,
		"cloud.vpc.cidr":      vpc.CIDR,
		"cloud.region":        vpc.Region,
		"cloud.project_id":    vpc.ProjectID,
	}
	
	if vpc.IsDefault {
		labels["cloud.vpc.is_default"] = "true"
	}
	
	// 合并自定义标签
	for k, v := range vpc.Tags {
		labels["cloud.tag."+k] = v
	}
	
	return labels
}

// ==================== 子网 ====================

// Subnet 子网
type Subnet struct {
	BaseAsset
	
	VPCID       string `json:"vpc_id"`       // 所属VPC ID
	VPCName     string `json:"vpc_name"`     // 所属VPC名称
	CIDR        string `json:"cidr"`         // 网段
	GatewayIP   string `json:"gateway_ip"`   // 网关IP
	DHCPEnable  bool   `json:"dhcp_enable"`  // DHCP是否启用
	AZ          string `json:"az"`           // 可用区
	
	// IP使用情况
	TotalIPs    int `json:"total_ips"`      // 总IP数
	UsedIPs     int `json:"used_ips"`       // 已使用IP数
	AvailableIPs int `json:"available_ips"` // 可用IP数
}

// ToLabels 转换为标签
func (s *Subnet) ToLabels() map[string]string {
	labels := map[string]string{
		"cloud.provider":      "huaweicloud",
		"cloud.resource_type": "subnet",
		"cloud.resource_id":   s.ID,
		"cloud.subnet.name":   s.Name,
		"cloud.subnet.cidr":   s.CIDR,
		"cloud.vpc.id":        s.VPCID,
		"cloud.vpc.name":      s.VPCName,
		"cloud.az":            s.AZ,
		"cloud.region":        s.Region,
		"cloud.project_id":    s.ProjectID,
	}
	
	// 合并自定义标签
	for k, v := range s.Tags {
		labels["cloud.tag."+k] = v
	}
	
	return labels
}

// ==================== 宿主机 ====================

// Host 宿主机（物理机）
type Host struct {
	BaseAsset
	
	HostType     string `json:"host_type"`     // 宿主机类型
	vCPUs        int    `json:"vcpus"`         // 总CPU核数
	vCPUsUsed    int    `json:"vcpus_used"`    // 已使用CPU
	MemoryMB     int    `json:"memory_mb"`     // 总内存(MB)
	MemoryUsedMB int    `json:"memory_used_mb"` // 已使用内存
	DiskGB       int    `json:"disk_gb"`       // 总磁盘(GB)
	DiskUsedGB   int    `json:"disk_used_gb"`  // 已使用磁盘
	
	AZ           string `json:"az"`            // 可用区
	Hypervisor   string `json:"hypervisor"`    // 虚拟化类型
	
	// 运行的VM列表
	VMIDs        []string `json:"vm_ids"`      // VM ID列表
	VMCount      int      `json:"vm_count"`    // VM数量
	
	// 状态
	State        string `json:"state"`         // 运行状态
}

// ToLabels 转换为标签
func (h *Host) ToLabels() map[string]string {
	labels := map[string]string{
		"cloud.provider":      "huaweicloud",
		"cloud.resource_type": "host",
		"cloud.resource_id":   h.ID,
		"cloud.host.name":     h.Name,
		"cloud.host.type":     h.HostType,
		"cloud.host.vcpus":    fmt.Sprintf("%d", h.vCPUs),
		"cloud.host.memory_mb": fmt.Sprintf("%d", h.MemoryMB),
		"cloud.az":            h.AZ,
		"cloud.region":        h.Region,
		"cloud.project_id":    h.ProjectID,
		"cloud.hypervisor":    h.Hypervisor,
	}
	
	// 合并自定义标签
	for k, v := range h.Tags {
		labels["cloud.tag."+k] = v
	}
	
	return labels
}

// ==================== 资产列表响应 ====================

// VMListResponse 虚拟机列表响应
type VMListResponse struct {
	Servers []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Status    string `json:"status"`
		Created   string `json:"created"`
		Updated   string `json:"updated"`
		Flavor    struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Disk int    `json:"disk"`
			RAM  int    `json:"ram"`
			vCPUs int   `json:"vcpus"`
		} `json:"flavor"`
		Image struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"image"`
		Addresses map[string][]struct {
			Version string `json:"version"`
			Addr    string `json:"addr"`
			Type    string `json:"OS-EXT-IPS:type"`
		} `json:"addresses"`
		Metadata    map[string]string `json:"metadata"`
		TenantID    string `json:"tenant_id"`
		UserID      string `json:"user_id"`
		KeyName     string `json:"key_name"`
		HostID      string `json:"hostId"`
		OSExtAZ     string `json:"OS-EXT-AZ:availability_zone"`
		OSExtHost   string `json:"OS-EXT-SRV-ATTR:host"`
		OSExtHyper  string `json:"OS-EXT-SRV-ATTR:hypervisor_hostname"`
	} `json:"servers"`
}

// VPCListResponse VPC列表响应
type VPCListResponse struct {
	Vpcs []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Cidr        string `json:"cidr"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
		TenantID    string `json:"tenant_id"`
		IsDefault   bool   `json:"is_default"`
		Subnets     []string `json:"subnets"`
	} `json:"vpcs"`
}

// SubnetListResponse 子网列表响应
type SubnetListResponse struct {
	Subnets []struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Cidr          string `json:"cidr"`
		GatewayIP     string `json:"gateway_ip"`
		NetworkID     string `json:"network_id"`
		TenantID      string `json:"tenant_id"`
		AvailabilityZone string `json:"availability_zone"`
		Status        string `json:"status"`
		TotalIPs      int    `json:"total_ips"`
		UsedIPs       int    `json:"used_ips"`
	} `json:"subnets"`
}

// HostListResponse 宿主机列表响应
type HostListResponse struct {
	Hosts []struct {
		ID               string `json:"id"`
		HostName         string `json:"host_name"`
		Service          string `json:"service"`
		Zone             string `json:"zone"`
		HostType         string `json:"host_type"`
		vCPUs            int    `json:"vcpus"`
		MemoryMB         int    `json:"memory_mb"`
		LocalGB          int    `json:"local_gb"`
		vCPUsUsed        int    `json:"vcpus_used"`
		MemoryMBUsed     int    `json:"memory_mb_used"`
		LocalGBUsed      int    `json:"local_gb_used"`
		HypervisorType   string `json:"hypervisor_type"`
		HypervisorVersion string `json:"hypervisor_version"`
		RunningVMs       int    `json:"running_vms"`
		State            string `json:"state"`
		Status           string `json:"status"`
	} `json:"hypervisors"`
}

// ==================== 资产存储 ====================

// AssetStore 资产存储接口
type AssetStore interface {
	SaveVM(vm *VM) error
	SaveVPC(vpc *VPC) error
	SaveSubnet(subnet *Subnet) error
	SaveHost(host *Host) error
	
	GetVM(id string) (*VM, error)
	GetVPC(id string) (*VPC, error)
	GetSubnet(id string) (*Subnet, error)
	GetHost(id string) (*Host, error)
	
	ListVMs() ([]*VM, error)
	ListVPCs() ([]*VPC, error)
	ListSubnets() ([]*Subnet, error)
	ListHosts() ([]*Host, error)
	
	DeleteVM(id string) error
	DeleteVPC(id string) error
	DeleteSubnet(id string) error
	DeleteHost(id string) error
	
	// 按条件查询
	FindVMsByHost(hostID string) ([]*VM, error)
	FindVMsByVPC(vpcID string) ([]*VM, error)
	FindVMsBySubnet(subnetID string) ([]*VM, error)
	FindSubnetsByVPC(vpcID string) ([]*Subnet, error)
}

// MemoryAssetStore 内存资产存储（用于测试）
type MemoryAssetStore struct {
	mu      sync.RWMutex
	vms     map[string]*VM
	vpcs    map[string]*VPC
	subnets map[string]*Subnet
	hosts   map[string]*Host
}

// NewMemoryAssetStore 创建内存资产存储
func NewMemoryAssetStore() *MemoryAssetStore {
	return &MemoryAssetStore{
		vms:     make(map[string]*VM),
		vpcs:    make(map[string]*VPC),
		subnets: make(map[string]*Subnet),
		hosts:   make(map[string]*Host),
	}
}

func (s *MemoryAssetStore) SaveVM(vm *VM) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vms[vm.ID] = vm
	return nil
}

func (s *MemoryAssetStore) SaveVPC(vpc *VPC) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vpcs[vpc.ID] = vpc
	return nil
}

func (s *MemoryAssetStore) SaveSubnet(subnet *Subnet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subnets[subnet.ID] = subnet
	return nil
}

func (s *MemoryAssetStore) SaveHost(host *Host) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hosts[host.ID] = host
	return nil
}

func (s *MemoryAssetStore) GetVM(id string) (*VM, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.vms[id], nil
}

func (s *MemoryAssetStore) GetVPC(id string) (*VPC, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.vpcs[id], nil
}

func (s *MemoryAssetStore) GetSubnet(id string) (*Subnet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.subnets[id], nil
}

func (s *MemoryAssetStore) GetHost(id string) (*Host, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hosts[id], nil
}

func (s *MemoryAssetStore) ListVMs() ([]*VM, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*VM, 0, len(s.vms))
	for _, vm := range s.vms {
		result = append(result, vm)
	}
	return result, nil
}

func (s *MemoryAssetStore) ListVPCs() ([]*VPC, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*VPC, 0, len(s.vpcs))
	for _, vpc := range s.vpcs {
		result = append(result, vpc)
	}
	return result, nil
}

func (s *MemoryAssetStore) ListSubnets() ([]*Subnet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Subnet, 0, len(s.subnets))
	for _, subnet := range s.subnets {
		result = append(result, subnet)
	}
	return result, nil
}

func (s *MemoryAssetStore) ListHosts() ([]*Host, error) {
	result := make([]*Host, 0, len(s.hosts))
	for _, host := range s.hosts {
		result = append(result, host)
	}
	return result, nil
}

func (s *MemoryAssetStore) DeleteVM(id string) error {
	delete(s.vms, id)
	return nil
}

func (s *MemoryAssetStore) DeleteVPC(id string) error {
	delete(s.vpcs, id)
	return nil
}

func (s *MemoryAssetStore) DeleteSubnet(id string) error {
	delete(s.subnets, id)
	return nil
}

func (s *MemoryAssetStore) DeleteHost(id string) error {
	delete(s.hosts, id)
	return nil
}

func (s *MemoryAssetStore) FindVMsByHost(hostID string) ([]*VM, error) {
	result := make([]*VM, 0)
	for _, vm := range s.vms {
		if vm.HostID == hostID {
			result = append(result, vm)
		}
	}
	return result, nil
}

func (s *MemoryAssetStore) FindVMsByVPC(vpcID string) ([]*VM, error) {
	result := make([]*VM, 0)
	for _, vm := range s.vms {
		if vm.VPCID == vpcID {
			result = append(result, vm)
		}
	}
	return result, nil
}

func (s *MemoryAssetStore) FindVMsBySubnet(subnetID string) ([]*VM, error) {
	result := make([]*VM, 0)
	for _, vm := range s.vms {
		for _, sid := range vm.SubnetIDs {
			if sid == subnetID {
				result = append(result, vm)
				break
			}
		}
	}
	return result, nil
}

func (s *MemoryAssetStore) FindSubnetsByVPC(vpcID string) ([]*Subnet, error) {
	result := make([]*Subnet, 0)
	for _, subnet := range s.subnets {
		if subnet.VPCID == vpcID {
			result = append(result, subnet)
		}
	}
	return result, nil
}
