// Package huaweicloud 提供华为云Stack V8 API对接功能
// 本文件实现与告警模块的集成
package huaweicloud

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud-flow-agent/internal/alert"
	"cloud-flow-agent/pkg/logger"
)

// AlertIntegration 告警模块集成
type AlertIntegration struct {
	client         *Client
	syncService    *SyncService
	labelInjector  *LabelInjector
	queryService   *QueryService
	store          AssetStore
	alertManager   *alert.AlertManager
	log            *logger.Logger
	config         IntegrationConfig
	
	// 控制
	stopCh         chan struct{}
	wg             sync.WaitGroup
	isRunning      bool
	mu             sync.RWMutex
}

// IntegrationConfig 集成配置
type IntegrationConfig struct {
	Enabled              bool          `yaml:"enabled" json:"enabled"`
	AutoSyncOnStart      bool          `yaml:"auto_sync_on_start" json:"auto_sync_on_start"`
	LabelInjectionEnabled bool         `yaml:"label_injection_enabled" json:"label_injection_enabled"`
	QueryEnabled         bool          `yaml:"query_enabled" json:"query_enabled"`
	SyncInterval         time.Duration `yaml:"sync_interval" json:"sync_interval"`
}

// DefaultIntegrationConfig 默认集成配置
func DefaultIntegrationConfig() IntegrationConfig {
	return IntegrationConfig{
		Enabled:               true,
		AutoSyncOnStart:       true,
		LabelInjectionEnabled: true,
		QueryEnabled:          true,
		SyncInterval:          5 * time.Minute,
	}
}

// NewAlertIntegration 创建告警集成
func NewAlertIntegration(client *Client, alertManager *alert.AlertManager, log *logger.Logger, config IntegrationConfig) *AlertIntegration {
	// 创建存储
	store := NewMemoryAssetStore()
	
	// 创建同步服务
	syncConfig := DefaultSyncConfig()
	syncConfig.Interval = config.SyncInterval
	syncService := NewSyncService(client, store, log, syncConfig)
	
	// 创建标签注入器
	labelConfig := DefaultLabelConfig()
	labelInjector := NewLabelInjector(store, log, labelConfig)
	
	// 创建查询服务
	queryConfig := DefaultQueryConfig()
	queryService := NewQueryService(store, log, queryConfig)
	
	return &AlertIntegration{
		client:        client,
		syncService:   syncService,
		labelInjector: labelInjector,
		queryService:  queryService,
		store:         store,
		alertManager:  alertManager,
		log:           log,
		config:        config,
		stopCh:        make(chan struct{}),
	}
}

// Start 启动集成
func (i *AlertIntegration) Start() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	
	if i.isRunning {
		return fmt.Errorf("集成已在运行")
	}
	
	if !i.config.Enabled {
		i.log.Info("华为云集成已禁用")
		return nil
	}
	
	// 设置同步回调
	i.setupSyncCallbacks()
	
	// 启动同步服务
	if err := i.syncService.Start(); err != nil {
		return fmt.Errorf("启动同步服务失败: %w", err)
	}
	
	i.isRunning = true
	i.log.Info("华为云告警集成已启动")
	
	return nil
}

// Stop 停止集成
func (i *AlertIntegration) Stop() {
	i.mu.Lock()
	if !i.isRunning {
		i.mu.Unlock()
		return
	}
	i.isRunning = false
	i.mu.Unlock()
	
	// 停止同步服务
	i.syncService.Stop()
	
	close(i.stopCh)
	i.wg.Wait()
	
	i.log.Info("华为云告警集成已停止")
}

// IsRunning 检查是否运行中
func (i *AlertIntegration) IsRunning() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.isRunning
}

// setupSyncCallbacks 设置同步回调
func (i *AlertIntegration) setupSyncCallbacks() {
	// VM同步回调
	i.syncService.SetVMSyncCallback(func(vms []*VM) {
		i.log.Infof("同步了 %d 台VM，刷新标签注入器缓存", len(vms))
		
		// 刷新标签注入器缓存
		if err := i.labelInjector.RefreshCache(); err != nil {
			i.log.Errorf("刷新标签注入器缓存失败: %v", err)
		}
		
		// 刷新查询服务索引
		if err := i.queryService.BuildIndex(); err != nil {
			i.log.Errorf("刷新查询索引失败: %v", err)
		}
	})
}

// ==================== 标签注入集成 ====================

// InjectLabelsToMetrics 为指标注入云资产标签
func (i *AlertIntegration) InjectLabelsToMetrics(metrics []alert.MetricData) []alert.MetricData {
	if !i.config.LabelInjectionEnabled {
		return metrics
	}
	
	// 转换指标数据格式
	hwMetrics := make([]MetricData, len(metrics))
	for idx, m := range metrics {
		hwMetrics[idx] = MetricData{
			Name:      m.Name,
			Value:     m.Value,
			Labels:    m.Labels,
			Timestamp: m.Timestamp,
		}
	}
	
	// 注入标签
	injectedMetrics := i.labelInjector.InjectLabelsToMetrics(hwMetrics)
	
	// 转换回alert.MetricData
	result := make([]alert.MetricData, len(injectedMetrics))
	for idx, m := range injectedMetrics {
		result[idx] = alert.MetricData{
			Name:      m.Name,
			Value:     m.Value,
			Labels:    m.Labels,
			Timestamp: m.Timestamp,
		}
	}
	
	return result
}

// InjectLabelsToAlert 为告警注入云资产标签
func (i *AlertIntegration) InjectLabelsToAlert(event *alert.AlertEvent) *alert.AlertEvent {
	if !i.config.LabelInjectionEnabled || event == nil {
		return event
	}
	
	// 转换告警格式
	hwAlert := &AlertEvent{
		ID:          event.ID,
		RuleID:      event.RuleID,
		RuleName:    event.RuleName,
		Level:       event.Level.String(),
		Labels:      event.Labels,
		Annotations: event.Annotations,
	}
	
	// 注入标签
	injectedAlert := i.labelInjector.InjectLabelsToAlert(hwAlert)
	
	// 更新原始告警
	event.Labels = injectedAlert.Labels
	event.Annotations = injectedAlert.Annotations
	
	return event
}

// ==================== 查询集成 ====================

// QueryVMsByIP 通过IP查询VM
func (i *AlertIntegration) QueryVMsByIP(ip string) ([]*VM, error) {
	if !i.config.QueryEnabled {
		return nil, fmt.Errorf("查询功能已禁用")
	}
	
	vm, err := i.queryService.GetVMByIP(ip)
	if err != nil {
		return nil, err
	}
	
	if vm == nil {
		return []*VM{}, nil
	}
	
	return []*VM{vm}, nil
}

// QueryVMsByAlert 通过告警查询关联的VM
func (i *AlertIntegration) QueryVMsByAlert(event *alert.AlertEvent) ([]*VM, error) {
	if !i.config.QueryEnabled {
		return nil, fmt.Errorf("查询功能已禁用")
	}
	
	// 从告警标签中提取IP或主机名
	ip := event.Labels["instance"]
	if ip == "" {
		ip = event.Labels["ip"]
	}
	
	if ip != "" {
		return i.QueryVMsByIP(ip)
	}
	
	hostname := event.Labels["host"]
	if hostname != "" {
		vm, err := i.queryService.GetVMByHostname(hostname)
		if err != nil {
			return nil, err
		}
		if vm != nil {
			return []*VM{vm}, nil
		}
	}
	
	return []*VM{}, nil
}

// GetVMDetail 获取VM详情
func (i *AlertIntegration) GetVMDetail(vmID string) (*VM, error) {
	if !i.config.QueryEnabled {
		return nil, fmt.Errorf("查询功能已禁用")
	}
	
	return i.store.GetVM(vmID)
}

// GetVPCDetail 获取VPC详情
func (i *AlertIntegration) GetVPCDetail(vpcID string) (*VPC, error) {
	if !i.config.QueryEnabled {
		return nil, fmt.Errorf("查询功能已禁用")
	}
	
	return i.store.GetVPC(vpcID)
}

// GetHostDetail 获取宿主机详情
func (i *AlertIntegration) GetHostDetail(hostID string) (*Host, error) {
	if !i.config.QueryEnabled {
		return nil, fmt.Errorf("查询功能已禁用")
	}
	
	return i.store.GetHost(hostID)
}

// ==================== 告警关联查询 ====================

// GetRelatedAlertsByVM 获取与VM关联的所有告警
func (i *AlertIntegration) GetRelatedAlertsByVM(vmID string) ([]*alert.AlertEvent, error) {
	// 获取VM
	vm, err := i.store.GetVM(vmID)
	if err != nil {
		return nil, err
	}
	if vm == nil {
		return []*alert.AlertEvent{}, nil
	}
	
	// 获取所有活跃告警
	alerts := i.alertManager.GetActiveAlerts()
	
	// 过滤与VM相关的告警
	var relatedAlerts []*alert.AlertEvent
	for _, alertEvent := range alerts {
		// 检查IP匹配
		for _, ip := range vm.PrivateIPs {
			if alertEvent.Labels["instance"] == ip || alertEvent.Labels["ip"] == ip {
				relatedAlerts = append(relatedAlerts, alertEvent)
				break
			}
		}
		// 检查主机名匹配
		if alertEvent.Labels["host"] == vm.Name {
			relatedAlerts = append(relatedAlerts, alertEvent)
		}
	}
	
	return relatedAlerts, nil
}

// GetRelatedAlertsByVPC 获取与VPC关联的所有告警
func (i *AlertIntegration) GetRelatedAlertsByVPC(vpcID string) ([]*alert.AlertEvent, error) {
	// 获取VPC下的所有VM
	vms, err := i.queryService.GetVMsByVPC(vpcID)
	if err != nil {
		return nil, err
	}
	
	// 收集所有相关告警
	alertMap := make(map[string]*alert.AlertEvent)
	for _, vm := range vms {
		alerts, _ := i.GetRelatedAlertsByVM(vm.ID)
		for _, alertEvent := range alerts {
			alertMap[alertEvent.ID] = alertEvent
		}
	}
	
	// 转换为切片
	result := make([]*alert.AlertEvent, 0, len(alertMap))
	for _, alertEvent := range alertMap {
		result = append(result, alertEvent)
	}
	
	return result, nil
}

// ==================== 统计信息 ====================

// GetIntegrationStatus 获取集成状态
func (i *AlertIntegration) GetIntegrationStatus() map[string]interface{} {
	return map[string]interface{}{
		"enabled":              i.config.Enabled,
		"is_running":           i.IsRunning(),
		"label_injection":      i.config.LabelInjectionEnabled,
		"query_enabled":        i.config.QueryEnabled,
		"sync_status":          i.syncService.GetSyncStatus(),
		"label_cache_stats":    i.labelInjector.GetCacheStats(),
		"query_index_stats":    i.queryService.GetIndexStats(),
		"asset_stats":          i.queryService.GetAssetStats(),
	}
}

// GetAssetStats 获取资产统计
func (i *AlertIntegration) GetAssetStats() map[string]interface{} {
	return i.queryService.GetAssetStats()
}

// ==================== 手动同步 ====================

// TriggerSync 触发手动同步
func (i *AlertIntegration) TriggerSync() error {
	return i.syncService.FullSync()
}

// TriggerIncrementalSync 触发增量同步
func (i *AlertIntegration) TriggerIncrementalSync() error {
	return i.syncService.IncrementalSync()
}

// ==================== 配置更新 ====================

// UpdateConfig 更新配置
func (i *AlertIntegration) UpdateConfig(config IntegrationConfig) {
	i.mu.Lock()
	defer i.mu.Unlock()
	
	i.config = config
	
	// 更新同步服务配置
	syncConfig := i.syncService.config
	syncConfig.Enabled = config.Enabled
	syncConfig.Interval = config.SyncInterval
	
	i.log.Info("华为云集成配置已更新")
}
