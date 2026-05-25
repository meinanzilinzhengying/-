//go:build linux

// Package alert 提供告警管理功能
// 本文件定义内置告警模板库
package alert

import (
	"fmt"
	"sync"
	"time"
)

// BuiltInTemplate 内置告警模板类型
type BuiltInTemplate string

const (
	// ========== 网络类告警 ==========
	// TemplateHighLatency 高时延告警
	TemplateHighLatency BuiltInTemplate = "high_latency"
	// TemplatePacketLoss 丢包告警
	TemplatePacketLoss BuiltInTemplate = "packet_loss"
	// TemplateHighRetransmit 高重传率告警
	TemplateHighRetransmit BuiltInTemplate = "high_retransmit"
	// TemplateConnectionFailed 连接失败告警
	TemplateConnectionFailed BuiltInTemplate = "connection_failed"
	// TemplateConnectionTimeout 连接超时告警
	TemplateConnectionTimeout BuiltInTemplate = "connection_timeout"
	// TemplateDNSError DNS解析错误告警
	TemplateDNSError BuiltInTemplate = "dns_error"
	// TemplateTCPRetransmit TCP重传告警
	TemplateTCPRetransmit BuiltInTemplate = "tcp_retransmit"
	// TemplateNetworkIOHigh 网络IO高负载告警
	TemplateNetworkIOHigh BuiltInTemplate = "network_io_high"

	// ========== 安全类告警 ==========
	// TemplateSSLExpiring SSL证书即将过期告警
	TemplateSSLExpiring BuiltInTemplate = "ssl_expiring"
	// TemplateSSLExpired SSL证书已过期告警
	TemplateSSLExpired BuiltInTemplate = "ssl_expired"
	// TemplateAuthFailed 认证失败告警
	TemplateAuthFailed BuiltInTemplate = "auth_failed"
	// TemplateSuspiciousTraffic 可疑流量告警
	TemplateSuspiciousTraffic BuiltInTemplate = "suspicious_traffic"

	// ========== 资源类告警 ==========
	// TemplateCPUHigh CPU使用率高告警
	TemplateCPUHigh BuiltInTemplate = "cpu_high"
	// TemplateMemoryHigh 内存使用率高告警
	TemplateMemoryHigh BuiltInTemplate = "memory_high"
	// TemplateDiskHigh 磁盘使用率高告警
	TemplateDiskHigh BuiltInTemplate = "disk_high"
	// TemplateDiskIOHigh 磁盘IO高负载告警
	TemplateDiskIOHigh BuiltInTemplate = "disk_io_high"
	// TemplateConnectionPool 连接池耗尽告警
	TemplateConnectionPool BuiltInTemplate = "connection_pool"
	// TemplateFDHigh 文件描述符使用率高告警
	TemplateFDHigh BuiltInTemplate = "fd_high"
	// TemplateProcessHigh 进程数过多告警
	TemplateProcessHigh BuiltInTemplate = "process_high"

	// ========== 应用类告警 ==========
	// TemplateErrorRateHigh 错误率高告警
	TemplateErrorRateHigh BuiltInTemplate = "error_rate_high"
	// TemplateSlowQuery 慢查询告警
	TemplateSlowQuery BuiltInTemplate = "slow_query"
	// TemplateServiceDown 服务宕机告警
	TemplateServiceDown BuiltInTemplate = "service_down"
	// TemplateQueueBacklog 队列积压告警
	TemplateQueueBacklog BuiltInTemplate = "queue_backlog"
	// TemplateAPILatencyHigh API延迟高告警
	TemplateAPILatencyHigh BuiltInTemplate = "api_latency_high"
	// TemplateThroughputDrop 吞吐量下降告警
	TemplateThroughputDrop BuiltInTemplate = "throughput_drop"
)

// TemplateDefinition 模板定义
type TemplateDefinition struct {
	Template      BuiltInTemplate   `json:"template"`
	Name          string            `json:"name"`
	Level         AlertLevel        `json:"level"`
	Category      string            `json:"category"`
	Metric        string            `json:"metric"`
	Operator      Operator          `json:"operator"`
	Threshold     float64           `json:"threshold"`
	FireThreshold int               `json:"fire_threshold"`
	ResolveAfter  string            `json:"resolve_after"`
	Description   string            `json:"description"`
	Suggestion    string            `json:"suggestion"`
	Labels        map[string]string `json:"labels"`
	Annotations   map[string]string `json:"annotations"`
}

// TemplateLibrary 模板库
type TemplateLibrary struct {
	templates map[BuiltInTemplate]*TemplateDefinition
	mu        sync.RWMutex
}

// NewTemplateLibrary 创建模板库
func NewTemplateLibrary() *TemplateLibrary {
	lib := &TemplateLibrary{
		templates: make(map[BuiltInTemplate]*TemplateDefinition),
	}
	lib.initBuiltinTemplates()
	return lib
}

// Get 获取模板定义
func (lib *TemplateLibrary) Get(template BuiltInTemplate) (*TemplateDefinition, bool) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()

	t, ok := lib.templates[template]
	return t, ok
}

// GetAll 获取所有模板
func (lib *TemplateLibrary) GetAll() map[BuiltInTemplate]*TemplateDefinition {
	lib.mu.RLock()
	defer lib.mu.RUnlock()

	result := make(map[BuiltInTemplate]*TemplateDefinition)
	for k, v := range lib.templates {
		result[k] = v
	}
	return result
}

// GetByCategory 按分类获取模板
func (lib *TemplateLibrary) GetByCategory(category string) []*TemplateDefinition {
	lib.mu.RLock()
	defer lib.mu.RUnlock()

	var result []*TemplateDefinition
	for _, t := range lib.templates {
		if t.Category == category {
			result = append(result, t)
		}
	}
	return result
}

// ListCategories 获取所有分类
func (lib *TemplateLibrary) ListCategories() []string {
	lib.mu.RLock()
	defer lib.mu.RUnlock()

	categories := make(map[string]bool)
	for _, t := range lib.templates {
		categories[t.Category] = true
	}

	result := make([]string, 0, len(categories))
	for cat := range categories {
		result = append(result, cat)
	}
	return result
}

// initBuiltinTemplates 初始化内置模板
func (lib *TemplateLibrary) initBuiltinTemplates() {
	lib.templates = map[BuiltInTemplate]*TemplateDefinition{
		// ========== 网络类告警 ==========
		TemplateHighLatency: {
			Template:      TemplateHighLatency,
			Name:          "网络高时延",
			Level:         AlertLevelWarning,
			Category:      "network",
			Metric:        "network_latency_ms",
			Operator:      OpGreaterThan,
			Threshold:     100,
			FireThreshold: 3,
			ResolveAfter:  "5m",
			Description:   "网络延迟超过阈值，可能影响应用响应时间",
			Suggestion:    "检查网络链路质量，排查网络拥塞或路由问题",
			Labels:        map[string]string{"category": "network"},
			Annotations: map[string]string{
				"summary":     "网络高时延告警",
				"description": "实例 {{ $labels.instance }} 的网络延迟为 {{ $value }}ms，超过阈值 100ms",
			},
		},
		TemplatePacketLoss: {
			Template:      TemplatePacketLoss,
			Name:          "网络丢包",
			Level:         AlertLevelCritical,
			Category:      "network",
			Metric:        "packet_loss_rate",
			Operator:      OpGreaterThan,
			Threshold:     0.01,
			FireThreshold: 2,
			ResolveAfter:  "5m",
			Description:   "网络丢包率超过阈值，可能导致数据传输失败",
			Suggestion:    "检查网络设备状态，排查物理链路或网络配置问题",
			Labels:        map[string]string{"category": "network"},
			Annotations: map[string]string{
				"summary":     "网络丢包告警",
				"description": "实例 {{ $labels.instance }} 的丢包率为 {{ $value | humanizePercentage }}",
			},
		},
		TemplateHighRetransmit: {
			Template:      TemplateHighRetransmit,
			Name:          "TCP高重传率",
			Level:         AlertLevelWarning,
			Category:      "network",
			Metric:        "tcp_retransmit_rate",
			Operator:      OpGreaterThan,
			Threshold:     0.05,
			FireThreshold: 3,
			ResolveAfter:  "10m",
			Description:   "TCP重传率过高，可能存在网络不稳定或拥塞",
			Suggestion:    "检查网络质量，优化TCP参数或增加带宽",
			Labels:        map[string]string{"category": "network"},
			Annotations: map[string]string{
				"summary":     "TCP高重传率告警",
				"description": "实例 {{ $labels.instance }} 的TCP重传率为 {{ $value | humanizePercentage }}",
			},
		},
		TemplateConnectionFailed: {
			Template:      TemplateConnectionFailed,
			Name:          "连接建立失败",
			Level:         AlertLevelCritical,
			Category:      "network",
			Metric:        "connection_failure_rate",
			Operator:      OpGreaterThan,
			Threshold:     0.1,
			FireThreshold: 2,
			ResolveAfter:  "5m",
			Description:   "连接建立失败率过高，服务可能不可用",
			Suggestion:    "检查目标服务状态，排查防火墙或网络策略配置",
			Labels:        map[string]string{"category": "network"},
			Annotations: map[string]string{
				"summary":     "连接建立失败告警",
				"description": "实例 {{ $labels.instance }} 的连接失败率为 {{ $value | humanizePercentage }}",
			},
		},
		TemplateConnectionTimeout: {
			Template:      TemplateConnectionTimeout,
			Name:          "连接超时",
			Level:         AlertLevelWarning,
			Category:      "network",
			Metric:        "connection_timeout_rate",
			Operator:      OpGreaterThan,
			Threshold:     0.05,
			FireThreshold: 3,
			ResolveAfter:  "5m",
			Description:   "连接超时率过高，网络或目标服务响应缓慢",
			Suggestion:    "检查网络延迟和目标服务负载，考虑增加超时时间或优化服务性能",
			Labels:        map[string]string{"category": "network"},
			Annotations: map[string]string{
				"summary":     "连接超时告警",
				"description": "实例 {{ $labels.instance }} 的连接超时率为 {{ $value | humanizePercentage }}",
			},
		},
		TemplateDNSError: {
			Template:      TemplateDNSError,
			Name:          "DNS解析错误",
			Level:         AlertLevelWarning,
			Category:      "network",
			Metric:        "dns_error_rate",
			Operator:      OpGreaterThan,
			Threshold:     0.01,
			FireThreshold: 3,
			ResolveAfter:  "5m",
			Description:   "DNS解析错误率过高，域名解析服务可能异常",
			Suggestion:    "检查DNS服务器配置和网络连通性，考虑更换DNS服务器",
			Labels:        map[string]string{"category": "network"},
			Annotations: map[string]string{
				"summary":     "DNS解析错误告警",
				"description": "实例 {{ $labels.instance }} 的DNS错误率为 {{ $value | humanizePercentage }}",
			},
		},
		TemplateTCPRetransmit: {
			Template:      TemplateTCPRetransmit,
			Name:          "TCP重传异常",
			Level:         AlertLevelWarning,
			Category:      "network",
			Metric:        "tcp_retransmit_segments",
			Operator:      OpGreaterThan,
			Threshold:     100,
			FireThreshold: 3,
			ResolveAfter:  "10m",
			Description:   "TCP重传段数过多，网络质量可能下降",
			Suggestion:    "检查网络链路质量，排查丢包或乱序问题",
			Labels:        map[string]string{"category": "network"},
			Annotations: map[string]string{
				"summary":     "TCP重传异常告警",
				"description": "实例 {{ $labels.instance }} 的TCP重传段数为 {{ $value }}/秒",
			},
		},
		TemplateNetworkIOHigh: {
			Template:      TemplateNetworkIOHigh,
			Name:          "网络IO高负载",
			Level:         AlertLevelWarning,
			Category:      "network",
			Metric:        "network_io_utilization",
			Operator:      OpGreaterThan,
			Threshold:     0.8,
			FireThreshold: 3,
			ResolveAfter:  "10m",
			Description:   "网络带宽使用率过高，可能出现拥塞",
			Suggestion:    "检查网络流量来源，考虑扩容带宽或优化流量",
			Labels:        map[string]string{"category": "network"},
			Annotations: map[string]string{
				"summary":     "网络IO高负载告警",
				"description": "实例 {{ $labels.instance }} 的网络带宽使用率为 {{ $value | humanizePercentage }}",
			},
		},

		// ========== 安全类告警 ==========
		TemplateSSLExpiring: {
			Template:      TemplateSSLExpiring,
			Name:          "SSL证书即将过期",
			Level:         AlertLevelWarning,
			Category:      "security",
			Metric:        "ssl_cert_expire_days",
			Operator:      OpLessThan,
			Threshold:     30,
			FireThreshold: 1,
			ResolveAfter:  "24h",
			Description:   "SSL证书将在30天内过期，需要及时更新",
			Suggestion:    "尽快更新SSL证书，避免服务中断",
			Labels:        map[string]string{"category": "security"},
			Annotations: map[string]string{
				"summary":     "SSL证书即将过期",
				"description": "域名 {{ $labels.domain }} 的SSL证书将在 {{ $value }} 天后过期",
			},
		},
		TemplateSSLExpired: {
			Template:      TemplateSSLExpired,
			Name:          "SSL证书已过期",
			Level:         AlertLevelCritical,
			Category:      "security",
			Metric:        "ssl_cert_expire_days",
			Operator:      OpLessThan,
			Threshold:     0,
			FireThreshold: 1,
			ResolveAfter:  "1h",
			Description:   "SSL证书已过期，服务可能无法正常访问",
			Suggestion:    "立即更新SSL证书，恢复服务正常访问",
			Labels:        map[string]string{"category": "security"},
			Annotations: map[string]string{
				"summary":     "SSL证书已过期",
				"description": "域名 {{ $labels.domain }} 的SSL证书已过期",
			},
		},
		TemplateAuthFailed: {
			Template:      TemplateAuthFailed,
			Name:          "认证失败率高",
			Level:         AlertLevelWarning,
			Category:      "security",
			Metric:        "auth_failure_rate",
			Operator:      OpGreaterThan,
			Threshold:     0.1,
			FireThreshold: 3,
			ResolveAfter:  "10m",
			Description:   "认证失败率过高，可能存在暴力破解或配置错误",
			Suggestion:    "检查认证日志，排查异常访问或修复配置问题",
			Labels:        map[string]string{"category": "security"},
			Annotations: map[string]string{
				"summary":     "认证失败告警",
				"description": "服务 {{ $labels.service }} 的认证失败率为 {{ $value | humanizePercentage }}",
			},
		},
		TemplateSuspiciousTraffic: {
			Template:      TemplateSuspiciousTraffic,
			Name:          "可疑流量检测",
			Level:         AlertLevelWarning,
			Category:      "security",
			Metric:        "suspicious_traffic_rate",
			Operator:      OpGreaterThan,
			Threshold:     0.01,
			FireThreshold: 2,
			ResolveAfter:  "10m",
			Description:   "检测到可疑流量，可能存在安全威胁",
			Suggestion:    "检查访问日志，分析流量来源，必要时封禁可疑IP",
			Labels:        map[string]string{"category": "security"},
			Annotations: map[string]string{
				"summary":     "可疑流量告警",
				"description": "实例 {{ $labels.instance }} 检测到可疑流量",
			},
		},

		// ========== 资源类告警 ==========
		TemplateCPUHigh: {
			Template:      TemplateCPUHigh,
			Name:          "CPU使用率高",
			Level:         AlertLevelWarning,
			Category:      "resource",
			Metric:        "cpu_usage_percent",
			Operator:      OpGreaterThan,
			Threshold:     80,
			FireThreshold: 3,
			ResolveAfter:  "10m",
			Description:   "CPU使用率超过80%，系统负载较高",
			Suggestion:    "检查高CPU进程，优化应用性能或扩容资源",
			Labels:        map[string]string{"category": "resource"},
			Annotations: map[string]string{
				"summary":     "CPU高使用率告警",
				"description": "实例 {{ $labels.instance }} 的CPU使用率为 {{ $value }}%",
			},
		},
		TemplateMemoryHigh: {
			Template:      TemplateMemoryHigh,
			Name:          "内存使用率高",
			Level:         AlertLevelWarning,
			Category:      "resource",
			Metric:        "memory_usage_percent",
			Operator:      OpGreaterThan,
			Threshold:     85,
			FireThreshold: 3,
			ResolveAfter:  "10m",
			Description:   "内存使用率超过85%，可能存在内存泄漏或不足",
			Suggestion:    "检查内存使用情况，优化应用或增加内存",
			Labels:        map[string]string{"category": "resource"},
			Annotations: map[string]string{
				"summary":     "内存高使用率告警",
				"description": "实例 {{ $labels.instance }} 的内存使用率为 {{ $value }}%",
			},
		},
		TemplateDiskHigh: {
			Template:      TemplateDiskHigh,
			Name:          "磁盘使用率高",
			Level:         AlertLevelWarning,
			Category:      "resource",
			Metric:        "disk_usage_percent",
			Operator:      OpGreaterThan,
			Threshold:     85,
			FireThreshold: 2,
			ResolveAfter:  "1h",
			Description:   "磁盘使用率超过85%，需要清理或扩容",
			Suggestion:    "清理不必要的文件，或扩容磁盘空间",
			Labels:        map[string]string{"category": "resource"},
			Annotations: map[string]string{
				"summary":     "磁盘高使用率告警",
				"description": "实例 {{ $labels.instance }} 的磁盘使用率为 {{ $value }}%",
			},
		},
		TemplateDiskIOHigh: {
			Template:      TemplateDiskIOHigh,
			Name:          "磁盘IO高负载",
			Level:         AlertLevelWarning,
			Category:      "resource",
			Metric:        "disk_io_utilization",
			Operator:      OpGreaterThan,
			Threshold:     0.8,
			FireThreshold: 3,
			ResolveAfter:  "10m",
			Description:   "磁盘IO使用率过高，可能影响应用性能",
			Suggestion:    "检查IO密集型进程，优化磁盘访问或升级存储",
			Labels:        map[string]string{"category": "resource"},
			Annotations: map[string]string{
				"summary":     "磁盘IO高负载告警",
				"description": "实例 {{ $labels.instance }} 的磁盘IO使用率为 {{ $value | humanizePercentage }}",
			},
		},
		TemplateConnectionPool: {
			Template:      TemplateConnectionPool,
			Name:          "连接池耗尽",
			Level:         AlertLevelCritical,
			Category:      "resource",
			Metric:        "connection_pool_usage",
			Operator:      OpGreaterThan,
			Threshold:     0.95,
			FireThreshold: 2,
			ResolveAfter:  "5m",
			Description:   "连接池使用率超过95%，新连接可能无法建立",
			Suggestion:    "增加连接池大小或检查连接泄漏",
			Labels:        map[string]string{"category": "resource"},
			Annotations: map[string]string{
				"summary":     "连接池耗尽告警",
				"description": "服务 {{ $labels.service }} 的连接池使用率为 {{ $value | humanizePercentage }}",
			},
		},
		TemplateFDHigh: {
			Template:      TemplateFDHigh,
			Name:          "文件描述符使用率高",
			Level:         AlertLevelWarning,
			Category:      "resource",
			Metric:        "fd_usage_percent",
			Operator:      OpGreaterThan,
			Threshold:     80,
			FireThreshold: 3,
			ResolveAfter:  "10m",
			Description:   "文件描述符使用率过高，可能导致资源耗尽",
			Suggestion:    "检查文件句柄泄漏，增加系统限制",
			Labels:        map[string]string{"category": "resource"},
			Annotations: map[string]string{
				"summary":     "文件描述符高使用率告警",
				"description": "实例 {{ $labels.instance }} 的文件描述符使用率为 {{ $value }}%",
			},
		},
		TemplateProcessHigh: {
			Template:      TemplateProcessHigh,
			Name:          "进程数过多",
			Level:         AlertLevelWarning,
			Category:      "resource",
			Metric:        "process_count",
			Operator:      OpGreaterThan,
			Threshold:     1000,
			FireThreshold: 3,
			ResolveAfter:  "10m",
			Description:   "进程数过多，可能影响系统性能",
			Suggestion:    "检查僵尸进程或不必要的进程，优化进程管理",
			Labels:        map[string]string{"category": "resource"},
			Annotations: map[string]string{
				"summary":     "进程数过多告警",
				"description": "实例 {{ $labels.instance }} 的进程数为 {{ $value }}",
			},
		},

		// ========== 应用类告警 ==========
		TemplateErrorRateHigh: {
			Template:      TemplateErrorRateHigh,
			Name:          "错误率高",
			Level:         AlertLevelCritical,
			Category:      "application",
			Metric:        "error_rate",
			Operator:      OpGreaterThan,
			Threshold:     0.05,
			FireThreshold: 2,
			ResolveAfter:  "5m",
			Description:   "应用错误率超过5%，服务质量可能受损",
			Suggestion:    "检查应用日志，定位错误原因并修复",
			Labels:        map[string]string{"category": "application"},
			Annotations: map[string]string{
				"summary":     "错误率高告警",
				"description": "服务 {{ $labels.service }} 的错误率为 {{ $value | humanizePercentage }}",
			},
		},
		TemplateSlowQuery: {
			Template:      TemplateSlowQuery,
			Name:          "慢查询",
			Level:         AlertLevelWarning,
			Category:      "application",
			Metric:        "slow_query_count",
			Operator:      OpGreaterThan,
			Threshold:     10,
			FireThreshold: 2,
			ResolveAfter:  "10m",
			Description:   "慢查询数量过多，可能影响数据库性能",
			Suggestion:    "优化慢查询SQL，添加索引或优化查询逻辑",
			Labels:        map[string]string{"category": "application"},
			Annotations: map[string]string{
				"summary":     "慢查询告警",
				"description": "数据库 {{ $labels.database }} 的慢查询数为 {{ $value }}/分钟",
			},
		},
		TemplateServiceDown: {
			Template:      TemplateServiceDown,
			Name:          "服务宕机",
			Level:         AlertLevelCritical,
			Category:      "application",
			Metric:        "service_up",
			Operator:      OpEqual,
			Threshold:     0,
			FireThreshold: 1,
			ResolveAfter:  "1m",
			Description:   "服务不可用，需要立即处理",
			Suggestion:    "检查服务进程状态，查看日志排查故障原因",
			Labels:        map[string]string{"category": "application"},
			Annotations: map[string]string{
				"summary":     "服务宕机告警",
				"description": "服务 {{ $labels.service }} 在实例 {{ $labels.instance }} 上不可用",
			},
		},
		TemplateQueueBacklog: {
			Template:      TemplateQueueBacklog,
			Name:          "队列积压",
			Level:         AlertLevelWarning,
			Category:      "application",
			Metric:        "queue_depth",
			Operator:      OpGreaterThan,
			Threshold:     1000,
			FireThreshold: 3,
			ResolveAfter:  "10m",
			Description:   "队列积压严重，消费速度跟不上生产速度",
			Suggestion:    "增加消费者数量或优化消费逻辑",
			Labels:        map[string]string{"category": "application"},
			Annotations: map[string]string{
				"summary":     "队列积压告警",
				"description": "队列 {{ $labels.queue }} 的积压消息数为 {{ $value }}",
			},
		},
		TemplateAPILatencyHigh: {
			Template:      TemplateAPILatencyHigh,
			Name:          "API延迟高",
			Level:         AlertLevelWarning,
			Category:      "application",
			Metric:        "api_latency_p99",
			Operator:      OpGreaterThan,
			Threshold:     500,
			FireThreshold: 3,
			ResolveAfter:  "10m",
			Description:   "API P99延迟过高，用户体验可能受影响",
			Suggestion:    "优化API性能，检查下游依赖响应时间",
			Labels:        map[string]string{"category": "application"},
			Annotations: map[string]string{
				"summary":     "API延迟高告警",
				"description": "API {{ $labels.api }} 的P99延迟为 {{ $value }}ms",
			},
		},
		TemplateThroughputDrop: {
			Template:      TemplateThroughputDrop,
			Name:          "吞吐量下降",
			Level:         AlertLevelWarning,
			Category:      "application",
			Metric:        "throughput_drop_rate",
			Operator:      OpGreaterThan,
			Threshold:     0.3,
			FireThreshold: 2,
			ResolveAfter:  "10m",
			Description:   "吞吐量较基线下降超过30%，可能存在性能问题",
			Suggestion:    "检查系统负载和依赖服务状态，排查性能瓶颈",
			Labels:        map[string]string{"category": "application"},
			Annotations: map[string]string{
				"summary":     "吞吐量下降告警",
				"description": "服务 {{ $labels.service }} 的吞吐量下降了 {{ $value | humanizePercentage }}",
			},
		},
	}
}

// GetTemplateStats 获取模板统计
func (lib *TemplateLibrary) GetTemplateStats() map[string]interface{} {
	lib.mu.RLock()
	defer lib.mu.RUnlock()

	stats := map[string]interface{}{
		"total_templates": len(lib.templates),
		"categories":      lib.ListCategories(),
	}

	byCategory := make(map[string]int)
	byLevel := make(map[string]int)

	for _, t := range lib.templates {
		byCategory[t.Category]++
		byLevel[t.Level.String()]++
	}

	stats["by_category"] = byCategory
	stats["by_level"] = byLevel

	return stats
}

// ValidateTemplate 验证模板参数
func (lib *TemplateLibrary) ValidateTemplate(template BuiltInTemplate, overrides map[string]interface{}) error {
	def, exists := lib.Get(template)
	if !exists {
		return fmt.Errorf("模板不存在: %s", template)
	}

	// 验证阈值覆盖
	if threshold, ok := overrides["threshold"]; ok {
		switch v := threshold.(type) {
		case float64:
			if v < 0 {
				return fmt.Errorf("阈值不能为负数")
			}
		case int:
			if v < 0 {
				return fmt.Errorf("阈值不能为负数")
			}
		default:
			return fmt.Errorf("阈值类型无效")
		}
	}

	// 验证级别覆盖
	if level, ok := overrides["level"].(string); ok {
		parsed := ParseLevel(level)
		if parsed == AlertLevelInfo && level != "info" && level != "一般" {
			return fmt.Errorf("无效的告警级别: %s", level)
		}
	}

	// 验证触发阈值
	if fireThreshold, ok := overrides["fire_threshold"].(int); ok {
		if fireThreshold < 1 {
			return fmt.Errorf("触发阈值必须大于等于1")
		}
	}

	return nil
}
