// Package cluster 提供双中心故障检测与无缝切换能力
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// 故障检测与无缝切换
// ============================================================

// FailoverMode 故障切换模式
type FailoverMode string

const (
	FailoverModeAuto       FailoverMode = "auto"        // 自动切换
	FailoverModeManual     FailoverMode = "manual"      // 手动切换
	FailoverModeSemiAuto   FailoverMode = "semi_auto"   // 半自动（检测+确认）
)

// FailoverState 故障切换状态
type FailoverState string

const (
	FailoverStateNormal    FailoverState = "normal"     // 正常运行
	FailoverStateDetecting FailoverState = "detecting"  // 故障检测中
	FailoverStateFailing   FailoverState = "failing"    // 故障切换中
	FailoverStateRecovering FailoverState = "recovering" // 故障恢复中
	FailoverStateDegraded  FailoverState = "degraded"   // 降级运行
)

// HealthState 健康状态
type HealthState string

const (
	HealthStateHealthy   HealthState = "healthy"
	HealthStateUnhealthy HealthState = "unhealthy"
	HealthStateUnknown   HealthState = "unknown"
)

// FailoverConfig 故障切换配置
type FailoverConfig struct {
	Enabled             bool          `yaml:"enabled" json:"enabled"`
	Mode                FailoverMode  `yaml:"mode" json:"mode"`

	// 故障检测
	HeartbeatInterval   time.Duration `yaml:"heartbeat_interval" json:"heartbeat_interval"`
	HeartbeatTimeout    time.Duration `yaml:"heartbeat_timeout" json:"heartbeat_timeout"`
	FailureThreshold    int           `yaml:"failure_threshold" json:"failure_threshold"`     // 连续失败次数触发切换
	SuccessThreshold    int           `yaml:"success_threshold" json:"success_threshold"`     // 连续成功次数确认恢复

	// 切换配置
	SwitchTimeout       time.Duration `yaml:"switch_timeout" json:"switch_timeout"`           // 切换超时
	PreSwitchDelay      time.Duration `yaml:"pre_switch_delay" json:"pre_switch_delay"`       // 切换前延迟
	GracefulShutdown    bool          `yaml:"graceful_shutdown" json:"graceful_shutdown"`     // 优雅关闭
	DrainTimeout        time.Duration `yaml:"drain_timeout" json:"drain_timeout"`             // 排空超时

	// 恢复配置
	AutoRecover         bool          `yaml:"auto_recover" json:"auto_recover"`               // 自动恢复
	RecoverDelay        time.Duration `yaml:"recover_delay" json:"recover_delay"`             // 恢复延迟
	DataRepairOnRecover bool          `yaml:"data_repair_on_recover" json:"data_repair_on_recover"` // 恢复时数据补偿

	// 防脑裂
	FenceEnabled        bool          `yaml:"fence_enabled" json:"fence_enabled"`             // 启用围栏
	FenceTimeout        time.Duration `yaml:"fence_timeout" json:"fence_timeout"`             // 围栏超时
	QuorumRequired      bool          `yaml:"quorum_required" json:"quorum_required"`         // 需要仲裁
}

// DefaultFailoverConfig 默认故障切换配置
func DefaultFailoverConfig() *FailoverConfig {
	return &FailoverConfig{
		Enabled:             true,
		Mode:                FailoverModeAuto,
		HeartbeatInterval:   3 * time.Second,
		HeartbeatTimeout:    2 * time.Second,
		FailureThreshold:    3,
		SuccessThreshold:    5,
		SwitchTimeout:       30 * time.Second,
		PreSwitchDelay:      2 * time.Second,
		GracefulShutdown:    true,
		DrainTimeout:        60 * time.Second,
		AutoRecover:         true,
		RecoverDelay:        30 * time.Second,
		DataRepairOnRecover: true,
		FenceEnabled:        true,
		FenceTimeout:        10 * time.Second,
		QuorumRequired:      true,
	}
}

// FailoverManager 故障切换管理器
type FailoverManager struct {
	config *FailoverConfig
	sync   *DualCenterSync

	// 状态
	state       atomic.Value // FailoverState
	localRole   atomic.Value // CenterRole
	isActive    atomic.Bool

	// 心跳
	heartbeatResults map[string]*HeartbeatResult
	hbMutex          sync.RWMutex

	// 围栏令牌
	currentEpoch  atomic.Int64
	fenceToken    atomic.Pointer[FenceToken]

	// 切换历史
	history       []FailoverEvent
	historyMutex  sync.Mutex

	// 生命周期
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	running       atomic.Bool

	// 回调
	onFailover    func(from, to CenterRole, reason string)
	onRecover     func(centerID string)

	mu            sync.RWMutex
}

// HeartbeatResult 心跳结果
type HeartbeatResult struct {
	CenterID    string      `json:"center_id"`
	Latency     time.Duration `json:"latency"`
	Success     bool        `json:"success"`
	Error       string      `json:"error,omitempty"`
	Timestamp   time.Time   `json:"timestamp"`
	SeqNo       int64       `json:"seq_no"`
}

// FailoverEvent 故障切换事件
type FailoverEvent struct {
	ID          string        `json:"id"`
	Type        string        `json:"type"`         // failover / recover / fence
	FromRole    CenterRole    `json:"from_role"`
	ToRole      CenterRole    `json:"to_role"`
	Reason      string        `json:"reason"`
	CenterID    string        `json:"center_id"`
	Timestamp   time.Time     `json:"timestamp"`
	Duration    time.Duration `json:"duration"`
	Success     bool          `json:"success"`
}

// NewFailoverManager 创建故障切换管理器
func NewFailoverManager(cfg *FailoverConfig, sync *DualCenterSync) *FailoverManager {
	if cfg == nil {
		cfg = DefaultFailoverConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	fm := &FailoverManager{
		config:           cfg,
		sync:             sync,
		heartbeatResults: make(map[string]*HeartbeatResult),
		ctx:              ctx,
		cancel:           cancel,
	}

	fm.state.Store(FailoverStateNormal)
	fm.localRole.Store(sync.GetLocalCenter().Role)
	fm.isActive.Store(sync.GetLocalCenter().Role == CenterRolePrimary || sync.GetLocalCenter().Role == CenterRoleActive)

	return fm
}

// Start 启动故障切换管理器
func (fm *FailoverManager) Start() error {
	if fm.running.Load() {
		return fmt.Errorf("failover manager already running")
	}

	fm.running.Store(true)

	// 启动心跳检测
	fm.wg.Add(1)
	go fm.heartbeatLoop()

	// 注册同步引擎的对端状态回调
	fm.sync.OnPeerDown(fm.handlePeerDown)
	fm.sync.OnPeerUp(fm.handlePeerUp)

	return nil
}

// Stop 停止故障切换管理器
func (fm *FailoverManager) Stop() error {
	if !fm.running.Load() {
		return nil
	}

	fm.running.Store(false)
	fm.cancel()
	fm.wg.Wait()
	return nil
}

// heartbeatLoop 心跳检测循环
func (fm *FailoverManager) heartbeatLoop() {
	defer fm.wg.Done()

	ticker := time.NewTicker(fm.config.HeartbeatInterval)
	defer ticker.Stop()

	failureCounters := make(map[string]int)
	recoveryCounters := make(map[string]int)

	for {
		select {
		case <-fm.ctx.Done():
			return
		case <-ticker.C:
			peers := fm.sync.GetPeerCenters()
			for peerID, peer := range peers {
				result := fm.probePeer(peerID, peer)

				fm.hbMutex.Lock()
				fm.heartbeatResults[peerID] = result
				fm.hbMutex.Unlock()

				if result.Success {
					failureCounters[peerID] = 0
					recoveryCounters[peerID]++

					// 检查是否满足恢复条件
					if recoveryCounters[peerID] >= fm.config.SuccessThreshold {
						if fm.config.AutoRecover {
							fm.attemptRecover(peerID)
						}
						recoveryCounters[peerID] = 0
					}
				} else {
					recoveryCounters[peerID] = 0
					failureCounters[peerID]++

					// 检查是否触发故障切换
					if failureCounters[peerID] >= fm.config.FailureThreshold {
						if fm.config.Mode == FailoverModeAuto {
							fm.triggerFailover(peerID, fmt.Sprintf("heartbeat failure x%d", failureCounters[peerID]))
						} else if fm.config.Mode == FailoverModeSemiAuto {
							fm.state.Store(FailoverStateDetecting)
							// 等待手动确认
						}
						failureCounters[peerID] = 0
					}
				}
			}
		}
	}
}

// probePeer 探测对端中心
func (fm *FailoverManager) probePeer(peerID string, peer *CenterInfo) *HeartbeatResult {
	result := &HeartbeatResult{
		CenterID:  peerID,
		Timestamp: time.Now(),
	}

	start := time.Now()

	url := fmt.Sprintf("http://%s:%d/api/v1/dual-center/health", peer.Address, peer.Port)
	ctx, cancel := context.WithTimeout(fm.ctx, fm.config.HeartbeatTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req.Header.Set("X-Source-Center", fm.sync.GetLocalCenter().ID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.Latency = time.Since(start)

	if resp.StatusCode == http.StatusOK {
		result.Success = true

		var healthResp struct {
			CenterID   string `json:"center_id"`
			Role       string `json:"role"`
			Status     string `json:"status"`
		}
		if json.NewDecoder(resp.Body).Decode(&healthResp) == nil {
			result.SeqNo = peer.SeqNo
		}
	} else {
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return result
}

// triggerFailover 触发故障切换
func (fm *FailoverManager) triggerFailover(failedPeerID, reason string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	currentState := fm.state.Load().(FailoverState)
	if currentState == FailoverStateFailing {
		return // 已在切换中
	}

	fm.state.Store(FailoverStateFailing)

	startTime := time.Now()

	// 1. 前置延迟
	if fm.config.PreSwitchDelay > 0 {
		select {
		case <-time.After(fm.config.PreSwitchDelay):
		case <-fm.ctx.Done():
			return
		}
	}

	// 2. 仲裁检查
	if fm.config.QuorumRequired {
		// 检查是否能获得仲裁
		peers := fm.sync.GetPeerCenters()
		activePeers := 0
		for _, p := range peers {
			if p.ID != failedPeerID && p.IsActive {
				activePeers++
			}
		}
		totalCenters := len(peers) + 1 // +1 for local
		if 1+activePeers < totalCenters/2+1 {
			// 无法获得仲裁，不执行切换
			fm.state.Store(FailoverStateDegraded)
			fm.recordEvent(FailoverEvent{
				Type:     "fence",
				Reason:   "quorum not reached",
				CenterID: failedPeerID,
				Success:  false,
			})
			return
		}
	}

	// 3. 获取围栏令牌
	if fm.config.FenceEnabled {
		epoch := fm.currentEpoch.Add(1)
		fm.currentEpoch.Store(epoch)

		token := &FenceToken{
			Epoch:     epoch,
			CenterID:  fm.sync.GetLocalCenter().ID,
			Role:      fm.localRole.Load().(CenterRole),
			IssuedAt:  time.Now(),
			ExpiresAt: time.Now().Add(fm.config.FenceTimeout),
		}
		fm.fenceToken.Store(token)
	}

	// 4. 执行角色切换
	oldRole := fm.localRole.Load().(CenterRole)
	var newRole CenterRole

	switch oldRole {
	case CenterRoleSecondary, CenterRoleStandby:
		newRole = CenterRolePrimary
	case CenterRolePrimary, CenterRoleActive:
		// 主中心故障场景：备中心提升为主
		newRole = CenterRolePrimary
	default:
		newRole = CenterRolePrimary
	}

	fm.localRole.Store(newRole)
	fm.isActive.Store(true)

	// 5. 更新本地中心角色
	fm.sync.localCenter.Role = newRole

	// 6. 通知回调
	if fm.onFailover != nil {
		go fm.onFailover(oldRole, newRole, reason)
	}

	// 7. 记录事件
	fm.recordEvent(FailoverEvent{
		Type:      "failover",
		FromRole:  oldRole,
		ToRole:    newRole,
		Reason:    reason,
		CenterID:  fm.sync.GetLocalCenter().ID,
		Timestamp: time.Now(),
		Duration:  time.Since(startTime),
		Success:   true,
	})

	fm.state.Store(FailoverStateNormal)
}

// attemptRecover 尝试恢复
func (fm *FailoverManager) attemptRecover(peerID string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.state.Store(FailoverStateRecovering)

	// 延迟恢复
	if fm.config.RecoverDelay > 0 {
		select {
		case <-time.After(fm.config.RecoverDelay):
		case <-fm.ctx.Done():
			return
		}
	}

	// 数据补偿
	if fm.config.DataRepairOnRecover {
		repair := NewDataRepair(fm.sync)
		repair.Repair(fm.ctx, peerID)
	}

	// 通知回调
	if fm.onRecover != nil {
		go fm.onRecover(peerID)
	}

	// 记录事件
	fm.recordEvent(FailoverEvent{
		Type:      "recover",
		Reason:    "peer recovered",
		CenterID:  peerID,
		Timestamp: time.Now(),
		Success:   true,
	})

	fm.state.Store(FailoverStateNormal)
}

// handlePeerDown 处理对端下线
func (fm *FailoverManager) handlePeerDown(centerID string) {
	// 由心跳循环处理故障切换
}

// handlePeerUp 处理对端上线
func (fm *FailoverManager) handlePeerUp(centerID string) {
	// 由心跳循环处理恢复
}

// ManualFailover 手动触发故障切换
func (fm *FailoverManager) ManualFailover(reason string) error {
	if fm.config.Mode == FailoverModeAuto {
		return fmt.Errorf("manual failover not allowed in auto mode")
	}

	peers := fm.sync.GetPeerCenters()
	for peerID := range peers {
		fm.triggerFailover(peerID, reason)
	}
	return nil
}

// ConfirmFailover 确认故障切换（半自动模式）
func (fm *FailoverManager) ConfirmFailover() error {
	currentState := fm.state.Load().(FailoverState)
	if currentState != FailoverStateDetecting {
		return fmt.Errorf("no pending failover to confirm, current state: %s", currentState)
	}

	peers := fm.sync.GetPeerCenters()
	for peerID := range peers {
		fm.triggerFailover(peerID, "manual confirm")
	}
	return nil
}

// RejectFailover 拒绝故障切换（半自动模式）
func (fm *FailoverManager) RejectFailover() {
	currentState := fm.state.Load().(FailoverState)
	if currentState == FailoverStateDetecting {
		fm.state.Store(FailoverStateNormal)
	}
}

// recordEvent 记录故障切换事件
func (fm *FailoverManager) recordEvent(event FailoverEvent) {
	event.ID = fmt.Sprintf("fo-%d", time.Now().UnixNano())
	event.Timestamp = time.Now()

	fm.historyMutex.Lock()
	fm.history = append(fm.history, event)
	// 保留最近100条
	if len(fm.history) > 100 {
		fm.history = fm.history[len(fm.history)-100:]
	}
	fm.historyMutex.Unlock()
}

// OnFailover 注册故障切换回调
func (fm *FailoverManager) OnFailover(fn func(from, to CenterRole, reason string)) {
	fm.onFailover = fn
}

// OnRecover 注册恢复回调
func (fm *FailoverManager) OnRecover(fn func(centerID string)) {
	fm.onRecover = fn
}

// GetState 获取当前状态
func (fm *FailoverManager) GetState() FailoverState {
	return fm.state.Load().(FailoverState)
}

// GetLocalRole 获取本地角色
func (fm *FailoverManager) GetLocalRole() CenterRole {
	return fm.localRole.Load().(CenterRole)
}

// IsActive 是否为活跃中心
func (fm *FailoverManager) IsActive() bool {
	return fm.isActive.Load()
}

// GetHeartbeatResults 获取心跳结果
func (fm *FailoverManager) GetHeartbeatResults() map[string]*HeartbeatResult {
	fm.hbMutex.RLock()
	defer fm.hbMutex.RUnlock()

	result := make(map[string]*HeartbeatResult, len(fm.heartbeatResults))
	for k, v := range fm.heartbeatResults {
		result[k] = v
	}
	return result
}

// GetHistory 获取切换历史
func (fm *FailoverManager) GetHistory() []FailoverEvent {
	fm.historyMutex.Lock()
	defer fm.historyMutex.Unlock()

	events := make([]FailoverEvent, len(fm.history))
	copy(events, fm.history)
	return events
}

// GetFenceToken 获取当前围栏令牌
func (fm *FailoverManager) GetFenceToken() *FenceToken {
	return fm.fenceToken.Load()
}

// ValidateFenceToken 验证围栏令牌
func (fm *FailoverManager) ValidateFenceToken(token *FenceToken) bool {
	if token == nil {
		return false
	}

	// 检查是否过期
	if time.Now().After(token.ExpiresAt) {
		return false
	}

	// 检查epoch
	current := fm.currentEpoch.Load()
	return token.Epoch <= current
}

// ============================================================
// 无缝切换：连接接管
// ============================================================

// ConnectionTakeover 连接接管器
// 用于故障切换时无缝接管Agent连接
type ConnectionTakeover struct {
	failoverMgr *FailoverManager
	sync        *DualCenterSync

	// 连接表：agentID -> 当前连接的中心
	connections map[string]string // agentID -> centerID
	connMutex   sync.RWMutex

	// 接管状态
	takeoverInProgress atomic.Bool
}

// NewConnectionTakeover 创建连接接管器
func NewConnectionTakeover(fm *FailoverManager, sync *DualCenterSync) *ConnectionTakeover {
	ct := &ConnectionTakeover{
		failoverMgr: fm,
		sync:        sync,
		connections: make(map[string]string),
	}

	// 注册故障切换回调
	fm.OnFailover(func(from, to CenterRole, reason string) {
		ct.takeoverAllConnections(reason)
	})

	return ct
}

// RegisterConnection 注册Agent连接
func (ct *ConnectionTakeover) RegisterConnection(agentID, centerID string) {
	ct.connMutex.Lock()
	defer ct.connMutex.Unlock()
	ct.connections[agentID] = centerID
}

// UnregisterConnection 注销Agent连接
func (ct *ConnectionTakeover) UnregisterConnection(agentID string) {
	ct.connMutex.Lock()
	defer ct.connMutex.Unlock()
	delete(ct.connections, agentID)
}

// GetConnectionCount 获取连接数
func (ct *ConnectionTakeover) GetConnectionCount() int {
	ct.connMutex.RLock()
	defer ct.connMutex.RUnlock()
	return len(ct.connections)
}

// GetConnectionsByCenter 获取指定中心的连接
func (ct *ConnectionTakeover) GetConnectionsByCenter(centerID string) []string {
	ct.connMutex.RLock()
	defer ct.connMutex.RUnlock()

	var agents []string
	for agentID, cid := range ct.connections {
		if cid == centerID {
			agents = append(agents, agentID)
		}
	}
	return agents
}

// takeoverAllConnections 接管所有连接
func (ct *ConnectionTakeover) takeoverAllConnections(reason string) {
	if !ct.takeoverInProgress.CompareAndSwap(false, true) {
		return
	}
	defer ct.takeoverInProgress.Store(false)

	localID := ct.sync.GetLocalCenter().ID

	ct.connMutex.Lock()
	defer ct.connMutex.Unlock()

	takeoverCount := 0
	for agentID, centerID := range ct.connections {
		if centerID != localID {
			// 将连接标记为本地
			ct.connections[agentID] = localID
			takeoverCount++
		}
	}

	// 广播连接变更通知
	ct.broadcastTakeoverNotification(reason, takeoverCount)
}

// broadcastTakeoverNotification 广播接管通知
func (ct *ConnectionTakeover) broadcastTakeoverNotification(reason string, count int) {
	peers := ct.sync.GetPeerCenters()
	for peerID, peer := range peers {
		notification := map[string]interface{}{
			"type":         "connection_takeover",
			"target_center": ct.sync.GetLocalCenter().ID,
			"reason":       reason,
			"count":        count,
			"timestamp":    time.Now(),
		}

		data, _ := json.Marshal(notification)
		url := fmt.Sprintf("http://%s:%d/api/v1/dual-center/notify", peer.Address, peer.Port)

		go func(url string, data []byte) {
			resp, err := http.Post(url, "application/json", bytes.NewReader(data))
			if err != nil {
				return
			}
			resp.Body.Close()
		}(url, data)
	}
}

// ============================================================
// 无缝切换：DNS/虚拟IP切换
// ============================================================

// VIPManager 虚拟IP管理器
type VIPManager struct {
	config    *VIPConfig
	currentIP net.IP
	mu        sync.Mutex
}

// VIPConfig 虚拟IP配置
type VIPConfig struct {
	Enabled    bool     `yaml:"enabled" json:"enabled"`
	VirtualIP  string   `yaml:"virtual_ip" json:"virtual_ip"`
	Interface  string   `yaml:"interface" json:"interface"`
	Netmask    string   `yaml:"netmask" json:"netmask"`
}

// NewVIPManager 创建虚拟IP管理器
func NewVIPManager(cfg *VIPConfig) *VIPManager {
	return &VIPManager{
		config: cfg,
	}
}

// AcquireVIP 获取虚拟IP
func (vm *VIPManager) AcquireVIP() error {
	if !vm.config.Enabled {
		return nil
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	// 使用系统命令添加虚拟IP
	// 实际生产中应使用更可靠的方式（如keepalived、pacemaker等）
	ip := net.ParseIP(vm.config.VirtualIP)
	if ip == nil {
		return fmt.Errorf("invalid virtual IP: %s", vm.config.VirtualIP)
	}

	vm.currentIP = ip
	return nil
}

// ReleaseVIP 释放虚拟IP
func (vm *VIPManager) ReleaseVIP() error {
	if !vm.config.Enabled {
		return nil
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	vm.currentIP = nil
	return nil
}

// HasVIP 是否持有虚拟IP
func (vm *VIPManager) HasVIP() bool {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	return vm.currentIP != nil
}

// GetCurrentVIP 获取当前虚拟IP
func (vm *VIPManager) GetCurrentVIP() string {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	if vm.currentIP == nil {
		return ""
	}
	return vm.currentIP.String()
}

// ============================================================
// 无缝切换：流量重定向
// ============================================================

// TrafficRedirector 流量重定向器
type TrafficRedirector struct {
	failoverMgr *FailoverManager
	vipMgr      *VIPManager
	connTakeover *ConnectionTakeover
}

// NewTrafficRedirector 创建流量重定向器
func NewTrafficRedirector(fm *FailoverManager, vipMgr *VIPManager, ct *ConnectionTakeover) *TrafficRedirector {
	tr := &TrafficRedirector{
		failoverMgr:  fm,
		vipMgr:       vipMgr,
		connTakeover: ct,
	}

	// 注册故障切换回调
	fm.OnFailover(func(from, to CenterRole, reason string) {
		tr.redirectTraffic(reason)
	})

	return tr
}

// redirectTraffic 重定向流量
func (tr *TrafficRedirector) redirectTraffic(reason string) {
	// 1. 获取虚拟IP
	if tr.vipMgr != nil {
		if err := tr.vipMgr.AcquireVIP(); err != nil {
			// 虚拟IP获取失败，仅依赖连接接管
		}
	}

	// 2. 连接接管由ConnectionTakeover自动处理
}

// ReleaseTraffic 释放流量
func (tr *TrafficRedirector) ReleaseTraffic() {
	if tr.vipMgr != nil {
		tr.vipMgr.ReleaseVIP()
	}
}

// ============================================================
// 故障切换状态报告
// ============================================================

// FailoverStatus 故障切换状态报告
type FailoverStatus struct {
	State          FailoverState             `json:"state"`
	LocalRole      CenterRole                `json:"local_role"`
	IsActive       bool                      `json:"is_active"`
	Heartbeats     map[string]*HeartbeatResult `json:"heartbeats"`
	FenceToken     *FenceToken               `json:"fence_token,omitempty"`
	RecentEvents   []FailoverEvent           `json:"recent_events"`
	Connections    int                       `json:"connections"`
	VirtualIP      string                    `json:"virtual_ip,omitempty"`
}

// GetStatus 获取故障切换状态
func (fm *FailoverManager) GetStatus() *FailoverStatus {
	return &FailoverStatus{
		State:        fm.GetState(),
		LocalRole:    fm.GetLocalRole(),
		IsActive:     fm.IsActive(),
		Heartbeats:   fm.GetHeartbeatResults(),
		FenceToken:   fm.GetFenceToken(),
		RecentEvents: fm.GetHistory(),
	}
}

// ============================================================
// bytes 辅助
// ============================================================

var bytesPkg = &bytesHelperPkg{}

type bytesHelperPkg struct{}

func (b *bytesHelperPkg) NewReader(data []byte) *bytesReaderPkg {
	return &bytesReaderPkg{data: data, pos: 0}
}

type bytesReaderPkg struct {
	data []byte
	pos  int
}

func (r *bytesReaderPkg) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
