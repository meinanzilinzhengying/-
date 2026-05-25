//go:build linux

// Package cmdb 提供CMDB系统对接功能
// 本文件定义CMDB业务属性模型
package cmdb

import (
	"time"
)

// ==================== 业务属性模型 ====================

// BusinessSystem 业务系统
type BusinessSystem struct {
	ID          string            `json:"id" yaml:"id"`                   // 系统ID
	Name        string            `json:"name" yaml:"name"`               // 系统名称
	Code        string            `json:"code" yaml:"code"`               // 系统编码
	Level       SystemLevel       `json:"level" yaml:"level"`             // 系统等级
	Owner       string            `json:"owner" yaml:"owner"`             // 负责人
	Owners      []string          `json:"owners" yaml:"owners"`           // 负责人列表（支持多负责人）
	Department  string            `json:"department" yaml:"department"`   // 所属部门
	Description string            `json:"description" yaml:"description"` // 系统描述
	Status      string            `json:"status" yaml:"status"`           // 系统状态
	Labels      map[string]string `json:"labels" yaml:"labels"`           // 扩展标签
	CreateTime  time.Time         `json:"create_time" yaml:"create_time"` // 创建时间
	UpdateTime  time.Time         `json:"update_time" yaml:"update_time"` // 更新时间
}

// SystemLevel 系统等级
type SystemLevel int

const (
	SystemLevelCritical SystemLevel = iota // 核心系统
	SystemLevelImportant                   // 重要系统
	SystemLevelNormal                      // 一般系统
	SystemLevelLow                         // 低优先级
)

// String 返回系统等级字符串
func (l SystemLevel) String() string {
	switch l {
	case SystemLevelCritical:
		return "critical"
	case SystemLevelImportant:
		return "important"
	case SystemLevelNormal:
		return "normal"
	case SystemLevelLow:
		return "low"
	default:
		return "unknown"
	}
}

// ParseSystemLevel 解析系统等级
func ParseSystemLevel(s string) SystemLevel {
	switch s {
	case "critical", "核心", "核心系统":
		return SystemLevelCritical
	case "important", "重要", "重要系统":
		return SystemLevelImportant
	case "normal", "一般", "一般系统":
		return SystemLevelNormal
	case "low", "低", "低优先级":
		return SystemLevelLow
	default:
		return SystemLevelNormal
	}
}

// CIItem 配置项（CI）
type CIItem struct {
	ID              string            `json:"id" yaml:"id"`                           // CI ID
	Name            string            `json:"name" yaml:"name"`                       // CI名称
	Type            string            `json:"type" yaml:"type"`                       // CI类型（主机、应用、服务等）
	SystemID        string            `json:"system_id" yaml:"system_id"`             // 所属业务系统ID
	SystemName      string            `json:"system_name" yaml:"system_name"`         // 所属业务系统名称
	IP              string            `json:"ip" yaml:"ip"`                           // IP地址
	Hostname        string            `json:"hostname" yaml:"hostname"`               // 主机名
	Environment     string            `json:"environment" yaml:"environment"`         // 环境（生产、测试、开发）
	Labels          map[string]string `json:"labels" yaml:"labels"`                   // CMDB标签
	Attributes      map[string]interface{} `json:"attributes" yaml:"attributes"`        // 扩展属性
	CreateTime      time.Time         `json:"create_time" yaml:"create_time"`         // 创建时间
	UpdateTime      time.Time         `json:"update_time" yaml:"update_time"`         // 更新时间
}

// CICType CI类型常量
const (
	CITypeHost     = "host"     // 主机
	CITypeApp      = "app"      // 应用
	CITypeService  = "service"  // 服务
	CITypeDatabase = "database" // 数据库
	CITypeCache    = "cache"    // 缓存
	CITypeLB       = "lb"       // 负载均衡
	CITypeNetwork  = "network"  // 网络设备
)

// Environment 环境类型
const (
	EnvProduction = "production" // 生产环境
	EnvTesting    = "testing"    // 测试环境
	EnvDevelopment = "development" // 开发环境
	EnvStaging    = "staging"    // 预发布环境
)

// CMDBLabels CMDB标签集合
type CMDBLabels struct {
	SystemID       string            `json:"system_id"`       // 业务系统ID
	SystemName     string            `json:"system_name"`     // 业务系统名称
	SystemCode     string            `json:"system_code"`     // 业务系统编码
	SystemLevel    string            `json:"system_level"`    // 系统等级
	Owner          string            `json:"owner"`           // 负责人
	Department     string            `json:"department"`      // 部门
	Environment    string            `json:"environment"`     // 环境
	CIType         string            `json:"ci_type"`         // CI类型
	CIID           string            `json:"ci_id"`           // CI ID
	Labels         map[string]string `json:"labels"`          // 其他标签
}

// ToMap 转换为map
func (l *CMDBLabels) ToMap() map[string]string {
	result := make(map[string]string)
	if l.SystemID != "" {
		result["cmdb.system_id"] = l.SystemID
	}
	if l.SystemName != "" {
		result["cmdb.system_name"] = l.SystemName
	}
	if l.SystemCode != "" {
		result["cmdb.system_code"] = l.SystemCode
	}
	if l.SystemLevel != "" {
		result["cmdb.system_level"] = l.SystemLevel
	}
	if l.Owner != "" {
		result["cmdb.owner"] = l.Owner
	}
	if l.Department != "" {
		result["cmdb.department"] = l.Department
	}
	if l.Environment != "" {
		result["cmdb.environment"] = l.Environment
	}
	if l.CIType != "" {
		result["cmdb.ci_type"] = l.CIType
	}
	if l.CIID != "" {
		result["cmdb.ci_id"] = l.CIID
	}
	for k, v := range l.Labels {
		result["cmdb."+k] = v
	}
	return result
}

// MergeLabels 合并标签到现有map
func (l *CMDBLabels) MergeLabels(target map[string]string) map[string]string {
	if target == nil {
		target = make(map[string]string)
	}
	for k, v := range l.ToMap() {
		target[k] = v
	}
	return target
}

// ==================== 查询请求/响应 ====================

// QueryRequest CMDB查询请求
type QueryRequest struct {
	SystemID    string            `json:"system_id,omitempty"`    // 按业务系统查询
	SystemCode  string            `json:"system_code,omitempty"`  // 按系统编码查询
	CIType      string            `json:"ci_type,omitempty"`      // 按CI类型查询
	IP          string            `json:"ip,omitempty"`           // 按IP查询
	Hostname    string            `json:"hostname,omitempty"`     // 按主机名查询
	Environment string            `json:"environment,omitempty"`  // 按环境查询
	Owner       string            `json:"owner,omitempty"`        // 按负责人查询
	Level       SystemLevel       `json:"level,omitempty"`        // 按系统等级查询
	Labels      map[string]string `json:"labels,omitempty"`       // 按标签查询
}

// QueryResult CMDB查询结果
type QueryResult struct {
	Systems []BusinessSystem `json:"systems"` // 业务系统列表
	Items   []CIItem         `json:"items"`   // CI列表
	Total   int              `json:"total"`   // 总数
}

// ==================== 同步相关 ====================

// SyncStatus 同步状态
type SyncStatus struct {
	LastSyncTime    time.Time `json:"last_sync_time"`    // 上次同步时间
	NextSyncTime    time.Time `json:"next_sync_time"`    // 下次同步时间
	SystemCount     int       `json:"system_count"`      // 系统数量
	CIItemCount     int       `json:"ci_item_count"`     // CI数量
	LastSyncSuccess bool      `json:"last_sync_success"` // 上次同步是否成功
	LastError       string    `json:"last_error"`        // 上次错误信息
}

// ==================== 缓存条目 ====================

// CacheEntry 缓存条目
type CacheEntry struct {
	Item       *CIItem
	System     *BusinessSystem
	ExpireTime time.Time
}

// IsExpired 检查是否过期
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpireTime)
}
