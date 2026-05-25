// Package dynconfig 动态配置引擎
// 运行时修改采样率、指标项，热加载不重启
package dynconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"
)

// ============================================================
// 配置变更事件
// ============================================================

// ChangeEvent 配置变更事件
type ChangeEvent struct {
	Path      string      `json:"path"`       // 配置路径，如 "ebpf_resource.sample_rate_base"
	OldValue  interface{} `json:"old_value"`
	NewValue  interface{} `json:"new_value"`
	Timestamp time.Time   `json:"timestamp"`
	Source    string      `json:"source"`     // api/file/watch
}

// ChangeHandler 配置变更处理器
type ChangeHandler func(event *ChangeEvent)

// ============================================================
// 动态配置引擎
// ============================================================

// Engine 动态配置引擎
type Engine struct {
	configPath string
	data       map[string]interface{}
	mu         sync.RWMutex
	
	handlers   []ChangeHandler
	handlerMu  sync.RWMutex
	
	watchMode  bool
	stopCh     chan struct{}
	wg         sync.WaitGroup
	
	// 变更历史
	history    []*ChangeEvent
	maxHistory int
}

// NewEngine 创建动态配置引擎
func NewEngine(configPath string) *Engine {
	return &Engine{
		configPath: configPath,
		data:       make(map[string]interface{}),
		stopCh:     make(chan struct{}),
		maxHistory: 100,
		history:    make([]*ChangeEvent, 0, 100),
	}
}

// Load 加载配置文件
func (e *Engine) Load() error {
	data, err := os.ReadFile(e.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}
	
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}
	
	e.mu.Lock()
	e.data = config
	e.mu.Unlock()
	
	return nil
}

// LoadFromMap 从map加载配置
func (e *Engine) LoadFromMap(data map[string]interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.data = data
}

// Get 获取配置值
func (e *Engine) Get(path string) (interface{}, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	keys := splitPath(path)
	var current interface{} = e.data
	
	for _, key := range keys {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("路径 %s 不存在: %s 不是map", path, key)
		}
		
		val, exists := m[key]
		if !exists {
			return nil, fmt.Errorf("路径 %s 不存在: 键 %s 未找到", path, key)
		}
		current = val
	}
	
	return current, nil
}

// GetInt 获取整型配置
func (e *Engine) GetInt(path string, defaultVal int) int {
	val, err := e.Get(path)
	if err != nil {
		return defaultVal
	}
	
	switch v := val.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	}
	return defaultVal
}

// GetFloat 获取浮点配置
func (e *Engine) GetFloat(path string, defaultVal float64) float64 {
	val, err := e.Get(path)
	if err != nil {
		return defaultVal
	}
	
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	}
	return defaultVal
}

// GetBool 获取布尔配置
func (e *Engine) GetBool(path string, defaultVal bool) bool {
	val, err := e.Get(path)
	if err != nil {
		return defaultVal
	}
	
	switch v := val.(type) {
	case bool:
		return v
	}
	return defaultVal
}

// GetString 获取字符串配置
func (e *Engine) GetString(path string, defaultVal string) string {
	val, err := e.Get(path)
	if err != nil {
		return defaultVal
	}
	
	switch v := val.(type) {
	case string:
		return v
	}
	return defaultVal
}

// GetSlice 获取数组配置
func (e *Engine) GetSlice(path string) []interface{} {
	val, err := e.Get(path)
	if err != nil {
		return nil
	}
	
	if arr, ok := val.([]interface{}); ok {
		return arr
	}
	return nil
}

// GetStringSlice 获取字符串数组配置
func (e *Engine) GetStringSlice(path string) []string {
	arr := e.GetSlice(path)
	if arr == nil {
		return nil
	}
	
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// Set 设置配置值（运行时动态修改）
func (e *Engine) Set(path string, value interface{}) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	keys := splitPath(path)
	if len(keys) == 0 {
		return fmt.Errorf("无效的配置路径")
	}
	
	// 获取旧值
	oldValue := e.getLocked(path)
	
	// 设置新值
	lastIdx := len(keys) - 1
	current := e.data
	
	for i := 0; i < lastIdx; i++ {
		m, ok := current.(map[string]interface{})
		if !ok {
			m = make(map[string]interface{})
			// 需要回溯设置
			current = m
		}
		
		nextKey := keys[i]
		if nextVal, exists := m[nextKey]; exists {
			current = nextVal
		} else {
			newMap := make(map[string]interface{})
			m[nextKey] = newMap
			current = newMap
		}
	}
	
	if m, ok := current.(map[string]interface{}); ok {
		m[keys[lastIdx]] = value
	} else {
		return fmt.Errorf("路径 %s 的父节点不是map", path)
	}
	
	// 触发变更事件
	event := &ChangeEvent{
		Path:      path,
		OldValue:  oldValue,
		NewValue:  value,
		Timestamp: time.Now(),
		Source:    "api",
	}
	
	e.recordEvent(event)
	e.notifyHandlers(event)
	
	return nil
}

// getLocked 内部获取（不加锁）
func (e *Engine) getLocked(path string) interface{} {
	keys := splitPath(path)
	var current interface{} = e.data
	
	for _, key := range keys {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		val, exists := m[key]
		if !exists {
			return nil
		}
		current = val
	}
	
	return current
}

// OnChange 注册配置变更处理器
func (e *Engine) OnChange(handler ChangeHandler) {
	e.handlerMu.Lock()
	defer e.handlerMu.Unlock()
	e.handlers = append(e.handlers, handler)
}

// notifyHandlers 通知处理器
func (e *Engine) notifyHandlers(event *ChangeEvent) {
	e.handlerMu.RLock()
	handlers := make([]ChangeHandler, len(e.handlers))
	copy(handlers, e.handlers)
	e.handlerMu.RUnlock()
	
	for _, h := range handlers {
		go h(event)
	}
}

// recordEvent 记录变更事件
func (e *Engine) recordEvent(event *ChangeEvent) {
	e.history = append(e.history, event)
	if len(e.history) > e.maxHistory {
		e.history = e.history[len(e.history)-e.maxHistory:]
	}
}

// GetHistory 获取变更历史
func (e *Engine) GetHistory() []*ChangeEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	result := make([]*ChangeEvent, len(e.history))
	copy(result, e.history)
	return result
}

// ============================================================
// 采样率动态调整
// ============================================================

// SampleRateConfig 采样率配置
type SampleRateConfig struct {
	BaseRate   int `json:"base_rate"`    // 基础采样率 (Hz)
	MinRate    int `json:"min_rate"`     // 最小采样率
	MaxRate    int `json:"max_rate"`     // 最大采样率
	CurrentRate int `json:"current_rate"` // 当前采样率
}

// AdjustSampleRate 动态调整采样率
func (e *Engine) AdjustSampleRate(module string, newRate int) error {
	path := fmt.Sprintf("%s.sample_rate_base", module)
	
	if newRate < 0 {
		return fmt.Errorf("采样率不能为负数")
	}
	
	minRate := e.GetInt(fmt.Sprintf("%s.sample_rate_min", module), 100)
	maxRate := e.GetInt(fmt.Sprintf("%s.sample_rate_max", module), 10000)
	
	if newRate < minRate {
		newRate = minRate
	}
	if newRate > maxRate {
		newRate = maxRate
	}
	
	return e.Set(path, newRate)
}

// GetSampleRate 获取当前采样率
func (e *Engine) GetSampleRate(module string) int {
	return e.GetInt(fmt.Sprintf("%s.sample_rate_base", module), 1000)
}

// ============================================================
// 指标项动态配置
// ============================================================

// MetricConfig 指标配置
type MetricConfig struct {
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Interval int    `json:"interval"` // 采集间隔（秒）
}

// EnableMetric 启用指标
func (e *Engine) EnableMetric(module, metric string) error {
	path := fmt.Sprintf("%s.%s.enabled", module, metric)
	return e.Set(path, true)
}

// DisableMetric 禁用指标
func (e *Engine) DisableMetric(module, metric string) error {
	path := fmt.Sprintf("%s.%s.enabled", module, metric)
	return e.Set(path, false)
}

// SetMetricInterval 设置指标采集间隔
func (e *Engine) SetMetricInterval(module, metric string, interval int) error {
	path := fmt.Sprintf("%s.%s.interval", module, metric)
	return e.Set(path, interval)
}

// GetMetricConfig 获取指标配置
func (e *Engine) GetMetricConfig(module string) map[string]*MetricConfig {
	configs := make(map[string]*MetricConfig)
	
	// 获取模块下的所有配置
	val, err := e.Get(module)
	if err != nil {
		return configs
	}
	
	if m, ok := val.(map[string]interface{}); ok {
		for key, subVal := range m {
			if sub, ok := subVal.(map[string]interface{}); ok {
				enabled := true
				interval := 60
				
				if v, ok := sub["enabled"].(bool); ok {
					enabled = v
				}
				if v, ok := sub["interval"].(float64); ok {
					interval = int(v)
				}
				
				configs[key] = &MetricConfig{
					Name:     key,
					Enabled:  enabled,
					Interval: interval,
				}
			}
		}
	}
	
	return configs
}

// ============================================================
// 配置快照与回滚
// ============================================================

// Snapshot 创建配置快照
func (e *Engine) Snapshot() (map[string]interface{}, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	// 深拷贝
	snapshot := deepCopy(e.data)
	return snapshot, nil
}

// Rollback 回滚到快照
func (e *Engine) Rollback(snapshot map[string]interface{}) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	e.data = deepCopy(snapshot)
	return nil
}

// Export 导出配置为JSON
func (e *Engine) Export() ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	return json.MarshalIndent(e.data, "", "  ")
}

// Import 从JSON导入配置
func (e *Engine) Import(data []byte) error {
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析导入数据失败: %w", err)
	}
	
	e.mu.Lock()
	e.data = config
	e.mu.Unlock()
	
	return nil
}

// Save 保存到文件
func (e *Engine) Save() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	data, err := json.MarshalIndent(e.data, "", "  ")
	if err != nil {
		return err
	}
	
	// 原子写入
	dir := filepath.Dir(e.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	tmpPath := e.configPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	
	return os.Rename(tmpPath, e.configPath)
}

// ============================================================
// 文件监听
// ============================================================

// StartWatch 启动文件监听（热加载）
func (e *Engine) StartWatch(interval time.Duration) {
	e.watchMode = true
	e.wg.Add(1)
	go e.watchLoop(interval)
}

// StopWatch 停止文件监听
func (e *Engine) StopWatch() {
	e.watchMode = false
	close(e.stopCh)
	e.wg.Wait()
}

func (e *Engine) watchLoop(interval time.Duration) {
	defer e.wg.Done()
	
	var lastMod time.Time
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			info, err := os.Stat(e.configPath)
			if err != nil {
				continue
			}
			
			if info.ModTime() != lastMod {
				lastMod = info.ModTime()
				
				// 重新加载
				if err := e.Load(); err != nil {
					continue
				}
				
				// 通知变更
				event := &ChangeEvent{
					Path:      "*",
					NewValue:  "file reloaded",
					Timestamp: time.Now(),
					Source:    "watch",
				}
				e.notifyHandlers(event)
			}
			
		case <-e.stopCh:
			return
		}
	}
}

// ============================================================
// 工具函数
// ============================================================

func splitPath(path string) []string {
	return strings.Split(path, ".")
}

func deepCopy(src map[string]interface{}) map[string]interface{} {
	data, _ := json.Marshal(src)
	var dst map[string]interface{}
	json.Unmarshal(data, &dst)
	return dst
}

// ============================================================
// 导入
// ============================================================
