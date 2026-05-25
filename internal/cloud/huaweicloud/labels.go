// Package huaweicloud 提供华为云Stack V8 API对接功能
// 本文件实现VM元数据标签自动注入服务
package huaweicloud

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// LabelInjector 标签注入器
type LabelInjector struct {
	store       AssetStore
	log         *logger.Logger
	config      LabelConfig
	
	// IP到VM的映射缓存
	ipVMCache   map[string]*VM
	ipCacheMu   sync.RWMutex
	cacheExpire time.Duration
}

// LabelConfig 标签注入配置
type LabelConfig struct {
	Enabled           bool     `yaml:"enabled" json:"enabled"`
	InjectVMLabels    bool     `yaml:"inject_vm_labels" json:"inject_vm_labels"`       // 注入VM标签
	InjectVPCLabels   bool     `yaml:"inject_vpc_labels" json:"inject_vpc_labels"`     // 注入VPC标签
	InjectHostLabels  bool     `yaml:"inject_host_labels" json:"inject_host_labels"`   // 注入宿主机标签
	LabelPrefix       string   `yaml:"label_prefix" json:"label_prefix"`               // 标签前缀
	CacheTTL          int      `yaml:"cache_ttl" json:"cache_ttl"`                     // 缓存TTL（秒）
	MatchFields       []string `yaml:"match_fields" json:"match_fields"`               // 匹配字段
}

// DefaultLabelConfig 默认标签配置
func DefaultLabelConfig() LabelConfig {
	return LabelConfig{
		Enabled:          true,
		InjectVMLabels:   true,
		InjectVPCLabels:  true,
		InjectHostLabels: true,
		LabelPrefix:      "cloud.",
		CacheTTL:         300,
		MatchFields:      []string{"instance", "host", "ip", "source"},
	}
}

// NewLabelInjector 创建标签注入器
func NewLabelInjector(store AssetStore, log *logger.Logger, config LabelConfig) *LabelInjector {
	return &LabelInjector{
		store:       store,
		log:         log,
		config:      config,
		ipVMCache:   make(map[string]*VM),
		cacheExpire: time.Duration(config.CacheTTL) * time.Second,
	}
}

// ==================== 标签注入方法 ====================

// InjectLabels 为观测数据注入云资产标签
// data: 原始观测数据（包含instance, host, ip等字段）
// 返回: 注入标签后的数据
func (i *LabelInjector) InjectLabels(data map[string]string) map[string]string {
	if !i.config.Enabled {
		return data
	}
	
	// 复制原始数据
	result := make(map[string]string)
	for k, v := range data {
		result[k] = v
	}
	
	// 尝试匹配VM
	vm := i.matchVM(data)
	if vm == nil {
		return result
	}
	
	// 注入VM标签
	if i.config.InjectVMLabels {
		vmLabels := vm.ToLabels()
		for k, v := range vmLabels {
			result[i.config.LabelPrefix+k] = v
		}
	}
	
	// 注入VPC标签
	if i.config.InjectVPCLabels && vm.VPCID != "" {
		vpc, err := i.store.GetVPC(vm.VPCID)
		if err == nil && vpc != nil {
			vpcLabels := vpc.ToLabels()
			for k, v := range vpcLabels {
				result[i.config.LabelPrefix+k] = v
			}
		}
	}
	
	// 注入宿主机标签
	if i.config.InjectHostLabels && vm.HostID != "" {
		host, err := i.store.GetHost(vm.HostID)
		if err == nil && host != nil {
			hostLabels := host.ToLabels()
			for k, v := range hostLabels {
				result[i.config.LabelPrefix+k] = v
			}
		}
	}
	
	return result
}

// InjectLabelsToMetrics 为指标数据注入标签
func (i *LabelInjector) InjectLabelsToMetrics(metrics []MetricData) []MetricData {
	if !i.config.Enabled {
		return metrics
	}
	
	result := make([]MetricData, len(metrics))
	for idx, metric := range metrics {
		result[idx] = metric
		
		// 将metric转换为map进行标签注入
		data := make(map[string]string)
		data["instance"] = metric.Labels["instance"]
		data["host"] = metric.Labels["host"]
		data["ip"] = metric.Labels["ip"]
		
		// 注入标签
		injected := i.InjectLabels(data)
		
		// 将注入的标签合并回metric.Labels
		for k, v := range injected {
			if strings.HasPrefix(k, i.config.LabelPrefix) {
				result[idx].Labels[k] = v
			}
		}
	}
	
	return result
}

// InjectLabelsToAlert 为告警注入标签
func (i *LabelInjector) InjectLabelsToAlert(alert *AlertEvent) *AlertEvent {
	if !i.config.Enabled || alert == nil {
		return alert
	}
	
	// 构建匹配数据
	data := make(map[string]string)
	data["instance"] = alert.Labels["instance"]
	data["host"] = alert.Labels["host"]
	data["ip"] = alert.Labels["ip"]
	
	// 注入标签
	injected := i.InjectLabels(data)
	
	// 合并到告警标签
	for k, v := range injected {
		if strings.HasPrefix(k, i.config.LabelPrefix) {
			alert.Labels[k] = v
		}
	}
	
	// 添加云资产信息到注解
	if alert.Annotations == nil {
		alert.Annotations = make(map[string]string)
	}
	
	vm := i.matchVM(data)
	if vm != nil {
		alert.Annotations["cloud.vm.id"] = vm.ID
		alert.Annotations["cloud.vm.name"] = vm.Name
		alert.Annotations["cloud.vm.status"] = vm.Status
		
		if vm.VPCID != "" {
			alert.Annotations["cloud.vpc.id"] = vm.VPCID
		}
		if vm.HostID != "" {
			alert.Annotations["cloud.host.id"] = vm.HostID
		}
	}
	
	return alert
}

// ==================== VM匹配方法 ====================

// matchVM 根据观测数据匹配VM
func (i *LabelInjector) matchVM(data map[string]string) *VM {
	// 按优先级尝试匹配
	
	// 1. 尝试通过instance字段匹配（通常是IP或主机名）
	if instance := data["instance"]; instance != "" {
		if vm := i.matchByInstance(instance); vm != nil {
			return vm
		}
	}
	
	// 2. 尝试通过host字段匹配
	if host := data["host"]; host != "" {
		if vm := i.matchByHost(host); vm != nil {
			return vm
		}
	}
	
	// 3. 尝试通过IP匹配
	if ip := data["ip"]; ip != "" {
		if vm := i.matchByIP(ip); vm != nil {
			return vm
		}
	}
	
	// 4. 尝试通过source字段匹配
	if source := data["source"]; source != "" {
		if vm := i.matchBySource(source); vm != nil {
			return vm
		}
	}
	
	return nil
}

// matchByInstance 通过instance匹配
func (i *LabelInjector) matchByInstance(instance string) *VM {
	// 先检查缓存
	i.ipCacheMu.RLock()
	if vm, ok := i.ipVMCache[instance]; ok {
		i.ipCacheMu.RUnlock()
		return vm
	}
	i.ipCacheMu.RUnlock()
	
	// 尝试作为IP匹配
	if net.ParseIP(instance) != nil {
		return i.matchByIP(instance)
	}
	
	// 尝试作为主机名匹配
	vms, _ := i.store.ListVMs()
	for _, vm := range vms {
		if vm.Name == instance {
			i.cacheVM(instance, vm)
			return vm
		}
	}
	
	return nil
}

// matchByHost 通过host匹配
func (i *LabelInjector) matchByHost(host string) *VM {
	// 检查缓存
	i.ipCacheMu.RLock()
	if vm, ok := i.ipVMCache[host]; ok {
		i.ipCacheMu.RUnlock()
		return vm
	}
	i.ipCacheMu.RUnlock()
	
	// 尝试作为IP匹配
	if net.ParseIP(host) != nil {
		return i.matchByIP(host)
	}
	
	// 尝试作为主机名匹配
	vms, _ := i.store.ListVMs()
	for _, vm := range vms {
		if vm.Name == host {
			i.cacheVM(host, vm)
			return vm
		}
	}
	
	return nil
}

// matchByIP 通过IP匹配
func (i *LabelInjector) matchByIP(ip string) *VM {
	// 标准化IP
	ip = normalizeIP(ip)
	if ip == "" {
		return nil
	}
	
	// 检查缓存
	i.ipCacheMu.RLock()
	if vm, ok := i.ipVMCache[ip]; ok {
		i.ipCacheMu.RUnlock()
		return vm
	}
	i.ipCacheMu.RUnlock()
	
	// 查询所有VM
	vms, _ := i.store.ListVMs()
	for _, vm := range vms {
		// 匹配私网IP
		for _, vmIP := range vm.PrivateIPs {
			if normalizeIP(vmIP) == ip {
				i.cacheVM(ip, vm)
				return vm
			}
		}
		// 匹配公网IP
		if normalizeIP(vm.PublicIP) == ip {
			i.cacheVM(ip, vm)
			return vm
		}
	}
	
	return nil
}

// matchBySource 通过source匹配
func (i *LabelInjector) matchBySource(source string) *VM {
	// 检查缓存
	i.ipCacheMu.RLock()
	if vm, ok := i.ipVMCache[source]; ok {
		i.ipCacheMu.RUnlock()
		return vm
	}
	i.ipCacheMu.RUnlock()
	
	// 尝试作为IP匹配
	if net.ParseIP(source) != nil {
		return i.matchByIP(source)
	}
	
	// 尝试作为主机名匹配
	vms, _ := i.store.ListVMs()
	for _, vm := range vms {
		if vm.Name == source {
			i.cacheVM(source, vm)
			return vm
		}
	}
	
	return nil
}

// cacheVM 缓存VM
func (i *LabelInjector) cacheVM(key string, vm *VM) {
	i.ipCacheMu.Lock()
	defer i.ipCacheMu.Unlock()
	
	i.ipVMCache[key] = vm
	
	// 同时缓存所有IP
	for _, ip := range vm.PrivateIPs {
		i.ipVMCache[normalizeIP(ip)] = vm
	}
	if vm.PublicIP != "" {
		i.ipVMCache[normalizeIP(vm.PublicIP)] = vm
	}
}

// ClearCache 清空缓存
func (i *LabelInjector) ClearCache() {
	i.ipCacheMu.Lock()
	defer i.ipCacheMu.Unlock()
	
	i.ipVMCache = make(map[string]*VM)
}

// RefreshCache 刷新缓存
func (i *LabelInjector) RefreshCache() error {
	i.ClearCache()
	
	// 预加载所有VM到缓存
	vms, err := i.store.ListVMs()
	if err != nil {
		return fmt.Errorf("加载VM列表失败: %w", err)
	}
	
	for _, vm := range vms {
		// 缓存VM名称
		i.cacheVM(vm.Name, vm)
		
		// 缓存VM ID
		i.cacheVM(vm.ID, vm)
	}
	
	i.log.Infof("标签注入器缓存已刷新，共 %d 台VM", len(vms))
	return nil
}

// ==================== 辅助函数 ====================

// normalizeIP 标准化IP地址
func normalizeIP(ip string) string {
	ip = strings.TrimSpace(ip)
	
	// 处理带端口的地址
	if idx := strings.LastIndex(ip, ":"); idx > 0 {
		// 检查是否是IPv6
		if !strings.Contains(ip, "]") {
			ip = ip[:idx]
		}
	}
	
	// 解析IP
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return ""
	}
	
	// 返回标准化格式
	return parsedIP.String()
}

// GetCacheStats 获取缓存统计
func (i *LabelInjector) GetCacheStats() map[string]interface{} {
	i.ipCacheMu.RLock()
	defer i.ipCacheMu.RUnlock()
	
	return map[string]interface{}{
		"cache_size":    len(i.ipVMCache),
		"cache_ttl":     i.cacheExpire.Seconds(),
		"inject_enabled": i.config.Enabled,
	}
}

// GetConfig 获取配置
func (i *LabelInjector) GetConfig() LabelConfig {
	return i.config
}

// MetricData 指标数据（与alert模块兼容）
type MetricData struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels"`
	Timestamp time.Time         `json:"timestamp"`
}

// AlertEvent 告警事件（与alert模块兼容）
type AlertEvent struct {
	ID          string            `json:"id"`
	RuleID      string            `json:"rule_id"`
	RuleName    string            `json:"rule_name"`
	Level       string            `json:"level"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}
