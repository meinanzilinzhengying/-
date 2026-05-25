// Package cluster 提供双中心数据同步与故障切换能力
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/meinanzilinzhengying/cloud-flow-agent/pkg/models"
)

// ============================================================
// 双中心数据同步引擎
// ============================================================

// CenterRole 中心角色
type CenterRole string

const (
	CenterRolePrimary   CenterRole = "primary"   // 主中心
	CenterRoleSecondary CenterRole = "secondary" // 备中心
	CenterRoleActive    CenterRole = "active"    // 活跃中心（AA模式）
	CenterRoleStandby   CenterRole = "standby"   // 待命中心（AA模式）
)

// SyncMode 同步模式
type SyncMode string

const (
	SyncModeSync      SyncMode = "sync"       // 同步复制
	SyncModeAsync     SyncMode = "async"      // 异步复制
	SyncModeSemiSync  SyncMode = "semi_sync"  // 半同步复制
)

// DataType 数据类型
type DataType string

const (
	DataTypeMetric      DataType = "metric"       // 指标数据
	DataTypeTrace       DataType = "trace"        // 链路追踪
	DataTypeAlert       DataType = "alert"        // 告警数据
	DataTypeConfig      DataType = "config"       // 配置数据
	DataTypeTopology    DataType = "topology"     // 拓扑数据
	DataTypeAgentStatus DataType = "agent_status" // Agent状态
)

// DataBatch 数据批次
type DataBatch struct {
	BatchID   string          `json:"batch_id"`
	DataType  DataType        `json:"data_type"`
	Source    string          `json:"source"`     // 来源中心ID
	SeqNo     int64           `json:"seq_no"`     // 序列号
	Timestamp time.Time       `json:"timestamp"`
	Count     int             `json:"count"`
	Data      json.RawMessage `json:"data"`
	Checksum  string          `json:"checksum"`
}

// SyncAck 同步确认
type SyncAck struct {
	BatchID    string    `json:"batch_id"`
	Source     string    `json:"source"`
	Target     string    `json:"target"`
	SeqNo      int64     `json:"seq_no"`
	AckTime    time.Time `json:"ack_time"`
	Success    bool      `json:"success"`
	LagMs      int64     `json:"lag_ms"` // 复制延迟（毫秒）
}

// CenterInfo 远端中心信息
type CenterInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Role      CenterRole `json:"role"`
	Address   string    `json:"address"`
	Port      int       `json:"port"`
	Region    string    `json:"region"`
	IsActive  bool      `json:"is_active"`
	LastSync  time.Time `json:"last_sync"`
	LagMs     int64     `json:"lag_ms"`
	SeqNo     int64     `json:"seq_no"`
}

// DualCenterConfig 双中心配置
type DualCenterConfig struct {
	Enabled         bool          `yaml:"enabled" json:"enabled"`
	LocalCenterID   string        `yaml:"local_center_id" json:"local_center_id"`
	LocalCenterName string        `yaml:"local_center_name" json:"local_center_name"`
	LocalRole       CenterRole    `yaml:"local_role" json:"local_role"`
	LocalAddress    string        `yaml:"local_address" json:"local_address"`
	LocalPort       int           `yaml:"local_port" json:"local_port"`
	Region          string        `yaml:"region" json:"region"`

	// 远端中心
	PeerCenters []PeerCenterConfig `yaml:"peer_centers" json:"peer_centers"`

	// 同步配置
	SyncMode        SyncMode     `yaml:"sync_mode" json:"sync_mode"`
	BatchSize       int          `yaml:"batch_size" json:"batch_size"`
	FlushInterval   time.Duration `yaml:"flush_interval" json:"flush_interval"`
	CompressEnabled bool         `yaml:"compress_enabled" json:"compress_enabled"`
	MaxRetries      int          `yaml:"max_retries" json:"max_retries"`
	RetryDelay      time.Duration `yaml:"retry_delay" json:"retry_delay"`

	// 队列配置
	QueueSize       int `yaml:"queue_size" json:"queue_size"`
	QueueOverflow   string `yaml:"queue_overflow" json:"queue_overflow"` // drop_latest / drop_oldest / block
}

// PeerCenterConfig 对端中心配置
type PeerCenterConfig struct {
	ID        string `yaml:"id" json:"id"`
	Name      string `yaml:"name" json:"name"`
	Role      CenterRole `yaml:"role" json:"role"`
	Address   string `yaml:"address" json:"address"`
	Port      int    `yaml:"port" json:"port"`
	Region    string `yaml:"region" json:"region"`
	TLSEnabled bool  `yaml:"tls_enabled" json:"tls_enabled"`
}

// DualCenterSync 双中心数据同步引擎
type DualCenterSync struct {
	config *DualCenterConfig

	// 本地中心
	localCenter *CenterInfo

	// 远端中心
	peerCenters map[string]*CenterInfo
	peerClients map[string]*http.Client

	// 数据队列（按数据类型分队列）
	queues     map[DataType]chan *DataBatch
	queueMutex sync.RWMutex

	// 序列号（按远端中心+数据类型）
	seqNos     map[string]map[DataType]int64
	seqNoMutex sync.Mutex

	// 统计
	stats      *SyncStats

	// 生命周期
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	running    atomic.Bool

	// 回调
	onPeerDown func(centerID string)
	onPeerUp   func(centerID string)

	mu         sync.RWMutex
}

// SyncStats 同步统计
type SyncStats struct {
	TotalSent     atomic.Int64 `json:"total_sent"`
	TotalRecv     atomic.Int64 `json:"total_recv"`
	TotalFailed   atomic.Int64 `json:"total_failed"`
	TotalRetried  atomic.Int64 `json:"total_retried"`
	CurrentLagMs  atomic.Int64 `json:"current_lag_ms"`
	QueueLen      atomic.Int64 `json:"queue_len"`
	LastSyncTime  atomic.Int64 `json:"last_sync_time"` // unix timestamp
}

// NewDualCenterSync 创建双中心同步引擎
func NewDualCenterSync(cfg *DualCenterConfig) (*DualCenterSync, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("dual center sync is not enabled")
	}

	ctx, cancel := context.WithCancel(context.Background())

	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 10000
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 500
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = 2 * time.Second
	}
	if cfg.QueueOverflow == "" {
		cfg.QueueOverflow = "drop_oldest"
	}

	dcs := &DualCenterSync{
		config:      cfg,
		localCenter: &CenterInfo{
			ID:       cfg.LocalCenterID,
			Name:     cfg.LocalCenterName,
			Role:     cfg.LocalRole,
			Address:  cfg.LocalAddress,
			Port:     cfg.LocalPort,
			Region:   cfg.Region,
			IsActive: true,
		},
		peerCenters: make(map[string]*CenterInfo),
		peerClients: make(map[string]*http.Client),
		queues:      make(map[DataType]chan *DataBatch),
		seqNos:      make(map[string]map[DataType]int64),
		stats:       &SyncStats{},
		ctx:         ctx,
		cancel:      cancel,
	}

	// 初始化远端中心
	for _, pc := range cfg.PeerCenters {
		dcs.peerCenters[pc.ID] = &CenterInfo{
			ID:       pc.ID,
			Name:     pc.Name,
			Role:     pc.Role,
			Address:  pc.Address,
			Port:     pc.Port,
			Region:   pc.Region,
		}

		// 创建HTTP客户端
		transport := &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     90 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		}
		dcs.peerClients[pc.ID] = &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}

		// 初始化序列号
		dcs.seqNos[pc.ID] = make(map[DataType]int64)
	}

	// 初始化数据队列
	for _, dt := range []DataType{
		DataTypeMetric, DataTypeTrace, DataTypeAlert,
		DataTypeConfig, DataTypeTopology, DataTypeAgentStatus,
	} {
		dcs.queues[dt] = make(chan *DataBatch, cfg.QueueSize)
	}

	return dcs, nil
}

// Start 启动双中心同步
func (dcs *DualCenterSync) Start() error {
	if dcs.running.Load() {
		return fmt.Errorf("dual center sync already running")
	}

	dcs.running.Store(true)

	// 为每个远端中心启动同步goroutine
	for peerID := range dcs.peerCenters {
		for _, dt := range []DataType{
			DataTypeMetric, DataTypeTrace, DataTypeAlert,
			DataTypeConfig, DataTypeTopology, DataTypeAgentStatus,
		} {
			dcs.wg.Add(1)
			go dcs.syncWorker(peerID, dt)
		}

		// 启动对端健康检查
		dcs.wg.Add(1)
		go dcs.peerHealthChecker(peerID)
	}

	// 启动HTTP接收服务
	dcs.wg.Add(1)
	go dcs.serveRecv()

	return nil
}

// Stop 停止双中心同步
func (dcs *DualCenterSync) Stop() error {
	if !dcs.running.Load() {
		return nil
	}

	dcs.running.Store(false)
	dcs.cancel()
	dcs.wg.Wait()
	return nil
}

// Push 推送数据到同步队列
func (dcs *DualCenterSync) Push(dataType DataType, data interface{}) error {
	if !dcs.running.Load() {
		return fmt.Errorf("dual center sync is not running")
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	batch := &DataBatch{
		BatchID:   generateBatchID(),
		DataType:  dataType,
		Source:    dcs.config.LocalCenterID,
		Timestamp: time.Now(),
		Count:     1,
		Data:      dataBytes,
		Checksum:  checksum(dataBytes),
	}

	// 放入对应数据类型的队列
	queue, ok := dcs.queues[dataType]
	if !ok {
		return fmt.Errorf("unknown data type: %s", dataType)
	}

	select {
	case queue <- batch:
		dcs.stats.QueueLen.Add(1)
		return nil
	default:
		// 队列满，根据溢出策略处理
		switch dcs.config.QueueOverflow {
		case "drop_latest":
			return fmt.Errorf("queue full, dropping latest")
		case "drop_oldest":
			select {
			case <-queue:
				dcs.stats.QueueLen.Add(-1)
			default:
			}
			queue <- batch
			return nil
		case "block":
			queue <- batch
			dcs.stats.QueueLen.Add(1)
			return nil
		default:
			return fmt.Errorf("queue full for type %s", dataType)
		}
	}
}

// PushBatch 批量推送数据
func (dcs *DualCenterSync) PushBatch(dataType DataType, items []interface{}) error {
	if len(items) == 0 {
		return nil
	}

	dataBytes, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("failed to marshal batch: %w", err)
	}

	batch := &DataBatch{
		BatchID:   generateBatchID(),
		DataType:  dataType,
		Source:    dcs.config.LocalCenterID,
		Timestamp: time.Now(),
		Count:     len(items),
		Data:      dataBytes,
		Checksum:  checksum(dataBytes),
	}

	queue, ok := dcs.queues[dataType]
	if !ok {
		return fmt.Errorf("unknown data type: %s", dataType)
	}

	select {
	case queue <- batch:
		dcs.stats.QueueLen.Add(1)
		return nil
	default:
		// 尝试丢弃最旧
		if dcs.config.QueueOverflow == "drop_oldest" {
			select {
			case <-queue:
			default:
			}
			queue <- batch
			return nil
		}
		return fmt.Errorf("queue full for type %s", dataType)
	}
}

// syncWorker 同步工作协程
func (dcs *DualCenterSync) syncWorker(peerID string, dataType DataType) {
	defer dcs.wg.Done()

	queue := dcs.queues[dataType]
	flushTicker := time.NewTicker(dcs.config.FlushInterval)
	defer flushTicker.Stop()

	var batch []*DataBatch

	for {
		select {
		case <-dcs.ctx.Done():
			// 发送剩余数据
			if len(batch) > 0 {
				dcs.sendBatch(peerID, dataType, batch)
			}
			return

		case item, ok := <-queue:
			if !ok {
				return
			}
			dcs.stats.QueueLen.Add(-1)
			batch = append(batch, item)

			if len(batch) >= dcs.config.BatchSize {
				dcs.sendBatch(peerID, dataType, batch)
				batch = batch[:0]
			}

		case <-flushTicker.C:
			if len(batch) > 0 {
				dcs.sendBatch(peerID, dataType, batch)
				batch = batch[:0]
			}
		}
	}
}

// sendBatch 发送数据批次到远端中心
func (dcs *DualCenterSync) sendBatch(peerID string, dataType DataType, batch []*DataBatch) {
	peer, ok := dcs.peerCenters[peerID]
	if !ok {
		return
	}

	client, ok := dcs.peerClients[peerID]
	if !ok {
		return
	}

	// 合并批次
	merged := &DataBatch{
		BatchID:   generateBatchID(),
		DataType:  dataType,
		Source:    dcs.config.LocalCenterID,
		Timestamp: time.Now(),
		Count:     0,
		Data:      dcs.mergeBatchData(batch),
	}

	for _, b := range batch {
		merged.Count += b.Count
	}

	// 分配序列号
	dcs.seqNoMutex.Lock()
	seqMap := dcs.seqNos[peerID]
	seqMap[dataType]++
	merged.SeqNo = seqMap[dataType]
	dcs.seqNoMutex.Unlock()

	// 发送
	url := fmt.Sprintf("http://%s:%d/api/v1/dual-center/sync", peer.Address, peer.Port)
	data, _ := json.Marshal(merged)

	var lastErr error
	for attempt := 0; attempt < dcs.config.MaxRetries; attempt++ {
		if attempt > 0 {
			dcs.stats.TotalRetried.Add(1)
			time.Sleep(dcs.config.RetryDelay)
		}

		req, err := http.NewRequestWithContext(dcs.ctx, "POST", url, bytes.NewReader(data))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Source-Center", dcs.config.LocalCenterID)
		req.Header.Set("X-Data-Type", string(dataType))

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// 读取响应
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			// 解析ACK
			var ack SyncAck
			if json.Unmarshal(body, &ack) == nil {
				dcs.mu.Lock()
				peer.LastSync = time.Now()
				peer.LagMs = ack.LagMs
				peer.SeqNo = ack.SeqNo
				dcs.mu.Unlock()

				dcs.stats.TotalSent.Add(int64(merged.Count))
				dcs.stats.CurrentLagMs.Store(ack.LagMs)
				dcs.stats.LastSyncTime.Store(time.Now().Unix())
			}
			return
		}

		lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	dcs.stats.TotalFailed.Add(int64(merged.Count))
}

// mergeBatchData 合并批次数据
func (dcs *DualCenterSync) mergeBatchData(batch []*DataBatch) json.RawMessage {
	var allItems []json.RawMessage
	for _, b := range batch {
		var items []json.RawMessage
		if json.Unmarshal(b.Data, &items) == nil {
			allItems = append(allItems, items...)
		} else {
			allItems = append(allItems, b.Data)
		}
	}
	merged, _ := json.Marshal(allItems)
	return merged
}

// peerHealthChecker 对端健康检查
func (dcs *DualCenterSync) peerHealthChecker(peerID string) {
	defer dcs.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	consecutiveFailures := 0
	peerWasUp := true

	for {
		select {
		case <-dcs.ctx.Done():
			return
		case <-ticker.C:
			peer := dcs.peerCenters[peerID]
			client := dcs.peerClients[peerID]

			url := fmt.Sprintf("http://%s:%d/api/v1/dual-center/health", peer.Address, peer.Port)
			ctx, cancel := context.WithTimeout(dcs.ctx, 3*time.Second)
			req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
			req.Header.Set("X-Source-Center", dcs.config.LocalCenterID)

			resp, err := client.Do(req)
			cancel()

			if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
				if resp != nil {
					resp.Body.Close()
				}
				consecutiveFailures++
				if peerWasUp && consecutiveFailures >= 3 {
					peerWasUp = false
					dcs.mu.Lock()
					peer.IsActive = false
					dcs.mu.Unlock()
					if dcs.onPeerDown != nil {
						dcs.onPeerDown(peerID)
					}
				}
			} else {
				io.ReadAll(resp.Body)
				resp.Body.Close()
				consecutiveFailures = 0
				if !peerWasUp {
					peerWasUp = true
					dcs.mu.Lock()
					peer.IsActive = true
					dcs.mu.Unlock()
					if dcs.onPeerUp != nil {
						dcs.onPeerUp(peerID)
					}
				}
			}
		}
	}
}

// serveRecv 启动HTTP接收服务
func (dcs *DualCenterSync) serveRecv() {
	defer dcs.wg.Done()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/dual-center/sync", dcs.handleSync)
	mux.HandleFunc("/api/v1/dual-center/health", dcs.handleHealth)
	mux.HandleFunc("/api/v1/dual-center/seq", dcs.handleSeqQuery)

	addr := fmt.Sprintf("%s:%d", dcs.config.LocalAddress, dcs.config.LocalPort)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		<-dcs.ctx.Done()
		server.Shutdown(dcs.ctx)
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		// 启动失败
	}
}

// handleSync 处理同步请求
func (dcs *DualCenterSync) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var batch DataBatch
	if err := json.Unmarshal(body, &batch); err != nil {
		http.Error(w, "invalid batch", http.StatusBadRequest)
		return
	}

	// 校验checksum
	if batch.Checksum != "" && batch.Checksum != checksum(batch.Data) {
		http.Error(w, "checksum mismatch", http.StatusBadRequest)
		return
	}

	// 处理数据（回调给上层）
	dcs.stats.TotalRecv.Add(int64(batch.Count))
	dcs.stats.LastSyncTime.Store(time.Now().Unix())

	// 返回ACK
	lagMs := time.Since(batch.Timestamp).Milliseconds()
	ack := SyncAck{
		BatchID: batch.BatchID,
		Source:  dcs.config.LocalCenterID,
		Target:  batch.Source,
		SeqNo:   batch.SeqNo,
		AckTime: time.Now(),
		Success: true,
		LagMs:   lagMs,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ack)
}

// handleHealth 处理健康检查
func (dcs *DualCenterSync) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"center_id":   dcs.config.LocalCenterID,
		"center_name": dcs.config.LocalCenterName,
		"role":        dcs.config.LocalRole,
		"status":      "healthy",
		"timestamp":   time.Now(),
	})
}

// handleSeqQuery 处理序列号查询
func (dcs *DualCenterSync) handleSeqQuery(w http.ResponseWriter, r *http.Request) {
	dcs.seqNoMutex.Lock()
	result := make(map[string]map[DataType]int64)
	for peerID, seqMap := range dcs.seqNos {
		result[peerID] = make(map[DataType]int64)
		for dt, seq := range seqMap {
			result[peerID][dt] = seq
		}
	}
	dcs.seqNoMutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// OnPeerDown 注册对端下线回调
func (dcs *DualCenterSync) OnPeerDown(fn func(centerID string)) {
	dcs.onPeerDown = fn
}

// OnPeerUp 注册对端上线回调
func (dcs *DualCenterSync) OnPeerUp(fn func(centerID string)) {
	dcs.onPeerUp = fn
}

// GetStats 获取同步统计
func (dcs *DualCenterSync) GetStats() *SyncStats {
	return dcs.stats
}

// GetPeerCenters 获取远端中心状态
func (dcs *DualCenterSync) GetPeerCenters() map[string]*CenterInfo {
	dcs.mu.RLock()
	defer dcs.mu.RUnlock()

	result := make(map[string]*CenterInfo, len(dcs.peerCenters))
	for k, v := range dcs.peerCenters {
		cp := *v
		result[k] = &cp
	}
	return result
}

// GetLocalCenter 获取本地中心信息
func (dcs *DualCenterSync) GetLocalCenter() *CenterInfo {
	return dcs.localCenter
}

// IsRunning 是否运行中
func (dcs *DualCenterSync) IsRunning() bool {
	return dcs.running.Load()
}

// ============================================================
// 辅助函数
// ============================================================

func generateBatchID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomHex(8))
}

func randomHex(n int) string {
	const hexChars = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hexChars[time.Now().Nanosecond()%16]
	}
	return string(b)
}

func checksum(data []byte) string {
	// 简单的FNV-1a校验
	var h uint32 = 2166136261
	for _, b := range data {
		h ^= uint32(b)
		h *= 16777619
	}
	return fmt.Sprintf("%08x", h)
}

// ============================================================
// 与现有模型集成
// ============================================================

// PushMetric 推送指标数据
func (dcs *DualCenterSync) PushMetric(metric *models.SystemMetric) error {
	return dcs.Push(DataTypeMetric, metric)
}

// PushMetrics 批量推送指标数据
func (dcs *DualCenterSync) PushMetrics(metrics []*models.SystemMetric) error {
	items := make([]interface{}, len(metrics))
	for i, m := range metrics {
		items[i] = m
	}
	return dcs.PushBatch(DataTypeMetric, items)
}

// PushNetworkFlow 推送网络流量数据
func (dcs *DualCenterSync) PushNetworkFlow(flow *models.NetworkFlow) error {
	return dcs.Push(DataTypeMetric, flow)
}

// PushNetworkFlows 批量推送网络流量
func (dcs *DualCenterSync) PushNetworkFlows(flows []*models.NetworkFlow) error {
	items := make([]interface{}, len(flows))
	for i, f := range flows {
		items[i] = f
	}
	return dcs.PushBatch(DataTypeMetric, items)
}

// PushAgentStatus 推送Agent状态
func (dcs *DualCenterSync) PushAgentStatus(status *models.AgentStatus) error {
	return dcs.Push(DataTypeAgentStatus, status)
}

// ============================================================
// 数据补偿与一致性
// ============================================================

// DataRepair 数据补偿器，用于故障恢复后的数据补齐
type DataRepair struct {
	sync      *DualCenterSync
	localSeq  map[DataType]int64 // 本地各数据类型的最大序列号
	mu        sync.Mutex
}

// NewDataRepair 创建数据补偿器
func NewDataRepair(sync *DualCenterSync) *DataRepair {
	return &DataRepair{
		sync:     sync,
		localSeq: make(map[DataType]int64),
	}
}

// RecordLocalSeq 记录本地序列号
func (dr *DataRepair) RecordLocalSeq(dataType DataType, seq int64) {
	dr.mu.Lock()
	defer dr.mu.Unlock()
	if seq > dr.localSeq[dataType] {
		dr.localSeq[dataType] = seq
	}
}

// GetRepairRange 获取需要补偿的数据范围
func (dr *DataRepair) GetRepairRange(peerID string, dataType DataType) (from, to int64) {
	dr.mu.Lock()
	localSeq := dr.localSeq[dataType]
	dr.mu.Unlock()

	dr.sync.seqNoMutex.Lock()
	peerSeq := dr.sync.seqNos[peerID][dataType]
	dr.sync.seqNoMutex.Unlock()

	if peerSeq > localSeq {
		return localSeq + 1, peerSeq
	}
	return 0, 0
}

// Repair 执行数据补偿
func (dr *DataRepair) Repair(ctx context.Context, peerID string) error {
	// 查询远端缺失的数据
	peer, ok := dr.sync.peerCenters[peerID]
	if !ok {
		return fmt.Errorf("peer %s not found", peerID)
	}

	client := dr.sync.peerClients[peerID]

	for _, dt := range []DataType{
		DataTypeMetric, DataTypeTrace, DataTypeAlert,
		DataTypeConfig, DataTypeTopology, DataTypeAgentStatus,
	} {
		from, to := dr.GetRepairRange(peerID, dt)
		if from == 0 && to == 0 {
			continue
		}

		url := fmt.Sprintf("http://%s:%d/api/v1/dual-center/repair?from=%d&to=%d&type=%s",
			peer.Address, peer.Port, from, to, dt)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("X-Source-Center", dr.sync.config.LocalCenterID)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			// 处理补偿数据
			_ = body // 交给上层处理
		}
	}

	return nil
}

// ============================================================
// 双中心状态查询
// ============================================================

// DualCenterStatus 双中心状态
type DualCenterStatus struct {
	LocalCenter  *CenterInfo            `json:"local_center"`
	PeerCenters  map[string]*CenterInfo `json:"peer_centers"`
	SyncStats    *SyncStats             `json:"sync_stats"`
	SyncMode     SyncMode               `json:"sync_mode"`
	IsRunning    bool                   `json:"is_running"`
}

// GetStatus 获取双中心状态
func (dcs *DualCenterSync) GetStatus() *DualCenterStatus {
	return &DualCenterStatus{
		LocalCenter: dcs.localCenter,
		PeerCenters: dcs.GetPeerCenters(),
		SyncStats:   dcs.stats,
		SyncMode:    dcs.config.SyncMode,
		IsRunning:   dcs.running.Load(),
	}
}

// ============================================================
// 防脑裂机制
// ============================================================

// FenceToken 围栏令牌，用于防止脑裂
type FenceToken struct {
	Epoch     int64     `json:"epoch"`
	CenterID  string    `json:"center_id"`
	Role      CenterRole `json:"role"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// QuorumChecker 仲裁检查器
type QuorumChecker struct {
	peerCenters map[string]*CenterInfo
	localID     string
	quorumSize  int // 需要的仲裁数量
	mu          sync.RWMutex
}

// NewQuorumChecker 创建仲裁检查器
func NewQuorumChecker(localID string, totalCenters int) *QuorumChecker {
	// 仲裁数 = 多数派: (total/2) + 1
	quorum := totalCenters/2 + 1
	return &QuorumChecker{
		localID:    localID,
		quorumSize: quorum,
	}
}

// CanPromote 检查是否可以提升为主
func (qc *QuorumChecker) CanPromote() bool {
	qc.mu.RLock()
	defer qc.mu.RUnlock()

	activeCount := 0
	for _, peer := range qc.peerCenters {
		if peer.IsActive {
			activeCount++
		}
	}

	// 本地 + 活跃远端 >= 仲裁数
	return 1+activeCount >= qc.quorumSize
}

// UpdatePeerStatus 更新对端状态
func (qc *QuorumChecker) UpdatePeerStatus(peerID string, active bool) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	if peer, ok := qc.peerCenters[peerID]; ok {
		peer.IsActive = active
	}
}

// ============================================================
// 端口解析辅助
// ============================================================

func parsePort(addr string, defaultPort int) int {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		// 尝试从纯端口字符串解析
		p, err := strconv.Atoi(strings.TrimSpace(addr))
		if err == nil {
			return p
		}
		return defaultPort
	}
	p, err := strconv.Atoi(portStr)
	if err != nil {
		return defaultPort
	}
	return p
}
