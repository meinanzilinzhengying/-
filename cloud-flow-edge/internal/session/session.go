// Package session 管理 Agent 与 Edge 的会话状态
// 使用本地内存缓存 + Redis 持久化，支持 Edge 重启后 Agent 会话恢复
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Status 会话状态
type Status string

const (
	StatusRegistering Status = "registering" // 注册中
	StatusOnline      Status = "online"      // 在线
	StatusIdle        Status = "idle"        // 空闲（心跳超时但未达到清理阈值）
	StatusOffline     Status = "offline"     // 离线
)

// Session Agent 会话
type Session struct {
	SessionID     string            `json:"session_id"`      // 会话唯一标识
	ProbeID       string            `json:"probe_id"`        // Agent ID
	AssignedEdge  string            `json:"assigned_edge"`   // 归属 Edge ID
	ClientIP      string            `json:"client_ip"`       // 客户端 IP
	Hostname      string            `json:"hostname"`        // 主机名
	Version       string            `json:"version"`         // Agent 版本
	Region        string            `json:"region"`          // 区域
	Zone          string            `json:"zone"`            // 可用区
	Cluster       string            `json:"cluster"`         // 集群
	Status        Status            `json:"status"`          // 会话状态
	Metadata      map[string]string `json:"metadata"`       // Agent 元数据
	CreatedAt     time.Time         `json:"created_at"`      // 创建时间
	LastHeartbeat time.Time         `json:"last_heartbeat"`  // 最后心跳时间
	HeartbeatCount uint64           `json:"heartbeat_count"` // 心跳计数
}

// Store 会话存储接口
type Store interface {
	// CreateSession 创建新会话
	CreateSession(session *Session) error
	// GetSession 获取会话
	GetSession(probeID string) (*Session, bool)
	// UpdateHeartbeat 更新心跳
	UpdateHeartbeat(probeID string) error
	// UpdateStatus 更新状态
	UpdateStatus(probeID string, status Status) error
	// UpdateMetadata 更新元数据
	UpdateMetadata(probeID string, metadata map[string]string) error
	// DeleteSession 删除会话
	DeleteSession(probeID string) error
	// ListSessions 列出所有会话
	ListSessions() []*Session
	// ListOnlineSessions 列出在线会话
	ListOnlineSessions() []*Session
	// GetSessionCount 获取在线会话数
	GetSessionCount() int
	// CleanupExpired 清理过期会话
	CleanupExpired(timeout time.Duration) int
	// Close 关闭存储
	Close() error
}

// RedisStore Redis 会话存储（用于多 Edge 节点共享会话状态）
type RedisStore struct {
	// 预留 Redis 客户端
	// 当前使用内存存储，后续可替换为 Redis 实现
	local *MemoryStore
}

// NewRedisStore 创建 Redis 会话存储
// TODO: 实现真正的 Redis 存储
func NewRedisStore() *RedisStore {
	return &RedisStore{
		local: NewMemoryStore(),
	}
}

func (r *RedisStore) CreateSession(session *Session) error  { return r.local.CreateSession(session) }
func (r *RedisStore) GetSession(probeID string) (*Session, bool) { return r.local.GetSession(probeID) }
func (r *RedisStore) UpdateHeartbeat(probeID string) error  { return r.local.UpdateHeartbeat(probeID) }
func (r *RedisStore) UpdateStatus(probeID string, status Status) error { return r.local.UpdateStatus(probeID, status) }
func (r *RedisStore) UpdateMetadata(probeID string, metadata map[string]string) error { return r.local.UpdateMetadata(probeID, metadata) }
func (r *RedisStore) DeleteSession(probeID string) error    { return r.local.DeleteSession(probeID) }
func (r *RedisStore) ListSessions() []*Session              { return r.local.ListSessions() }
func (r *RedisStore) ListOnlineSessions() []*Session        { return r.local.ListOnlineSessions() }
func (r *RedisStore) GetSessionCount() int                  { return r.local.GetSessionCount() }
func (r *RedisStore) CleanupExpired(timeout time.Duration) int { return r.local.CleanupExpired(timeout) }
func (r *RedisStore) Close() error                          { return r.local.Close() }

// MemoryStore 内存会话存储
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session // key: ProbeID
	stopCh   chan struct{}
	stopped  bool
	stopMu   sync.Mutex
}

// NewMemoryStore 创建内存会话存储
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]*Session),
		stopCh:   make(chan struct{}),
	}
}

// CreateSession 创建新会话
func (m *MemoryStore) CreateSession(session *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session.SessionID == "" {
		session.SessionID = generateSessionID()
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}
	session.LastHeartbeat = time.Now()
	session.Status = StatusOnline
	if session.Metadata == nil {
		session.Metadata = make(map[string]string)
	}

	m.sessions[session.ProbeID] = session
	return nil
}

// GetSession 获取会话
func (m *MemoryStore) GetSession(probeID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[probeID]
	if !ok {
		return nil, false
	}
	// 返回副本，避免外部修改
	cp := *s
	cp.Metadata = make(map[string]string, len(s.Metadata))
	for k, v := range s.Metadata {
		cp.Metadata[k] = v
	}
	return &cp, true
}

// UpdateHeartbeat 更新心跳
func (m *MemoryStore) UpdateHeartbeat(probeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[probeID]
	if !ok {
		return fmt.Errorf("会话不存在: %s", probeID)
	}
	s.LastHeartbeat = time.Now()
	s.HeartbeatCount++
	s.Status = StatusOnline
	return nil
}

// UpdateStatus 更新状态
func (m *MemoryStore) UpdateStatus(probeID string, status Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[probeID]
	if !ok {
		return fmt.Errorf("会话不存在: %s", probeID)
	}
	s.Status = status
	return nil
}

// UpdateMetadata 更新元数据
func (m *MemoryStore) UpdateMetadata(probeID string, metadata map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[probeID]
	if !ok {
		return fmt.Errorf("会话不存在: %s", probeID)
	}
	for k, v := range metadata {
		s.Metadata[k] = v
	}
	return nil
}

// DeleteSession 删除会话
func (m *MemoryStore) DeleteSession(probeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, probeID)
	return nil
}

// ListSessions 列出所有会话
func (m *MemoryStore) ListSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		cp := *s
		result = append(result, &cp)
	}
	return result
}

// ListOnlineSessions 列出在线会话
func (m *MemoryStore) ListOnlineSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Session, 0)
	for _, s := range m.sessions {
		if s.Status == StatusOnline {
			cp := *s
			result = append(result, &cp)
		}
	}
	return result
}

// GetSessionCount 获取在线会话数
func (m *MemoryStore) GetSessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, s := range m.sessions {
		if s.Status == StatusOnline {
			count++
		}
	}
	return count
}

// CleanupExpired 清理过期会话
func (m *MemoryStore) CleanupExpired(timeout time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	removed := 0
	for id, s := range m.sessions {
		if now.Sub(s.LastHeartbeat) > timeout {
			s.Status = StatusOffline
			delete(m.sessions, id)
			removed++
		}
	}
	return removed
}

// StartCleanup 启动定期清理
func (m *MemoryStore) StartCleanup(ctx context.Context, interval, timeout time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				removed := m.CleanupExpired(timeout)
				if removed > 0 {
					// 可选: 记录日志
					_ = removed
				}
			case <-ctx.Done():
				return
			case <-m.stopCh:
				return
			}
		}
	}()
}

// Close 关闭存储
func (m *MemoryStore) Close() error {
	m.stopMu.Lock()
	defer m.stopMu.Unlock()
	if !m.stopped {
		m.stopped = true
		close(m.stopCh)
	}
	return nil
}

// generateSessionID 生成会话 ID
func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
