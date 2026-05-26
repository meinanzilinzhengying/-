// Package election 预留 Edge Leader Election 能力
// 当前为接口定义和空实现，后续可基于 Redis 或 etcd 实现
package election

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// State Leader 状态
type State string

const (
	StateFollower State = "follower" // 跟随者
	StateCandidate State = "candidate" // 候选者
	StateLeader    State = "leader"    // 领导者
)

// LeaderElection Leader 选举接口
type LeaderElection interface {
	// Campaign 发起选举
	Campaign(ctx context.Context) error
	// Resign 辞去 Leader
	Resign(ctx context.Context) error
	// IsLeader 是否是 Leader
	IsLeader() bool
	// GetState 获取当前状态
	GetState() State
	// GetLeaderID 获取当前 Leader ID
	GetLeaderID() string
	// OnLeaderChange 注册 Leader 变更回调
	OnLeaderChange(callback func(leaderID string))
	// Close 关闭选举
	Close() error
}

// NoopElection 空实现（单节点模式或禁用选举时使用）
type NoopElection struct {
	mu        sync.RWMutex
	nodeID    string
	state     State
	callbacks []func(string)
	stopCh    chan struct{}
	stopped   bool
}

// NewNoopElection 创建空选举实现
func NewNoopElection(nodeID string) *NoopElection {
	return &NoopElection{
		nodeID: nodeID,
		state:  StateLeader, // 单节点模式下自己是 Leader
		stopCh: make(chan struct{}),
	}
}

func (n *NoopElection) Campaign(ctx context.Context) error {
	n.mu.Lock()
	n.state = StateLeader
	n.mu.Unlock()
	n.notifyCallbacks(n.nodeID)
	return nil
}

func (n *NoopElection) Resign(ctx context.Context) error {
	n.mu.Lock()
	n.state = StateFollower
	n.mu.Unlock()
	return nil
}

func (n *NoopElection) IsLeader() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.state == StateLeader
}

func (n *NoopElection) GetState() State {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.state
}

func (n *NoopElection) GetLeaderID() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.state == StateLeader {
		return n.nodeID
	}
	return ""
}

func (n *NoopElection) OnLeaderChange(callback func(string)) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.callbacks = append(n.callbacks, callback)
}

func (n *NoopElection) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.stopped {
		n.stopped = true
		close(n.stopCh)
	}
	return nil
}

func (n *NoopElection) notifyCallbacks(leaderID string) {
	for _, cb := range n.callbacks {
		go cb(leaderID)
	}
}

// RedisElection 基于 Redis 的 Leader 选举（预留实现）
type RedisElection struct {
	mu        sync.RWMutex
	nodeID    string
	leaderID  string
	state     State
	ttl       time.Duration
	callbacks []func(string)
	stopCh    chan struct{}
	stopped   bool
}

// NewRedisElection 创建基于 Redis 的选举
// TODO: 实现基于 Redis SETNX + TTL 的 Leader 选举
func NewRedisElection(nodeID string, ttl time.Duration) *RedisElection {
	return &RedisElection{
		nodeID:   nodeID,
		ttl:      ttl,
		stopCh:   make(chan struct{}),
		callbacks: make([]func(string), 0),
	}
}

func (r *RedisElection) Campaign(ctx context.Context) error {
	r.mu.Lock()
	r.state = StateCandidate
	r.mu.Unlock()

	// TODO: 实现 Redis SETNX 竞选
	// 1. SET leader_key {nodeID} NX EX {ttl}
	// 2. 如果成功，成为 Leader
	// 3. 如果失败，获取当前 Leader ID
	// 4. 启动续约协程

	return fmt.Errorf("RedisElection.Campaign 尚未实现")
}

func (r *RedisElection) Resign(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = StateFollower
	r.leaderID = ""
	// TODO: DEL leader_key
	return nil
}

func (r *RedisElection) IsLeader() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state == StateLeader
}

func (r *RedisElection) GetState() State {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

func (r *RedisElection) GetLeaderID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.leaderID
}

func (r *RedisElection) OnLeaderChange(callback func(string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callbacks = append(r.callbacks, callback)
}

func (r *RedisElection) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.stopped {
		r.stopped = true
		close(r.stopCh)
	}
	return nil
}
