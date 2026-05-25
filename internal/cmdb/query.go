//go:build linux

// Package cmdb 提供CMDB系统对接功能
// 本文件实现业务维度查询接口
package cmdb

import (
	"fmt"
	"strings"
	"sync"

	"cloud-flow-agent/pkg/logger"
)

// QueryEngine 查询引擎
type QueryEngine struct {
	syncService *SyncService
	log         *logger.Logger

	// 索引
	systemCodeIndex map[string]string // system_code -> system_id
	ownerIndex      map[string][]string // owner -> []system_id
	departmentIndex map[string][]string // department -> []system_id
	envIndex        map[string][]string // environment -> []ci_id

	// 保护索引的锁
	mu sync.RWMutex
}

// NewQueryEngine 创建查询引擎
func NewQueryEngine(syncService *SyncService, log *logger.Logger) *QueryEngine {
	return &QueryEngine{
		syncService:     syncService,
		log:             log,
		systemCodeIndex: make(map[string]string),
		ownerIndex:      make(map[string][]string),
		departmentIndex: make(map[string][]string),
		envIndex:        make(map[string][]string),
	}
}

// BuildIndex 构建查询索引
func (q *QueryEngine) BuildIndex() {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 清空旧索引
	q.systemCodeIndex = make(map[string]string)
	q.ownerIndex = make(map[string][]string)
	q.departmentIndex = make(map[string][]string)
	q.envIndex = make(map[string][]string)

	// 从同步服务获取所有数据
	systems := q.syncService.GetAllSystems()

	// 构建业务系统索引
	for _, system := range systems {
		// 系统编码索引
		if system.Code != "" {
			q.systemCodeIndex[system.Code] = system.ID
		}

		// 负责人索引
		if system.Owner != "" {
			q.ownerIndex[system.Owner] = append(q.ownerIndex[system.Owner], system.ID)
		}
		for _, owner := range system.Owners {
			if owner != "" {
				q.ownerIndex[owner] = append(q.ownerIndex[owner], system.ID)
			}
		}

		// 部门索引
		if system.Department != "" {
			q.departmentIndex[system.Department] = append(q.departmentIndex[system.Department], system.ID)
		}
	}

	// 构建CI索引
	// 注意：这里需要通过syncService的内部方法获取所有CI
	// 简化实现：遍历所有系统获取CI
	for _, system := range systems {
		cis := q.syncService.GetCIItemsBySystem(system.ID)
		for _, ci := range cis {
			if ci.Environment != "" {
				q.envIndex[ci.Environment] = append(q.envIndex[ci.Environment], ci.ID)
			}
		}
	}

	q.log.Infof("CMDB查询索引构建完成: 系统编码=%d, 负责人=%d, 部门=%d, 环境=%d",
		len(q.systemCodeIndex), len(q.ownerIndex), len(q.departmentIndex), len(q.envIndex))
}

// ==================== 业务系统查询 ====================

// QuerySystems 查询业务系统
func (q *QueryEngine) QuerySystems(req QueryRequest) []*BusinessSystem {
	var result []*BusinessSystem

	// 按ID查询
	if req.SystemID != "" {
		if system, exists := q.syncService.GetSystemByID(req.SystemID); exists {
			result = append(result, system)
		}
		return result
	}

	// 按编码查询
	if req.SystemCode != "" {
		q.mu.RLock()
		systemID, exists := q.systemCodeIndex[req.SystemCode]
		q.mu.RUnlock()

		if exists {
			if system, exists := q.syncService.GetSystemByID(systemID); exists {
				result = append(result, system)
			}
		}
		return result
	}

	// 按负责人查询
	if req.Owner != "" {
		q.mu.RLock()
		systemIDs := q.ownerIndex[req.Owner]
		q.mu.RUnlock()

		for _, id := range systemIDs {
			if system, exists := q.syncService.GetSystemByID(id); exists {
				result = append(result, system)
			}
		}
		return result
	}

	// 按等级查询
	if req.Level != 0 {
		return q.syncService.GetSystemsByLevel(req.Level)
	}

	// 返回所有系统
	return q.syncService.GetAllSystems()
}

// GetSystemByCode 根据编码获取业务系统
func (q *QueryEngine) GetSystemByCode(code string) (*BusinessSystem, bool) {
	q.mu.RLock()
	systemID, exists := q.systemCodeIndex[code]
	q.mu.RUnlock()

	if !exists {
		return nil, false
	}

	return q.syncService.GetSystemByID(systemID)
}

// GetSystemsByOwner 根据负责人获取业务系统
func (q *QueryEngine) GetSystemsByOwner(owner string) []*BusinessSystem {
	q.mu.RLock()
	systemIDs := q.ownerIndex[owner]
	q.mu.RUnlock()

	result := make([]*BusinessSystem, 0, len(systemIDs))
	for _, id := range systemIDs {
		if system, exists := q.syncService.GetSystemByID(id); exists {
			result = append(result, system)
		}
	}
	return result
}

// GetSystemsByDepartment 根据部门获取业务系统
func (q *QueryEngine) GetSystemsByDepartment(dept string) []*BusinessSystem {
	q.mu.RLock()
	systemIDs := q.departmentIndex[dept]
	q.mu.RUnlock()

	result := make([]*BusinessSystem, 0, len(systemIDs))
	for _, id := range systemIDs {
		if system, exists := q.syncService.GetSystemByID(id); exists {
			result = append(result, system)
		}
	}
	return result
}

// ==================== CI查询 ====================

// QueryCIItems 查询CI项
func (q *QueryEngine) QueryCIItems(req QueryRequest) []*CIItem {
	var result []*CIItem

	// 按业务系统查询
	if req.SystemID != "" {
		return q.syncService.GetCIItemsBySystem(req.SystemID)
	}

	// 按环境查询
	if req.Environment != "" {
		q.mu.RLock()
		ciIDs := q.envIndex[req.Environment]
		q.mu.RUnlock()

		for _, id := range ciIDs {
			if ci, exists := q.syncService.GetCIByID(id); exists {
				result = append(result, ci)
			}
		}
		return result
	}

	// 按CI类型查询
	if req.CIType != "" {
		// 需要遍历所有CI
		return q.filterCIByType(req.CIType)
	}

	// 按IP查询
	if req.IP != "" {
		if ci, exists := q.syncService.GetCIByIP(req.IP); exists {
			result = append(result, ci)
		}
		return result
	}

	// 按主机名查询
	if req.Hostname != "" {
		if ci, exists := q.syncService.GetCIByHostname(req.Hostname); exists {
			result = append(result, ci)
		}
		return result
	}

	return result
}

// filterCIByType 按类型过滤CI
func (q *QueryEngine) filterCIByType(ciType string) []*CIItem {
	var result []*CIItem

	// 获取所有系统并过滤
	systems := q.syncService.GetAllSystems()
	for _, system := range systems {
		cis := q.syncService.GetCIItemsBySystem(system.ID)
		for _, ci := range cis {
			if ci.Type == ciType {
				result = append(result, ci)
			}
		}
	}

	return result
}

// ==================== 业务维度分析 ====================

// BusinessDimension 业务维度统计
type BusinessDimension struct {
	SystemID      string `json:"system_id"`
	SystemName    string `json:"system_name"`
	SystemCode    string `json:"system_code"`
	SystemLevel   string `json:"system_level"`
	Owner         string `json:"owner"`
	Department    string `json:"department"`
	CIType        string `json:"ci_type"`
	Count         int    `json:"count"`
}

// AnalyzeBySystem 按业务系统分析
func (q *QueryEngine) AnalyzeBySystem() []BusinessDimension {
	var result []BusinessDimension

	systems := q.syncService.GetAllSystems()
	for _, system := range systems {
		cis := q.syncService.GetCIItemsBySystem(system.ID)

		// 按CI类型分组统计
		typeCount := make(map[string]int)
		for _, ci := range cis {
			typeCount[ci.Type]++
		}

		for ciType, count := range typeCount {
			result = append(result, BusinessDimension{
				SystemID:    system.ID,
				SystemName:  system.Name,
				SystemCode:  system.Code,
				SystemLevel: system.Level.String(),
				Owner:       system.Owner,
				Department:  system.Department,
				CIType:      ciType,
				Count:       count,
			})
		}
	}

	return result
}

// AnalyzeByLevel 按系统等级分析
func (q *QueryEngine) AnalyzeByLevel() map[string]int {
	result := make(map[string]int)

	levels := []SystemLevel{SystemLevelCritical, SystemLevelImportant, SystemLevelNormal, SystemLevelLow}
	for _, level := range levels {
		systems := q.syncService.GetSystemsByLevel(level)
		result[level.String()] = len(systems)
	}

	return result
}

// AnalyzeByOwner 按负责人分析
func (q *QueryEngine) AnalyzeByOwner() map[string]int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make(map[string]int)
	for owner, systems := range q.ownerIndex {
		result[owner] = len(systems)
	}

	return result
}

// AnalyzeByDepartment 按部门分析
func (q *QueryEngine) AnalyzeByDepartment() map[string]int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make(map[string]int)
	for dept, systems := range q.departmentIndex {
		result[dept] = len(systems)
	}

	return result
}

// ==================== 过滤接口 ====================

// Filter 过滤器接口
type Filter interface {
	Match(item *CIItem) bool
}

// SystemFilter 业务系统过滤器
type SystemFilter struct {
	SystemIDs []string
}

// Match 实现Filter接口
func (f *SystemFilter) Match(item *CIItem) bool {
	for _, id := range f.SystemIDs {
		if item.SystemID == id {
			return true
		}
	}
	return false
}

// LevelFilter 系统等级过滤器
type LevelFilter struct {
	Levels []SystemLevel
}

// Match 实现Filter接口
func (f *LevelFilter) Match(item *CIItem) bool {
	if system, exists := item.system; exists {
		for _, level := range f.Levels {
			if system.Level == level {
				return true
			}
		}
	}
	return false
}

// EnvironmentFilter 环境过滤器
type EnvironmentFilter struct {
	Environments []string
}

// Match 实现Filter接口
func (f *EnvironmentFilter) Match(item *CIItem) bool {
	for _, env := range f.Environments {
		if item.Environment == env {
			return true
		}
	}
	return false
}

// FilterCIItems 使用过滤器过滤CI
func (q *QueryEngine) FilterCIItems(filters []Filter) []*CIItem {
	var result []*CIItem

	// 获取所有CI
	systems := q.syncService.GetAllSystems()
	for _, system := range systems {
		cis := q.syncService.GetCIItemsBySystem(system.ID)
		for _, ci := range cis {
			match := true
			for _, filter := range filters {
				if !filter.Match(ci) {
					match = false
					break
				}
			}
			if match {
				result = append(result, ci)
			}
		}
	}

	return result
}

// ==================== 搜索接口 ====================

// SearchResult 搜索结果
type SearchResult struct {
	Systems []*BusinessSystem `json:"systems"`
	Items   []*CIItem         `json:"items"`
	Total   int               `json:"total"`
}

// Search 搜索CMDB数据
func (q *QueryEngine) Search(keyword string) *SearchResult {
	result := &SearchResult{
		Systems: make([]*BusinessSystem, 0),
		Items:   make([]*CIItem, 0),
	}

	keyword = strings.ToLower(keyword)

	// 搜索业务系统
	systems := q.syncService.GetAllSystems()
	for _, system := range systems {
		if strings.Contains(strings.ToLower(system.Name), keyword) ||
			strings.Contains(strings.ToLower(system.Code), keyword) ||
			strings.Contains(strings.ToLower(system.Owner), keyword) {
			result.Systems = append(result.Systems, system)
		}
	}

	// 搜索CI（简化实现）
	for _, system := range systems {
		cis := q.syncService.GetCIItemsBySystem(system.ID)
		for _, ci := range cis {
			if strings.Contains(strings.ToLower(ci.Name), keyword) ||
				strings.Contains(strings.ToLower(ci.Hostname), keyword) ||
				strings.Contains(ci.IP, keyword) {
				result.Items = append(result.Items, ci)
			}
		}
	}

	result.Total = len(result.Systems) + len(result.Items)
	return result
}

// ==================== 统计接口 ====================

// Statistics CMDB统计信息
type Statistics struct {
	TotalSystems      int            `json:"total_systems"`
	TotalCIItems      int            `json:"total_ci_items"`
	SystemByLevel     map[string]int `json:"system_by_level"`
	CIByType          map[string]int `json:"ci_by_type"`
	CIByEnvironment   map[string]int `json:"ci_by_environment"`
}

// GetStatistics 获取统计信息
func (q *QueryEngine) GetStatistics() *Statistics {
	stats := &Statistics{
		SystemByLevel:   make(map[string]int),
		CIByType:        make(map[string]int),
		CIByEnvironment: make(map[string]int),
	}

	// 统计业务系统
	systems := q.syncService.GetAllSystems()
	stats.TotalSystems = len(systems)

	for _, system := range systems {
		stats.SystemByLevel[system.Level.String()]++

		// 统计CI
		cis := q.syncService.GetCIItemsBySystem(system.ID)
		for _, ci := range cis {
			stats.TotalCIItems++
			stats.CIByType[ci.Type]++
			if ci.Environment != "" {
				stats.CIByEnvironment[ci.Environment]++
			}
		}
	}

	return stats
}
