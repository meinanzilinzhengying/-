//go:build linux

// Package cmdb 提供CMDB系统对接功能
// 本文件是模块入口，提供统一的初始化接口
package cmdb

import (
	"cloud-flow-agent/pkg/logger"
)

// ModuleConfig CMDB模块配置
type ModuleConfig struct {
	Enabled   bool           `yaml:"enabled" json:"enabled"`     // 是否启用
	Client    ClientConfig   `yaml:"client" json:"client"`       // 客户端配置
	Sync      SyncConfig     `yaml:"sync" json:"sync"`           // 同步配置
	Injector  InjectorConfig `yaml:"injector" json:"injector"`   // 注入器配置
}

// DefaultModuleConfig 默认模块配置
func DefaultModuleConfig() ModuleConfig {
	return ModuleConfig{
		Enabled:  true,
		Client:   DefaultClientConfig(),
		Sync:     DefaultSyncConfig(),
		Injector: DefaultInjectorConfig(),
	}
}

// Module CMDB模块
type Module struct {
	config    ModuleConfig
	client    *Client
	syncService *SyncService
	injector  *LabelInjector
	queryEngine *QueryEngine
	log       *logger.Logger
}

// NewModule 创建CMDB模块
func NewModule(config ModuleConfig, log *logger.Logger) *Module {
	return &Module{
		config: config,
		log:    log,
	}
}

// Initialize 初始化模块
func (m *Module) Initialize() error {
	if !m.config.Enabled {
		m.log.Info("CMDB模块已禁用")
		return nil
	}

	m.log.Info("初始化CMDB模块")

	// 创建客户端
	m.client = NewClient(m.config.Client, m.log)

	// 测试连接
	if err := m.client.Authenticate(); err != nil {
		return err
	}

	// 创建同步服务
	m.syncService = NewSyncService(m.config.Sync, m.client, m.log)

	// 创建标签注入器
	m.injector = NewLabelInjector(m.config.Injector, m.syncService, m.log)

	// 创建查询引擎
	m.queryEngine = NewQueryEngine(m.syncService, m.log)

	m.log.Info("CMDB模块初始化完成")
	return nil
}

// Start 启动模块
func (m *Module) Start() error {
	if !m.config.Enabled {
		return nil
	}

	// 启动同步服务
	if err := m.syncService.Start(); err != nil {
		return err
	}

	// 构建查询索引
	m.queryEngine.BuildIndex()

	m.log.Info("CMDB模块已启动")
	return nil
}

// Stop 停止模块
func (m *Module) Stop() {
	if !m.config.Enabled {
		return
	}

	m.syncService.Stop()
	m.log.Info("CMDB模块已停止")
}

// GetClient 获取客户端
func (m *Module) GetClient() *Client {
	return m.client
}

// GetSyncService 获取同步服务
func (m *Module) GetSyncService() *SyncService {
	return m.syncService
}

// GetInjector 获取标签注入器
func (m *Module) GetInjector() *LabelInjector {
	return m.injector
}

// GetQueryEngine 获取查询引擎
func (m *Module) GetQueryEngine() *QueryEngine {
	return m.queryEngine
}

// InjectLabels 便捷方法：注入标签
func (m *Module) InjectLabels(data map[string]string) map[string]string {
	if m.injector == nil {
		return data
	}
	return m.injector.InjectLabels(data)
}

// QuerySystems 便捷方法：查询业务系统
func (m *Module) QuerySystems(req QueryRequest) []*BusinessSystem {
	if m.queryEngine == nil {
		return nil
	}
	return m.queryEngine.QuerySystems(req)
}

// QueryCIItems 便捷方法：查询CI
func (m *Module) QueryCIItems(req QueryRequest) []*CIItem {
	if m.queryEngine == nil {
		return nil
	}
	return m.queryEngine.QueryCIItems(req)
}

// GetStatus 获取模块状态
func (m *Module) GetStatus() map[string]interface{} {
	if m.syncService == nil {
		return map[string]interface{}{
			"enabled": m.config.Enabled,
			"status":  "not_initialized",
		}
	}

	return map[string]interface{}{
		"enabled":     m.config.Enabled,
		"status":      "running",
		"sync_status": m.syncService.GetStatus(),
		"sync_stats":  m.syncService.GetStats(),
	}
}
