// Package huaweicloud 提供华为云Stack V8 API对接功能
// 本文件实现资产同步服务
package huaweicloud

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// SyncService 资产同步服务
type SyncService struct {
	client      *Client
	store       AssetStore
	log         *logger.Logger
	config      SyncConfig
	
	// 控制
	stopCh      chan struct{}
	wg          sync.WaitGroup
	isRunning   bool
	mu          sync.RWMutex
	
	// 回调
	onVMSynced  func([]*VM)
	onVPCSynced func([]*VPC)
	onSubnetSynced func([]*Subnet)
	onHostSynced   func([]*Host)
}

// SyncConfig 同步配置
type SyncConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	Interval          time.Duration `yaml:"interval" json:"interval"`                     // 同步间隔
	VMEnabled         bool          `yaml:"vm_enabled" json:"vm_enabled"`                 // 同步VM
	VPCEnabled        bool          `yaml:"vpc_enabled" json:"vpc_enabled"`               // 同步VPC
	SubnetEnabled     bool          `yaml:"subnet_enabled" json:"subnet_enabled"`         // 同步子网
	HostEnabled       bool          `yaml:"host_enabled" json:"host_enabled"`             // 同步宿主机
	FullSyncOnStart   bool          `yaml:"full_sync_on_start" json:"full_sync_on_start"` // 启动时全量同步
	IncrementalSync   bool          `yaml:"incremental_sync" json:"incremental_sync"`     // 增量同步
	BatchSize         int           `yaml:"batch_size" json:"batch_size"`                 // 批量大小
}

// DefaultSyncConfig 默认同步配置
func DefaultSyncConfig() SyncConfig {
	return SyncConfig{
		Enabled:         true,
		Interval:        5 * time.Minute,
		VMEnabled:       true,
		VPCEnabled:      true,
		SubnetEnabled:   true,
		HostEnabled:     true,
		FullSyncOnStart: true,
		IncrementalSync: true,
		BatchSize:       100,
	}
}

// NewSyncService 创建同步服务
func NewSyncService(client *Client, store AssetStore, log *logger.Logger, config SyncConfig) *SyncService {
	return &SyncService{
		client:    client,
		store:     store,
		log:       log,
		config:    config,
		stopCh:    make(chan struct{}),
	}
}

// SetVMSyncCallback 设置VM同步回调
func (s *SyncService) SetVMSyncCallback(callback func([]*VM)) {
	s.onVMSynced = callback
}

// SetVPCSyncCallback 设置VPC同步回调
func (s *SyncService) SetVPCSyncCallback(callback func([]*VPC)) {
	s.onVPCSynced = callback
}

// SetSubnetSyncCallback 设置子网同步回调
func (s *SyncService) SetSubnetSyncCallback(callback func([]*Subnet)) {
	s.onSubnetSynced = callback
}

// SetHostSyncCallback 设置宿主机同步回调
func (s *SyncService) SetHostSyncCallback(callback func([]*Host)) {
	s.onHostSynced = callback
}

// Start 启动同步服务
func (s *SyncService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.isRunning {
		return fmt.Errorf("同步服务已在运行")
	}
	
	if !s.config.Enabled {
		s.log.Info("资产同步服务已禁用")
		return nil
	}
	
	s.isRunning = true
	s.stopCh = make(chan struct{})
	
	// 启动时全量同步
	if s.config.FullSyncOnStart {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.log.Info("执行启动时全量同步...")
			if err := s.FullSync(); err != nil {
				s.log.Errorf("启动时全量同步失败: %v", err)
			}
		}()
	}
	
	// 启动定时同步
	s.wg.Add(1)
	go s.syncLoop()
	
	s.log.Infof("资产同步服务已启动，同步间隔: %v", s.config.Interval)
	return nil
}

// Stop 停止同步服务
func (s *SyncService) Stop() {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = false
	s.mu.Unlock()
	
	close(s.stopCh)
	s.wg.Wait()
	s.log.Info("资产同步服务已停止")
}

// IsRunning 检查是否正在运行
func (s *SyncService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// syncLoop 同步循环
func (s *SyncService) syncLoop() {
	defer s.wg.Done()
	
	ticker := time.NewTicker(s.config.Interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			if err := s.IncrementalSync(); err != nil {
				s.log.Errorf("增量同步失败: %v", err)
			}
		}
	}
}

// FullSync 全量同步
func (s *SyncService) FullSync() error {
	s.log.Info("开始全量同步...")
	
	var errors []error
	
	// 同步宿主机（先同步，因为VM依赖宿主机信息）
	if s.config.HostEnabled {
		if err := s.syncHosts(); err != nil {
			s.log.Errorf("同步宿主机失败: %v", err)
			errors = append(errors, err)
		}
	}
	
	// 同步VPC
	if s.config.VPCEnabled {
		if err := s.syncVPCs(); err != nil {
			s.log.Errorf("同步VPC失败: %v", err)
			errors = append(errors, err)
		}
	}
	
	// 同步子网
	if s.config.SubnetEnabled {
		if err := s.syncSubnets(); err != nil {
			s.log.Errorf("同步子网失败: %v", err)
			errors = append(errors, err)
		}
	}
	
	// 同步VM
	if s.config.VMEnabled {
		if err := s.syncVMs(); err != nil {
			s.log.Errorf("同步VM失败: %v", err)
			errors = append(errors, err)
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("全量同步部分失败: %d个错误", len(errors))
	}
	
	s.log.Info("全量同步完成")
	return nil
}

// IncrementalSync 增量同步
func (s *SyncService) IncrementalSync() error {
	s.log.Debug("开始增量同步...")
	
	// 增量同步逻辑：只同步最近变更的资源
	// 实际实现中可以使用API的过滤参数，如 updated_at_gt
	
	// 这里简化处理，直接调用全量同步
	// 实际生产环境应该实现真正的增量同步
	return s.FullSync()
}

// ==================== 具体同步方法 ====================

// syncVMs 同步虚拟机
func (s *SyncService) syncVMs() error {
	s.log.Debug("同步虚拟机...")
	
	var resp VMListResponse
	if err := s.client.Get("ecs", "v2.1/servers/detail", nil, &resp); err != nil {
		return fmt.Errorf("获取VM列表失败: %w", err)
	}
	
	vms := make([]*VM, 0, len(resp.Servers))
	
	for _, server := range resp.Servers {
		vm := s.convertServerToVM(server)
		
		// 保存到存储
		if err := s.store.SaveVM(vm); err != nil {
			s.log.Errorf("保存VM失败 %s: %v", vm.ID, err)
			continue
		}
		
		vms = append(vms, vm)
	}
	
	s.log.Infof("同步了 %d 台虚拟机", len(vms))
	
	// 触发回调
	if s.onVMSynced != nil {
		s.onVMSynced(vms)
	}
	
	return nil
}

// syncVPCs 同步VPC
func (s *SyncService) syncVPCs() error {
	s.log.Debug("同步VPC...")
	
	var resp VPCListResponse
	if err := s.client.Get("vpc", "v2.0/vpcs", nil, &resp); err != nil {
		return fmt.Errorf("获取VPC列表失败: %w", err)
	}
	
	vpcs := make([]*VPC, 0, len(resp.Vpcs))
	
	for _, v := range resp.Vpcs {
		vpc := s.convertVpcToVPC(v)
		
		// 获取子网详情
		subnets := make([]SubnetInfo, 0)
		for _, subnetID := range v.Subnets {
			subnet, err := s.store.GetSubnet(subnetID)
			if err == nil && subnet != nil {
				subnets = append(subnets, SubnetInfo{
					ID:   subnet.ID,
					Name: subnet.Name,
					CIDR: subnet.CIDR,
				})
			}
		}
		vpc.Subnets = subnets
		
		// 保存到存储
		if err := s.store.SaveVPC(vpc); err != nil {
			s.log.Errorf("保存VPC失败 %s: %v", vpc.ID, err)
			continue
		}
		
		vpcs = append(vpcs, vpc)
	}
	
	s.log.Infof("同步了 %d 个VPC", len(vpcs))
	
	// 触发回调
	if s.onVPCSynced != nil {
		s.onVPCSynced(vpcs)
	}
	
	return nil
}

// syncSubnets 同步子网
func (s *SyncService) syncSubnets() error {
	s.log.Debug("同步子网...")
	
	var resp SubnetListResponse
	if err := s.client.Get("vpc", "v2.0/subnets", nil, &resp); err != nil {
		return fmt.Errorf("获取子网列表失败: %w", err)
	}
	
	subnets := make([]*Subnet, 0, len(resp.Subnets))
	
	for _, snet := range resp.Subnets {
		subnet := s.convertSubnetToSubnet(snet)
		
		// 保存到存储
		if err := s.store.SaveSubnet(subnet); err != nil {
			s.log.Errorf("保存子网失败 %s: %v", subnet.ID, err)
			continue
		}
		
		subnets = append(subnets, subnet)
	}
	
	s.log.Infof("同步了 %d 个子网", len(subnets))
	
	// 触发回调
	if s.onSubnetSynced != nil {
		s.onSubnetSynced(subnets)
	}
	
	return nil
}

// syncHosts 同步宿主机
func (s *SyncService) syncHosts() error {
	s.log.Debug("同步宿主机...")
	
	var resp HostListResponse
	if err := s.client.Get("ecs", "v2.1/os-hypervisors/detail", nil, &resp); err != nil {
		return fmt.Errorf("获取宿主机列表失败: %w", err)
	}
	
	hosts := make([]*Host, 0, len(resp.Hosts))
	
	for _, h := range resp.Hosts {
		host := s.convertHypervisorToHost(h)
		
		// 获取该宿主机上的VM列表
		vms, _ := s.store.FindVMsByHost(host.ID)
		host.VMIDs = make([]string, 0, len(vms))
		for _, vm := range vms {
			host.VMIDs = append(host.VMIDs, vm.ID)
		}
		host.VMCount = len(vms)
		
		// 保存到存储
		if err := s.store.SaveHost(host); err != nil {
			s.log.Errorf("保存宿主机失败 %s: %v", host.ID, err)
			continue
		}
		
		hosts = append(hosts, host)
	}
	
	s.log.Infof("同步了 %d 台宿主机", len(hosts))
	
	// 触发回调
	if s.onHostSynced != nil {
		s.onHostSynced(hosts)
	}
	
	return nil
}

// ==================== 数据转换方法 ====================

// convertServerToVM 将API响应转换为VM模型
func (s *SyncService) convertServerToVM(server struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Created   string `json:"created"`
	Updated   string `json:"updated"`
	Flavor    struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Disk  int    `json:"disk"`
		RAM   int    `json:"ram"`
		vCPUs int    `json:"vcpus"`
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
}) *VM {
	vm := &VM{
		BaseAsset: BaseAsset{
			ID:        server.ID,
			Name:      server.Name,
			Type:      AssetTypeVM,
			ProjectID: server.TenantID,
			Status:    server.Status,
			Metadata:  server.Metadata,
			Tags:      make(map[string]string),
		},
		FlavorID:   server.Flavor.ID,
		FlavorName: server.Flavor.Name,
		vCPUs:      server.Flavor.vCPUs,
		MemoryMB:   server.Flavor.RAM,
		DiskGB:     server.Flavor.Disk,
		ImageID:    server.Image.ID,
		ImageName:  server.Image.Name,
		KeyPair:    server.KeyName,
		HostID:     server.HostID,
		AZ:         server.OSExtAZ,
		HostName:   server.OSExtHost,
	}
	
	// 解析时间
	if created, err := time.Parse(time.RFC3339, server.Created); err == nil {
		vm.CreatedAt = created
	}
	if updated, err := time.Parse(time.RFC3339, server.Updated); err == nil {
		vm.UpdatedAt = updated
	}
	
	// 解析IP地址
	vm.PrivateIPs = make([]string, 0)
	for _, addrs := range server.Addresses {
		for _, addr := range addrs {
			if addr.Type == "fixed" {
				vm.PrivateIPs = append(vm.PrivateIPs, addr.Addr)
			} else if addr.Type == "floating" {
				vm.PublicIP = addr.Addr
			}
		}
	}
	
	// 从metadata提取VPC和子网信息
	if vpcID, ok := server.Metadata["vpc_id"]; ok {
		vm.VPCID = vpcID
	}
	if subnetID, ok := server.Metadata["subnet_id"]; ok {
		vm.SubnetIDs = append(vm.SubnetIDs, subnetID)
	}
	
	// 从metadata提取OS信息
	if os, ok := server.Metadata["os_type"]; ok {
		vm.OS = os
	}
	
	return vm
}

// convertVpcToVPC 将API响应转换为VPC模型
func (s *SyncService) convertVpcToVPC(v struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Cidr        string   `json:"cidr"`
	Status      string   `json:"status"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	TenantID    string   `json:"tenant_id"`
	IsDefault   bool     `json:"is_default"`
	Subnets     []string `json:"subnets"`
}) *VPC {
	vpc := &VPC{
		BaseAsset: BaseAsset{
			ID:        v.ID,
			Name:      v.Name,
			Type:      AssetTypeVPC,
			ProjectID: v.TenantID,
			Status:    v.Status,
			Tags:      make(map[string]string),
			Metadata:  make(map[string]string),
		},
		CIDR:      v.Cidr,
		IsDefault: v.IsDefault,
	}
	
	// 解析时间
	if created, err := time.Parse(time.RFC3339, v.CreatedAt); err == nil {
		vpc.CreatedAt = created
	}
	if updated, err := time.Parse(time.RFC3339, v.UpdatedAt); err == nil {
		vpc.UpdatedAt = updated
	}
	
	return vpc
}

// convertSubnetToSubnet 将API响应转换为子网模型
func (s *SyncService) convertSubnetToSubnet(snet struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Cidr             string `json:"cidr"`
	GatewayIP        string `json:"gateway_ip"`
	NetworkID        string `json:"network_id"`
	TenantID         string `json:"tenant_id"`
	AvailabilityZone string `json:"availability_zone"`
	Status           string `json:"status"`
	TotalIPs         int    `json:"total_ips"`
	UsedIPs          int    `json:"used_ips"`
}) *Subnet {
	subnet := &Subnet{
		BaseAsset: BaseAsset{
			ID:        snet.ID,
			Name:      snet.Name,
			Type:      AssetTypeSubnet,
			ProjectID: snet.TenantID,
			Status:    snet.Status,
			Tags:      make(map[string]string),
			Metadata:  make(map[string]string),
		},
		CIDR:         snet.Cidr,
		GatewayIP:    snet.GatewayIP,
		AZ:           snet.AvailabilityZone,
		TotalIPs:     snet.TotalIPs,
		UsedIPs:      snet.UsedIPs,
		AvailableIPs: snet.TotalIPs - snet.UsedIPs,
	}
	
	return subnet
}

// convertHypervisorToHost 将API响应转换为宿主机模型
func (s *SyncService) convertHypervisorToHost(h struct {
	ID                string `json:"id"`
	HostName          string `json:"host_name"`
	Service           string `json:"service"`
	Zone              string `json:"zone"`
	HostType          string `json:"host_type"`
	vCPUs             int    `json:"vcpus"`
	MemoryMB          int    `json:"memory_mb"`
	LocalGB           int    `json:"local_gb"`
	vCPUsUsed         int    `json:"vcpus_used"`
	MemoryMBUsed      int    `json:"memory_mb_used"`
	LocalGBUsed       int    `json:"local_gb_used"`
	HypervisorType    string `json:"hypervisor_type"`
	HypervisorVersion string `json:"hypervisor_version"`
	RunningVMs        int    `json:"running_vms"`
	State             string `json:"state"`
	Status            string `json:"status"`
}) *Host {
	host := &Host{
		BaseAsset: BaseAsset{
			ID:       h.ID,
			Name:     h.HostName,
			Type:     AssetTypeHost,
			Status:   h.Status,
			Tags:     make(map[string]string),
			Metadata: make(map[string]string),
		},
		HostType:     h.HostType,
		vCPUs:        h.vCPUs,
		vCPUsUsed:    h.vCPUsUsed,
		MemoryMB:     h.MemoryMB,
		MemoryUsedMB: h.MemoryMBUsed,
		DiskGB:       h.LocalGB,
		DiskUsedGB:   h.LocalGBUsed,
		AZ:           h.Zone,
		Hypervisor:   h.HypervisorType,
		VMCount:      h.RunningVMs,
		State:        h.State,
	}
	
	return host
}

// GetSyncStatus 获取同步状态
func (s *SyncService) GetSyncStatus() map[string]interface{} {
	return map[string]interface{}{
		"is_running":    s.IsRunning(),
		"interval":      s.config.Interval.String(),
		"vm_enabled":    s.config.VMEnabled,
		"vpc_enabled":   s.config.VPCEnabled,
		"subnet_enabled": s.config.SubnetEnabled,
		"host_enabled":  s.config.HostEnabled,
	}
}
