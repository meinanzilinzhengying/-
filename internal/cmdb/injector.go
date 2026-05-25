//go:build linux

// Package cmdb 提供CMDB系统对接功能
// 本文件实现标签注入器，将CMDB标签嵌入指标、链路、日志数据
package cmdb

import (
	"net"
	"strings"
	"sync"

	"cloud-flow-agent/pkg/logger"
)

// InjectorConfig 注入器配置
type InjectorConfig struct {
	Enabled           bool     `yaml:"enabled" json:"enabled"`                       // 是否启用
	InjectToMetrics   bool     `yaml:"inject_to_metrics" json:"inject_to_metrics"`   // 注入指标
	InjectToTraces    bool     `yaml:"inject_to_traces" json:"inject_to_traces"`     // 注入链路
	InjectToLogs      bool     `yaml:"inject_to_logs" json:"inject_to_logs"`         // 注入日志
	PriorityFields    []string `yaml:"priority_fields" json:"priority_fields"`       // 优先匹配字段
	CacheSize         int      `yaml:"cache_size" json:"cache_size"`                 // 缓存大小
}

// DefaultInjectorConfig 默认注入器配置
func DefaultInjectorConfig() InjectorConfig {
	return InjectorConfig{
		Enabled:         true,
		InjectToMetrics: true,
		InjectToTraces:  true,
		InjectToLogs:    true,
		PriorityFields:  []string{"instance", "host", "hostname", "source", "pod_ip", "node_ip"},
		CacheSize:       10000,
	}
}

// LabelInjector 标签注入器
type LabelInjector struct {
	config      InjectorConfig
	syncService *SyncService
	log         *logger.Logger

	// 标签缓存（避免重复查询）
	cache      map[string]*CMDBLabels
	cacheOrder []string // LRU顺序
	cacheMu    sync.RWMutex
}

// NewLabelInjector 创建标签注入器
func NewLabelInjector(config InjectorConfig, syncService *SyncService, log *logger.Logger) *LabelInjector {
	return &LabelInjector{
		config:      config,
		syncService: syncService,
		log:         log,
		cache:       make(map[string]*CMDBLabels),
		cacheOrder:  make([]string, 0, config.CacheSize),
	}
}

// ==================== 标签注入接口 ====================

// InjectLabels 注入CMDB标签到数据
func (i *LabelInjector) InjectLabels(data map[string]string) map[string]string {
	if !i.config.Enabled || data == nil {
		return data
	}

	// 提取标识符（IP或主机名）
	identifier, idType := i.extractIdentifier(data)
	if identifier == "" {
		return data
	}

	// 获取CMDB标签
	labels := i.getLabels(identifier, idType)
	if labels == nil {
		return data
	}

	// 合并标签
	return labels.MergeLabels(data)
}

// InjectMetrics 注入指标标签
func (i *LabelInjector) InjectMetrics(metricName string, labels map[string]string) map[string]string {
	if !i.config.Enabled || !i.config.InjectToMetrics {
		return labels
	}

	return i.InjectLabels(labels)
}

// InjectTrace 注入链路标签
func (i *LabelInjector) InjectTrace(trace map[string]string) map[string]string {
	if !i.config.Enabled || !i.config.InjectToTraces {
		return trace
	}

	return i.InjectLabels(trace)
}

// InjectLog 注入日志标签
func (i *LabelInjector) InjectLog(log map[string]string) map[string]string {
	if !i.config.Enabled || !i.config.InjectToLogs {
		return log
	}

	return i.InjectLabels(log)
}

// ==================== 内部方法 ====================

// extractIdentifier 从数据中提取标识符
func (i *LabelInjector) extractIdentifier(data map[string]string) (string, string) {
	// 按优先级查找字段
	for _, field := range i.config.PriorityFields {
		if value, exists := data[field]; exists && value != "" {
			// 判断是IP还是主机名
			if net.ParseIP(value) != nil {
				return value, "ip"
			}
			// 包含点号的可能包含IP
			if strings.Contains(value, ":") {
				// 可能是 host:port 格式，提取host
				parts := strings.Split(value, ":")
				if len(parts) >= 2 {
					host := strings.Join(parts[:len(parts)-1], ":")
					if net.ParseIP(host) != nil {
						return host, "ip"
					}
				}
			}
			return value, "hostname"
		}
	}

	// 尝试从其他常见字段提取
	for key, value := range data {
		lowerKey := strings.ToLower(key)
		if strings.Contains(lowerKey, "ip") || strings.Contains(lowerKey, "addr") {
			if net.ParseIP(value) != nil {
				return value, "ip"
			}
		}
		if strings.Contains(lowerKey, "host") || strings.Contains(lowerKey, "name") {
			return value, "hostname"
		}
	}

	return "", ""
}

// getLabels 获取CMDB标签（带缓存）
func (i *LabelInjector) getLabels(identifier, idType string) *CMDBLabels {
	// 先查缓存
	i.cacheMu.RLock()
	if labels, exists := i.cache[identifier]; exists {
		i.cacheMu.RUnlock()
		i.updateCacheOrder(identifier)
		return labels
	}
	i.cacheMu.RUnlock()

	// 从同步服务获取
	var labels *CMDBLabels
	if idType == "ip" {
		labels = i.syncService.GetLabelsByIP(identifier)
	} else {
		labels = i.syncService.GetLabelsByHostname(identifier)
	}

	if labels != nil {
		i.addToCache(identifier, labels)
	}

	return labels
}

// addToCache 添加标签到缓存
func (i *LabelInjector) addToCache(key string, labels *CMDBLabels) {
	i.cacheMu.Lock()
	defer i.cacheMu.Unlock()

	// 检查缓存是否已满
	if len(i.cache) >= i.config.CacheSize {
		// 淘汰最久未使用的
		oldest := i.cacheOrder[0]
		delete(i.cache, oldest)
		i.cacheOrder = i.cacheOrder[1:]
	}

	// 添加新条目
	i.cache[key] = labels
	i.cacheOrder = append(i.cacheOrder, key)
}

// updateCacheOrder 更新缓存访问顺序（LRU）
func (i *LabelInjector) updateCacheOrder(key string) {
	i.cacheMu.Lock()
	defer i.cacheMu.Unlock()

	// 找到key的位置并移到末尾
	for idx, k := range i.cacheOrder {
		if k == key {
			// 移除当前位置
			i.cacheOrder = append(i.cacheOrder[:idx], i.cacheOrder[idx+1:]...)
			// 添加到末尾
			i.cacheOrder = append(i.cacheOrder, key)
			break
		}
	}
}

// ClearCache 清空缓存
func (i *LabelInjector) ClearCache() {
	i.cacheMu.Lock()
	defer i.cacheMu.Unlock()

	i.cache = make(map[string]*CMDBLabels)
	i.cacheOrder = make([]string, 0, i.config.CacheSize)
}

// GetCacheStats 获取缓存统计
func (i *LabelInjector) GetCacheStats() map[string]interface{} {
	i.cacheMu.RLock()
	defer i.cacheMu.RUnlock()

	return map[string]interface{}{
		"cache_size":    len(i.cache),
		"cache_capacity": i.config.CacheSize,
		"cache_hit_rate": i.calculateHitRate(),
	}
}

// calculateHitRate 计算缓存命中率（简化实现）
func (i *LabelInjector) calculateHitRate() float64 {
	// 实际实现应该记录命中次数和总查询次数
	// 这里返回一个占位值
	return 0.0
}

// ==================== 批量注入接口 ====================

// BatchInjectMetrics 批量注入指标标签
func (i *LabelInjector) BatchInjectMetrics(metrics []map[string]string) []map[string]string {
	if !i.config.Enabled || !i.config.InjectToMetrics {
		return metrics
	}

	result := make([]map[string]string, len(metrics))
	for idx, metric := range metrics {
		result[idx] = i.InjectLabels(metric)
	}
	return result
}

// BatchInjectTraces 批量注入链路标签
func (i *LabelInjector) BatchInjectTraces(traces []map[string]string) []map[string]string {
	if !i.config.Enabled || !i.config.InjectToTraces {
		return traces
	}

	result := make([]map[string]string, len(traces))
	for idx, trace := range traces {
		result[idx] = i.InjectLabels(trace)
	}
	return result
}

// BatchInjectLogs 批量注入日志标签
func (i *LabelInjector) BatchInjectLogs(logs []map[string]string) []map[string]string {
	if !i.config.Enabled || !i.config.InjectToLogs {
		return logs
	}

	result := make([]map[string]string, len(logs))
	for idx, log := range logs {
		result[idx] = i.InjectLabels(log)
	}
	return result
}
