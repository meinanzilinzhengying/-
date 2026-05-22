/*
 * Cloud Flow Agent - High Availability Cluster
 *
 * 高可用集群架构，支持多节点部署，避免单点故障
 */

package ha

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrNotLeader     = errors.New("not the leader")
	ErrClusterDown   = errors.New("cluster is down")
	ErrNodeNotFound  = errors.New("node not found")
)

// NodeRole 节点角色
type NodeRole int

const (
	RoleFollower NodeRole = iota
	RoleCandidate
	RoleLeader
)

// NodeState 节点状态
type NodeState int

const (
	StateHealthy NodeState = iota
	StateDegraded
	StateUnhealthy
	StateOffline
)

// HAConfig 高可用配置
type HAConfig struct {
	Enabled           bool          // 启用高可用
	NodeID            string        // 当前节点 ID
	ClusterName       string        // 集群名称
	BindAddr          string        // 绑定地址
	BindPort          int           // 绑定端口
	Peers             []string      // 对等节点列表
	
	// 选举配置
	ElectionTimeout   time.Duration // 选举超时
	HeartbeatInterval time.Duration // 心跳间隔
	HeartbeatTimeout  time.Duration // 心跳超时
	
	// 故障转移配置
	FailoverEnabled   bool          // 启用故障转移
	FailoverTimeout   time.Duration // 故障转移超时
	AutoFailover      bool          // 自动故障转移
	
	// 数据同步配置
	SyncEnabled       bool          // 启用数据同步
	SyncInterval      time.Duration // 同步间隔
	SyncTimeout       time.Duration // 同步超时
	
	// 分片配置
	ShardingEnabled   bool          // 启用分片
	ShardCount        int           // 分片数量
	ReplicationFactor int           // 副本因子
}

// DefaultHAConfig 默认配置
func DefaultHAConfig() *HAConfig {
	return &HAConfig{
		Enabled:           true,
		BindPort:          7946,
		ElectionTimeout:   5 * time.Second,
		HeartbeatInterval: 1 * time.Second,
		HeartbeatTimeout:  3 * time.Second,
		FailoverEnabled:   true,
		FailoverTimeout:   10 * time.Second,
		AutoFailover:      true,
		SyncEnabled:       true,
		SyncInterval:      5 * time.Second,
		SyncTimeout:       10 * time.Second,
		ShardingEnabled:   true,
		ShardCount:        16,
		ReplicationFactor: 3,
	}
}

// ClusterNode 集群节点
type ClusterNode struct {
	ID          string    `json:"id"`
	Address     string    `json:"address"`
	Port        int       `json:"port"`
	Role        NodeRole  `json:"role"`
	State       NodeState `json:"state"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	Term        uint64    `json:"term"`
	Version     string    `json:"version"`
	Tags        map[string]string `json:"tags"`
}

// ClusterState 集群状态
type ClusterState struct {
	LeaderID    string                 `json:"leader_id"`
	Term        uint64                 `json:"term"`
	Nodes       map[string]*ClusterNode `json:"nodes"`
	Version     uint64                 `json:"version"`
	LastUpdated time.Time              `json:"last_updated"`
}

// ClusterManager 集群管理器
type ClusterManager struct {
	mu sync.RWMutex

	config *HAConfig
	
	// 当前节点
	self *ClusterNode
	
	// 集群状态
	state *ClusterState
	
	// 角色管理
	roleMu     sync.RWMutex
	role       NodeRole
	leaderID   string
	term       uint64
	
	// 通信
	transport  *ClusterTransport
	
	// 事件通道
	eventCh    chan ClusterEvent
	
	// 任务分发
	taskDistributor *TaskDistributor
	
	// 数据同步
	dataSync   *DataSynchronizer
	
	// 健康检查
	healthChecker *HealthChecker
	
	// 统计
	stats ClusterStats
	
	// 控制
	ctx    context.Context
	cancel context.CancelFunc
}

// ClusterEvent 集群事件
type ClusterEvent struct {
	Type      EventType     `json:"type"`
	NodeID    string        `json:"node_id"`
	Timestamp time.Time     `json:"timestamp"`
	Data      interface{}   `json:"data"`
}

type EventType string

const (
	EventNodeJoined    EventType = "node_joined"
	EventNodeLeft      EventType = "node_left"
	EventLeaderChanged EventType = "leader_changed"
	EventRoleChanged   EventType = "role_changed"
	EventStateChanged  EventType = "state_changed"
	EventFailover      EventType = "failover"
)

// ClusterStats 集群统计
type ClusterStats struct {
	ElectionCount     uint64
	FailoverCount     uint64
	HeartbeatSent     uint64
	HeartbeatReceived uint64
	SyncCount         uint64
	ConflictCount     uint64
}

// ClusterTransport 集群传输层
type ClusterTransport struct {
	bindAddr string
	bindPort int
}

// TaskDistributor 任务分发器
type TaskDistributor struct {
	mu       sync.RWMutex
	tasks    map[string]*Task
	sharding ShardingStrategy
}

// Task 任务
type Task struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Payload   []byte            `json:"payload"`
	NodeID    string            `json:"node_id"`
	Status    TaskStatus        `json:"status"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
)

// ShardingStrategy 分片策略
type ShardingStrategy interface {
	GetShardID(key string) int
	GetNodesForShard(shardID int) []string
}

// ConsistentHash 一致性哈希
type ConsistentHash struct {
	replicas int
	ring     map[uint32]string
	nodes    map[string]bool
}

// DataSynchronizer 数据同步器
type DataSynchronizer struct {
	mu       sync.RWMutex
	syncQueue chan *SyncRequest
}

// SyncRequest 同步请求
type SyncRequest struct {
	Key       string    `json:"key"`
	Value     []byte    `json:"value"`
	Version   uint64    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}

// HealthChecker 健康检查器
type HealthChecker struct {
	mu        sync.RWMutex
	checks    map[string]*HealthCheck
}

// HealthCheck 健康检查
type HealthCheck struct {
	NodeID      string    `json:"node_id"`
	LastCheck   time.Time `json:"last_check"`
	Healthy     bool      `json:"healthy"`
	FailCount   int       `json:"fail_count"`
	SuccessCount int      `json:"success_count"`
}

// NewClusterManager 创建集群管理器
func NewClusterManager(config *HAConfig) (*ClusterManager, error) {
	if config == nil {
		config = DefaultHAConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	self := &ClusterNode{
		ID:      config.NodeID,
		Address: config.BindAddr,
		Port:    config.BindPort,
		Role:    RoleFollower,
		State:   StateHealthy,
		Tags:    make(map[string]string),
	}

	cm := &ClusterManager{
		config: config,
		self:   self,
		state: &ClusterState{
			Nodes: make(map[string]*ClusterNode),
		},
		role:       RoleFollower,
		eventCh:    make(chan ClusterEvent, 100),
		transport:  &ClusterTransport{bindAddr: config.BindAddr, bindPort: config.BindPort},
		taskDistributor: &TaskDistributor{
			tasks: make(map[string]*Task),
		},
		dataSync: &DataSynchronizer{
			syncQueue: make(chan *SyncRequest, 1000),
		},
		healthChecker: &HealthChecker{
			checks: make(map[string]*HealthCheck),
		},
		ctx:    ctx,
		cancel: cancel,
	}

	// 注册自己
	cm.state.Nodes[config.NodeID] = self

	return cm, nil
}

// Start 启动集群管理器
func (cm *ClusterManager) Start() error {
	if !cm.config.Enabled {
		return nil
	}

	// 启动传输层
	if err := cm.startTransport(); err != nil {
		return fmt.Errorf("failed to start transport: %w", err)
	}

	// 加入集群
	if len(cm.config.Peers) > 0 {
		if err := cm.joinCluster(); err != nil {
			return fmt.Errorf("failed to join cluster: %w", err)
		}
	}

	// 启动心跳
	go cm.heartbeatLoop()

	// 启动选举监控
	go cm.electionLoop()

	// 启动健康检查
	go cm.healthCheckLoop()

	// 启动数据同步
	if cm.config.SyncEnabled {
		go cm.syncLoop()
	}

	return nil
}

// Stop 停止集群管理器
func (cm *ClusterManager) Stop() error {
	cm.cancel()
	
	// 通知集群离开
	cm.leaveCluster()
	
	return nil
}

// startTransport 启动传输层
func (cm *ClusterManager) startTransport() error {
	// 实际实现应启动 TCP/UDP 监听
	return nil
}

// joinCluster 加入集群
func (cm *ClusterManager) joinCluster() error {
	// 向已知节点发送加入请求
	for _, peer := range cm.config.Peers {
		if err := cm.sendJoinRequest(peer); err != nil {
			continue
		}
		// 成功加入一个节点即可
		break
	}
	return nil
}

// sendJoinRequest 发送加入请求
func (cm *ClusterManager) sendJoinRequest(peer string) error {
	// 实际实现应发送加入请求到对等节点
	return nil
}

// leaveCluster 离开集群
func (cm *ClusterManager) leaveCluster() {
	// 通知其他节点离开
}

// heartbeatLoop 心跳循环
func (cm *ClusterManager) heartbeatLoop() {
	ticker := time.NewTicker(cm.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.sendHeartbeats()
		case <-cm.ctx.Done():
			return
		}
	}
}

// sendHeartbeats 发送心跳
func (cm *ClusterManager) sendHeartbeats() {
	cm.mu.RLock()
	nodes := make([]*ClusterNode, 0, len(cm.state.Nodes))
	for _, node := range cm.state.Nodes {
		if node.ID != cm.config.NodeID {
			nodes = append(nodes, node)
		}
	}
	cm.mu.RUnlock()

	for _, node := range nodes {
		go cm.sendHeartbeat(node)
	}

	atomic.AddUint64(&cm.stats.HeartbeatSent, uint64(len(nodes)))
}

// sendHeartbeat 发送心跳到单个节点
func (cm *ClusterManager) sendHeartbeat(node *ClusterNode) {
	// 实际实现应发送心跳消息
}

// electionLoop 选举循环
func (cm *ClusterManager) electionLoop() {
	ticker := time.NewTicker(cm.config.ElectionTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.checkElection()
		case <-cm.ctx.Done():
			return
		}
	}
}

// checkElection 检查是否需要发起选举
func (cm *ClusterManager) checkElection() {
	cm.mu.RLock()
	leaderID := cm.state.LeaderID
	cm.mu.RUnlock()

	// 如果没有 Leader，发起选举
	if leaderID == "" {
		cm.startElection()
	}
}

// startElection 发起选举
func (cm *ClusterManager) startElection() {
	cm.roleMu.Lock()
	defer cm.roleMu.Unlock()

	if cm.role != RoleFollower {
		return
	}

	// 转为候选者
	cm.role = RoleCandidate
	cm.term++

	// 发送投票请求
	votes := cm.requestVotes()

	// 获得多数票则成为 Leader
	if votes > len(cm.state.Nodes)/2 {
		cm.becomeLeader()
	} else {
		cm.role = RoleFollower
	}

	atomic.AddUint64(&cm.stats.ElectionCount, 1)
}

// requestVotes 请求投票
func (cm *ClusterManager) requestVotes() int {
	votes := 1 // 自己投自己
	
	cm.mu.RLock()
	nodes := make([]*ClusterNode, 0, len(cm.state.Nodes))
	for _, node := range cm.state.Nodes {
		if node.ID != cm.config.NodeID {
			nodes = append(nodes, node)
		}
	}
	cm.mu.RUnlock()

	for _, node := range nodes {
		if cm.requestVote(node) {
			votes++
		}
	}

	return votes
}

// requestVote 向单个节点请求投票
func (cm *ClusterManager) requestVote(node *ClusterNode) bool {
	// 实际实现应发送投票请求
	return true
}

// becomeLeader 成为 Leader
func (cm *ClusterManager) becomeLeader() {
	cm.role = RoleLeader
	cm.state.LeaderID = cm.config.NodeID
	cm.self.Role = RoleLeader

	// 发送 Leader 变更通知
	cm.broadcastEvent(ClusterEvent{
		Type:      EventLeaderChanged,
		NodeID:    cm.config.NodeID,
		Timestamp: time.Now(),
		Data:      cm.config.NodeID,
	})
}

// stepDown 卸任 Leader
func (cm *ClusterManager) stepDown() {
	cm.roleMu.Lock()
	defer cm.roleMu.Unlock()

	if cm.role == RoleLeader {
		cm.role = RoleFollower
		cm.self.Role = RoleFollower
		cm.state.LeaderID = ""
	}
}

// healthCheckLoop 健康检查循环
func (cm *ClusterManager) healthCheckLoop() {
	ticker := time.NewTicker(cm.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.checkHealth()
		case <-cm.ctx.Done():
			return
		}
	}
}

// checkHealth 检查健康状态
func (cm *ClusterManager) checkHealth() {
	cm.mu.RLock()
	nodes := make([]*ClusterNode, 0, len(cm.state.Nodes))
	for _, node := range cm.state.Nodes {
		if node.ID != cm.config.NodeID {
			nodes = append(nodes, node)
		}
	}
	cm.mu.RUnlock()

	for _, node := range nodes {
		cm.checkNodeHealth(node)
	}
}

// checkNodeHealth 检查单个节点健康
func (cm *ClusterManager) checkNodeHealth(node *ClusterNode) {
	check := cm.healthChecker.checks[node.ID]
	if check == nil {
		check = &HealthCheck{NodeID: node.ID}
		cm.healthChecker.checks[node.ID] = check
	}

	// 检查最后心跳时间
	if time.Since(node.LastHeartbeat) > cm.config.HeartbeatTimeout {
		check.FailCount++
		check.Healthy = false

		// 触发故障转移
		if cm.config.FailoverEnabled && check.FailCount >= 3 {
			cm.handleNodeFailure(node)
		}
	} else {
		check.SuccessCount++
		check.FailCount = 0
		check.Healthy = true
	}

	check.LastCheck = time.Now()
}

// handleNodeFailure 处理节点故障
func (cm *ClusterManager) handleNodeFailure(node *ClusterNode) {
	node.State = StateUnhealthy

	// 广播故障事件
	cm.broadcastEvent(ClusterEvent{
		Type:      EventStateChanged,
		NodeID:    node.ID,
		Timestamp: time.Now(),
		Data:      StateUnhealthy,
	})

	// 如果故障的是 Leader，发起选举
	if node.ID == cm.state.LeaderID {
		cm.startElection()
	}

	atomic.AddUint64(&cm.stats.FailoverCount, 1)
}

// syncLoop 同步循环
func (cm *ClusterManager) syncLoop() {
	ticker := time.NewTicker(cm.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case req := <-cm.dataSync.syncQueue:
			cm.syncData(req)
		case <-ticker.C:
			cm.syncAllData()
		case <-cm.ctx.Done():
			return
		}
	}
}

// syncData 同步数据
func (cm *ClusterManager) syncData(req *SyncRequest) {
	if cm.role != RoleLeader {
		return
	}

	// 同步到所有 Follower
	cm.mu.RLock()
	nodes := make([]*ClusterNode, 0, len(cm.state.Nodes))
	for _, node := range cm.state.Nodes {
		if node.Role == RoleFollower && node.State == StateHealthy {
			nodes = append(nodes, node)
		}
	}
	cm.mu.RUnlock()

	for _, node := range nodes {
		go cm.sendSyncData(node, req)
	}

	atomic.AddUint64(&cm.stats.SyncCount, 1)
}

// sendSyncData 发送同步数据
func (cm *ClusterManager) sendSyncData(node *ClusterNode, req *SyncRequest) {
	// 实际实现应发送同步请求
}

// syncAllData 同步所有数据
func (cm *ClusterManager) syncAllData() {
	// 全量同步
}

// broadcastEvent 广播事件
func (cm *ClusterManager) broadcastEvent(event ClusterEvent) {
	select {
	case cm.eventCh <- event:
	default:
	}
}

// GetEventChannel 获取事件通道
func (cm *ClusterManager) GetEventChannel() <-chan ClusterEvent {
	return cm.eventCh
}

// IsLeader 检查是否为 Leader
func (cm *ClusterManager) IsLeader() bool {
	cm.roleMu.RLock()
	defer cm.roleMu.RUnlock()
	return cm.role == RoleLeader
}

// GetLeaderID 获取 Leader ID
func (cm *ClusterManager) GetLeaderID() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.state.LeaderID
}

// GetClusterState 获取集群状态
func (cm *ClusterManager) GetClusterState() *ClusterState {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	state := &ClusterState{
		LeaderID:    cm.state.LeaderID,
		Term:        cm.state.Term,
		Version:     cm.state.Version,
		LastUpdated: cm.state.LastUpdated,
		Nodes:       make(map[string]*ClusterNode),
	}

	for id, node := range cm.state.Nodes {
		copy := *node
		state.Nodes[id] = &copy
	}

	return state
}

// GetNodeCount 获取节点数量
func (cm *ClusterManager) GetNodeCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.state.Nodes)
}

// GetHealthyNodeCount 获取健康节点数量
func (cm *ClusterManager) GetHealthyNodeCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	count := 0
	for _, node := range cm.state.Nodes {
		if node.State == StateHealthy {
			count++
		}
	}
	return count
}

// GetStats 获取统计
func (cm *ClusterManager) GetStats() ClusterStats {
	return ClusterStats{
		ElectionCount:     atomic.LoadUint64(&cm.stats.ElectionCount),
		FailoverCount:     atomic.LoadUint64(&cm.stats.FailoverCount),
		HeartbeatSent:     atomic.LoadUint64(&cm.stats.HeartbeatSent),
		HeartbeatReceived: atomic.LoadUint64(&cm.stats.HeartbeatReceived),
		SyncCount:         atomic.LoadUint64(&cm.stats.SyncCount),
		ConflictCount:     atomic.LoadUint64(&cm.stats.ConflictCount),
	}
}

// DistributeTask 分发任务
func (cm *ClusterManager) DistributeTask(task *Task) error {
	if !cm.IsLeader() {
		return ErrNotLeader
	}

	// 使用一致性哈希选择节点
	nodeID := cm.selectNodeForTask(task.ID)
	if nodeID == "" {
		return ErrNodeNotFound
	}

	task.NodeID = nodeID
	task.Status = TaskPending
	task.CreatedAt = time.Now()

	cm.taskDistributor.mu.Lock()
	cm.taskDistributor.tasks[task.ID] = task
	cm.taskDistributor.mu.Unlock()

	// 发送任务到目标节点
	return cm.sendTaskToNode(nodeID, task)
}

// selectNodeForTask 为任务选择节点
func (cm *ClusterManager) selectNodeForNode(taskID string) string {
	// 使用一致性哈希
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, node := range cm.state.Nodes {
		if node.State == StateHealthy {
			return node.ID
		}
	}

	return ""
}

// sendTaskToNode 发送任务到节点
func (cm *ClusterManager) sendTaskToNode(nodeID string, task *Task) error {
	// 实际实现应发送任务到目标节点
	return nil
}

// GetShardForKey 获取键对应的分片
func (cm *ClusterManager) GetShardForKey(key string) int {
	if !cm.config.ShardingEnabled {
		return 0
	}

	// 使用一致性哈希
	hash := hashKey(key)
	return int(hash % uint32(cm.config.ShardCount))
}

// hashKey 计算键的哈希值
func hashKey(key string) uint32 {
	var hash uint32 = 0
	for i := 0; i < len(key); i++ {
		hash = hash*31 + uint32(key[i])
	}
	return hash
}
