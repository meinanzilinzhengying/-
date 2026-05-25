// Package huaweicloud 提供华为云Stack V8 API对接功能
// 本文件实现按云资源维度筛选和查询
package huaweicloud

import (
	"fmt"
	"strings"
	"sync"

	"cloud-flow-agent/pkg/logger"
)

// QueryService 资源查询服务
type QueryService struct {
	store  AssetStore
	log    *logger.Logger
	config QueryConfig
	
	// 索引
	vmByIP      map[string]*VM
	vmByHost    map[string]*VM
	vmByVPC     map[string][]*VM
	vmBySubnet  map[string][]*VM
	vmByHostID  map[string][]*VM
	indexMu     sync.RWMutex
}

// QueryConfig 查询配置
type QueryConfig struct {
	EnableIndexing bool `yaml:"enable_indexing" json:"enable_indexing"` // 启用索引加速
}

// DefaultQueryConfig 默认查询配置
func DefaultQueryConfig() QueryConfig {
	return QueryConfig{
		EnableIndexing: true,
	}
}

// NewQueryService 创建查询服务
func NewQueryService(store AssetStore, log *logger.Logger, config QueryConfig) *QueryService {
	s := &QueryService{
		store:      store,
		log:        log,
		config:     config,
		vmByIP:     make(map[string]*VM),
		vmByHost:   make(map[string]*VM),
		vmByVPC:    make(map[string][]*VM),
		vmBySubnet: make(map[string][]*VM),
		vmByHostID: make(map[string][]*VM),
	}
	
	// 构建索引
	if config.EnableIndexing {
		s.BuildIndex()
	}
	
	return s
}

// ==================== 索引方法 ====================

// BuildIndex 构建索引
func (s *QueryService) BuildIndex() error {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	
	// 清空旧索引
	s.vmByIP = make(map[string]*VM)
	s.vmByHost = make(map[string]*VM)
	s.vmByVPC = make(map[string][]*VM)
	s.vmBySubnet = make(map[string][]*VM)
	s.vmByHostID = make(map[string][]*VM)
	
	// 获取所有VM
	vms, err := s.store.ListVMs()
	if err != nil {
		return fmt.Errorf("获取VM列表失败: %w", err)
	}
	
	// 构建索引
	for _, vm := range vms {
		s.indexVM(vm)
	}
	
	s.log.Infof("查询索引已构建，共 %d 台VM", len(vms))
	return nil
}

// indexVM 索引单个VM
func (s *QueryService) indexVM(vm *VM) {
	// 按IP索引
	for _, ip := range vm.PrivateIPs {
		s.vmByIP[normalizeIP(ip)] = vm
	}
	if vm.PublicIP != "" {
		s.vmByIP[normalizeIP(vm.PublicIP)] = vm
	}
	
	// 按主机名索引
	s.vmByHost[vm.Name] = vm
	
	// 按VPC索引
	if vm.VPCID != "" {
		s.vmByVPC[vm.VPCID] = append(s.vmByVPC[vm.VPCID], vm)
	}
	
	// 按子网索引
	for _, subnetID := range vm.SubnetIDs {
		s.vmBySubnet[subnetID] = append(s.vmBySubnet[subnetID], vm)
	}
	
	// 按宿主机索引
	if vm.HostID != "" {
		s.vmByHostID[vm.HostID] = append(s.vmByHostID[vm.HostID], vm)
	}
}

// UpdateIndex 更新索引（单个VM）
func (s *QueryService) UpdateIndex(vm *VM) {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	
	s.indexVM(vm)
}

// ==================== 查询方法 ====================

// QueryVMs 查询虚拟机
func (s *QueryService) QueryVMs(filter VMFilter) ([]*VM, error) {
	// 如果有过滤条件，使用索引查询
	if s.config.EnableIndexing {
		return s.queryVMsWithIndex(filter)
	}
	
	// 否则全量查询后过滤
	return s.queryVMsWithFilter(filter)
}

// queryVMsWithIndex 使用索引查询
func (s *QueryService) queryVMsWithIndex(filter VMFilter) ([]*VM, error) {
	s.indexMu.RLock()
	defer s.indexMu.RUnlock()
	
	var result []*VM
	
	// 按IP查询（最精确）
	if filter.IP != "" {
		if vm, ok := s.vmByIP[normalizeIP(filter.IP)]; ok {
			if filter.match(vm) {
				return []*VM{vm}, nil
			}
		}
		return []*VM{}, nil
	}
	
	// 按主机名查询
	if filter.HostName != "" {
		if vm, ok := s.vmByHost[filter.HostName]; ok {
			if filter.match(vm) {
				return []*VM{vm}, nil
			}
		}
		return []*VM{}, nil
	}
	
	// 按VPC查询
	if filter.VPCID != "" {
		vms := s.vmByVPC[filter.VPCID]
		for _, vm := range vms {
			if filter.match(vm) {
				result = append(result, vm)
			}
		}
		return result, nil
	}
	
	// 按子网查询
	if filter.SubnetID != "" {
		vms := s.vmBySubnet[filter.SubnetID]
		for _, vm := range vms {
			if filter.match(vm) {
				result = append(result, vm)
			}
		}
		return result, nil
	}
	
	// 按宿主机查询
	if filter.HostID != "" {
		vms := s.vmByHostID[filter.HostID]
		for _, vm := range vms {
			if filter.match(vm) {
				result = append(result, vm)
			}
		}
		return result, nil
	}
	
	// 无索引条件，全量查询
	return s.queryVMsWithFilter(filter)
}

// queryVMsWithFilter 全量查询后过滤
func (s *QueryService) queryVMsWithFilter(filter VMFilter) ([]*VM, error) {
	vms, err := s.store.ListVMs()
	if err != nil {
		return nil, err
	}
	
	var result []*VM
	for _, vm := range vms {
		if filter.match(vm) {
			result = append(result, vm)
		}
	}
	
	return result, nil
}

// QueryVPCs 查询VPC
func (s *QueryService) QueryVPCs(filter VPCFilter) ([]*VPC, error) {
	vpcs, err := s.store.ListVPCs()
	if err != nil {
		return nil, err
	}
	
	var result []*VPC
	for _, vpc := range vpcs {
		if filter.match(vpc) {
			result = append(result, vpc)
		}
	}
	
	return result, nil
}

// QuerySubnets 查询子网
func (s *QueryService) QuerySubnets(filter SubnetFilter) ([]*Subnet, error) {
	subnets, err := s.store.ListSubnets()
	if err != nil {
		return nil, err
	}
	
	var result []*Subnet
	for _, subnet := range subnets {
		if filter.match(subnet) {
			result = append(result, subnet)
		}
	}
	
	return result, nil
}

// QueryHosts 查询宿主机
func (s *QueryService) QueryHosts(filter HostFilter) ([]*Host, error) {
	hosts, err := s.store.ListHosts()
	if err != nil {
		return nil, err
	}
	
	var result []*Host
	for _, host := range hosts {
		if filter.match(host) {
			result = append(result, host)
		}
	}
	
	return result, nil
}

// ==================== 关联查询 ====================

// GetVMsByHost 获取宿主机上的所有VM
func (s *QueryService) GetVMsByHost(hostID string) ([]*VM, error) {
	if s.config.EnableIndexing {
		s.indexMu.RLock()
		defer s.indexMu.RUnlock()
		return s.vmByHostID[hostID], nil
	}
	return s.store.FindVMsByHost(hostID)
}

// GetVMsByVPC 获取VPC下的所有VM
func (s *QueryService) GetVMsByVPC(vpcID string) ([]*VM, error) {
	if s.config.EnableIndexing {
		s.indexMu.RLock()
		defer s.indexMu.RUnlock()
		return s.vmByVPC[vpcID], nil
	}
	return s.store.FindVMsByVPC(vpcID)
}

// GetVMsBySubnet 获取子网下的所有VM
func (s *QueryService) GetVMsBySubnet(subnetID string) ([]*VM, error) {
	if s.config.EnableIndexing {
		s.indexMu.RLock()
		defer s.indexMu.RUnlock()
		return s.vmBySubnet[subnetID], nil
	}
	return s.store.FindVMsBySubnet(subnetID)
}

// GetSubnetsByVPC 获取VPC下的所有子网
func (s *QueryService) GetSubnetsByVPC(vpcID string) ([]*Subnet, error) {
	return s.store.FindSubnetsByVPC(vpcID)
}

// GetVMByIP 通过IP获取VM
func (s *QueryService) GetVMByIP(ip string) (*VM, error) {
	if s.config.EnableIndexing {
		s.indexMu.RLock()
		defer s.indexMu.RUnlock()
		return s.vmByIP[normalizeIP(ip)], nil
	}
	
	vms, err := s.store.ListVMs()
	if err != nil {
		return nil, err
	}
	
	for _, vm := range vms {
		for _, vmIP := range vm.PrivateIPs {
			if normalizeIP(vmIP) == normalizeIP(ip) {
				return vm, nil
			}
		}
		if normalizeIP(vm.PublicIP) == normalizeIP(ip) {
			return vm, nil
		}
	}
	
	return nil, nil
}

// GetVMByHostname 通过主机名获取VM
func (s *QueryService) GetVMByHostname(hostname string) (*VM, error) {
	if s.config.EnableIndexing {
		s.indexMu.RLock()
		defer s.indexMu.RUnlock()
		return s.vmByHost[hostname], nil
	}
	
	vms, err := s.store.ListVMs()
	if err != nil {
		return nil, err
	}
	
	for _, vm := range vms {
		if vm.Name == hostname {
			return vm, nil
		}
	}
	
	return nil, nil
}

// ==================== 过滤器定义 ====================

// VMFilter 虚拟机过滤器
type VMFilter struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	IP        string `json:"ip,omitempty"`
	HostName  string `json:"host_name,omitempty"`
	VPCID     string `json:"vpc_id,omitempty"`
	SubnetID  string `json:"subnet_id,omitempty"`
	HostID    string `json:"host_id,omitempty"`
	Region    string `json:"region,omitempty"`
	AZ        string `json:"az,omitempty"`
	Status    string `json:"status,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// match 检查VM是否匹配过滤器
func (f VMFilter) match(vm *VM) bool {
	if f.ID != "" && vm.ID != f.ID {
		return false
	}
	if f.Name != "" && !strings.Contains(vm.Name, f.Name) {
		return false
	}
	if f.VPCID != "" && vm.VPCID != f.VPCID {
		return false
	}
	if f.HostID != "" && vm.HostID != f.HostID {
		return false
	}
	if f.Region != "" && vm.Region != f.Region {
		return false
	}
	if f.AZ != "" && vm.AZ != f.AZ {
		return false
	}
	if f.Status != "" && vm.Status != f.Status {
		return false
	}
	if f.ProjectID != "" && vm.ProjectID != f.ProjectID {
		return false
	}
	if f.SubnetID != "" {
		found := false
		for _, sid := range vm.SubnetIDs {
			if sid == f.SubnetID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	// 标签匹配
	for k, v := range f.Tags {
		if vm.Tags[k] != v {
			return false
		}
	}
	return true
}

// VPCFilter VPC过滤器
type VPCFilter struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	CIDR      string `json:"cidr,omitempty"`
	Region    string `json:"region,omitempty"`
	Status    string `json:"status,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	IsDefault *bool  `json:"is_default,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// match 检查VPC是否匹配过滤器
func (f VPCFilter) match(vpc *VPC) bool {
	if f.ID != "" && vpc.ID != f.ID {
		return false
	}
	if f.Name != "" && !strings.Contains(vpc.Name, f.Name) {
		return false
	}
	if f.CIDR != "" && vpc.CIDR != f.CIDR {
		return false
	}
	if f.Region != "" && vpc.Region != f.Region {
		return false
	}
	if f.Status != "" && vpc.Status != f.Status {
		return false
	}
	if f.ProjectID != "" && vpc.ProjectID != f.ProjectID {
		return false
	}
	if f.IsDefault != nil && vpc.IsDefault != *f.IsDefault {
		return false
	}
	for k, v := range f.Tags {
		if vpc.Tags[k] != v {
			return false
		}
	}
	return true
}

// SubnetFilter 子网过滤器
type SubnetFilter struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	CIDR      string `json:"cidr,omitempty"`
	VPCID     string `json:"vpc_id,omitempty"`
	AZ        string `json:"az,omitempty"`
	Region    string `json:"region,omitempty"`
	Status    string `json:"status,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// match 检查子网是否匹配过滤器
func (f SubnetFilter) match(subnet *Subnet) bool {
	if f.ID != "" && subnet.ID != f.ID {
		return false
	}
	if f.Name != "" && !strings.Contains(subnet.Name, f.Name) {
		return false
	}
	if f.CIDR != "" && subnet.CIDR != f.CIDR {
		return false
	}
	if f.VPCID != "" && subnet.VPCID != f.VPCID {
		return false
	}
	if f.AZ != "" && subnet.AZ != f.AZ {
		return false
	}
	if f.Region != "" && subnet.Region != f.Region {
		return false
	}
	if f.Status != "" && subnet.Status != f.Status {
		return false
	}
	if f.ProjectID != "" && subnet.ProjectID != f.ProjectID {
		return false
	}
	for k, v := range f.Tags {
		if subnet.Tags[k] != v {
			return false
		}
	}
	return true
}

// HostFilter 宿主机过滤器
type HostFilter struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	HostType  string `json:"host_type,omitempty"`
	AZ        string `json:"az,omitempty"`
	Region    string `json:"region,omitempty"`
	Status    string `json:"status,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// match 检查宿主机是否匹配过滤器
func (f HostFilter) match(host *Host) bool {
	if f.ID != "" && host.ID != f.ID {
		return false
	}
	if f.Name != "" && !strings.Contains(host.Name, f.Name) {
		return false
	}
	if f.HostType != "" && host.HostType != f.HostType {
		return false
	}
	if f.AZ != "" && host.AZ != f.AZ {
		return false
	}
	if f.Region != "" && host.Region != f.Region {
		return false
	}
	if f.Status != "" && host.Status != f.Status {
		return false
	}
	if f.ProjectID != "" && host.ProjectID != f.ProjectID {
		return false
	}
	for k, v := range f.Tags {
		if host.Tags[k] != v {
			return false
		}
	}
	return true
}

// ==================== 统计查询 ====================

// GetAssetStats 获取资产统计
func (s *QueryService) GetAssetStats() map[string]interface{} {
	vms, _ := s.store.ListVMs()
	vpcs, _ := s.store.ListVPCs()
	subnets, _ := s.store.ListSubnets()
	hosts, _ := s.store.ListHosts()
	
	// 统计VM状态
	vmStatusCount := make(map[string]int)
	for _, vm := range vms {
		vmStatusCount[vm.Status]++
	}
	
	// 统计宿主机资源
	totalvCPUs := 0
	totalMemory := 0
	for _, host := range hosts {
		totalvCPUs += host.vCPUs
		totalMemory += host.MemoryMB
	}
	
	return map[string]interface{}{
		"vm_count":        len(vms),
		"vpc_count":       len(vpcs),
		"subnet_count":    len(subnets),
		"host_count":      len(hosts),
		"vm_status":       vmStatusCount,
		"total_vcpus":     totalvCPUs,
		"total_memory_mb": totalMemory,
	}
}

// GetIndexStats 获取索引统计
func (s *QueryService) GetIndexStats() map[string]interface{} {
	s.indexMu.RLock()
	defer s.indexMu.RUnlock()
	
	return map[string]interface{}{
		"indexed_ips":    len(s.vmByIP),
		"indexed_hosts":  len(s.vmByHost),
		"indexed_vpcs":   len(s.vmByVPC),
		"indexed_subnets": len(s.vmBySubnet),
		"indexed_hosts_vms": len(s.vmByHostID),
	}
}
