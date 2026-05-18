// Package cluster 提供集群管理功能，实现服务发现、主备选举和故障转移
package cluster

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"

	"cloud-flow-center/pkg/logger"
)

// Manager 集群管理器
type Manager struct {
	client        *clientv3.Client
	session       *concurrency.Session
	leaderKey     string
	leaseTTL      int64
	nodeID        string
	nodeAddr      string
	isLeader      atomic.Bool
	leaderCh      chan bool
	wg            sync.WaitGroup
	stopCh        chan struct{}
	logger        *logger.Logger
	storagePath   string
	dataSyncCh    chan struct{}
	stopOnce      sync.Once
}

// Config 集群配置
type Config struct {
	EtcdEndpoints []string
	LeaseTTL      int64
	NodeID        string
	NodeAddr      string
	StoragePath   string
}

// NewManager 创建集群管理器
func NewManager(cfg Config, log *logger.Logger) (*Manager, error) {
	// 初始化 etcd 客户端
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.EtcdEndpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("连接 etcd 失败: %w", err)
	}

	// 创建会话
	session, err := concurrency.NewSession(client, concurrency.WithTTL(int(cfg.LeaseTTL)))
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("创建会话失败: %w", err)
	}

	// 生成节点 ID
	nodeID := cfg.NodeID
	if nodeID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		nodeID = fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano())
	}

	// 校验 nodeID 只包含合法字符 [a-zA-Z0-9_-]，防止 etcd key 注入
	for _, c := range nodeID {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return nil, fmt.Errorf("nodeID 包含非法字符 '%c'，只允许 [a-zA-Z0-9_-]", c)
		}
	}

	// 生成节点地址
	nodeAddr := cfg.NodeAddr
	if nodeAddr == "" {
		addrs, err := net.InterfaceAddrs()
		if err == nil && len(addrs) > 0 {
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						nodeAddr = ipnet.IP.String()
						break
					}
				}
			}
		}
		if nodeAddr == "" {
			nodeAddr = "127.0.0.1"
		}
	}

	// 确保存储路径存在
	if err := os.MkdirAll(cfg.StoragePath, 0755); err != nil {
		client.Close()
		session.Close()
		return nil, fmt.Errorf("创建存储路径失败: %w", err)
	}

	return &Manager{
		client:      client,
		session:     session,
		leaderKey:   "/cloud-flow/leader",
		leaseTTL:    cfg.LeaseTTL,
		nodeID:      nodeID,
		nodeAddr:    nodeAddr,
		isLeader:    atomic.Bool{},
		leaderCh:    make(chan bool, 1),
		stopCh:      make(chan struct{}),
		logger:      log,
		storagePath: cfg.StoragePath,
		dataSyncCh:  make(chan struct{}, 1),
	}, nil
}

// DefaultCheckInterval 默认检查间隔常量。
// 告警检查循环和领导者选举心跳均依赖此间隔。
// 可通过配置 center.alerting.check_interval 覆盖告警检查间隔。
const DefaultCheckInterval = 10 * time.Second

// Start 启动集群管理器
func (m *Manager) Start() {
	m.wg.Add(2)
	go m.leaderElectionLoop()
	go m.serviceRegistrationLoop()
	m.logger.Info("集群管理器已启动")
}

// Stop 停止集群管理器
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
	m.wg.Wait()
	m.session.Close()
	m.client.Close()
	m.logger.Info("集群管理器已停止")
}

// IsLeader 检查当前节点是否为领导者
func (m *Manager) IsLeader() bool {
	return m.isLeader.Load()
}

// LeaderCh 返回领导者状态变化通道
func (m *Manager) LeaderCh() <-chan bool {
	return m.leaderCh
}

// DataSyncCh 返回数据同步通道
func (m *Manager) DataSyncCh() <-chan struct{} {
	return m.dataSyncCh
}

// leaderElectionLoop 领导者选举循环
func (m *Manager) leaderElectionLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.attemptLeaderElection()
		case <-m.stopCh:
			return
		}
	}
}

// attemptLeaderElection 尝试领导者选举
func (m *Manager) attemptLeaderElection() {
	election := concurrency.NewElection(m.session, m.leaderKey)

	// 尝试成为领导者
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 在 stopCh 关闭时取消 context，确保 Campaign 能及时退出
	go func() {
		select {
		case <-m.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	err := election.Campaign(ctx, m.nodeID)
	if err != nil {
		m.logger.Warnf("竞选领导者失败: %v", err)
		return
	}

	// 成为领导者
	if !m.isLeader.Load() {
		m.isLeader.Store(true)
		m.leaderCh <- true
		m.logger.Info("当前节点成为领导者")

		// 通知数据同步
		select {
		case m.dataSyncCh <- struct{}{}:
		default:
		}
	}

	// 监控领导者状态
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	leaderCh := election.Observe(ctx)

	// 等待领导者变化
	for {
		select {
		case resp, ok := <-leaderCh:
			if !ok {
				// 通道关闭
				m.logger.Warn("领导者监控通道关闭")
				election.Resign(context.Background())
				return
			}
			// 领导者变化，检查是否还是当前节点
			if len(resp.Kvs) == 0 {
				// 没有领导者，重新竞选
				continue
			}
			if string(resp.Kvs[0].Value) != m.nodeID {
				// 失去领导者地位
				m.isLeader.Store(false)
				m.leaderCh <- false
				m.logger.Info("当前节点失去领导者地位")
				election.Resign(context.Background())
				return
			}
		case <-m.stopCh:
			election.Resign(context.Background())
			return
		}
	}
}

// serviceRegistrationLoop 服务注册循环
func (m *Manager) serviceRegistrationLoop() {
	defer m.wg.Done()

	serviceKey := fmt.Sprintf("/cloud-flow/services/%s", m.nodeID)
	serviceValue := m.nodeAddr

	// 初始注册
	m.registerService(serviceKey, serviceValue)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.registerService(serviceKey, serviceValue)
		case <-m.stopCh:
			// 注销服务
			m.client.Delete(context.Background(), serviceKey)
			return
		}
	}
}

// registerService 注册服务
func (m *Manager) registerService(key, value string) {
	_, err := m.client.Put(context.Background(), key, value, clientv3.WithLease(m.session.Lease()))
	if err != nil {
		m.logger.Warnf("注册服务失败: %v", err)
	} else {
		m.logger.Debugf("服务注册成功: %s=%s", key, value)
	}
}

// GetServices 获取所有服务节点
func (m *Manager) GetServices() ([]string, error) {
	resp, err := m.client.Get(context.Background(), "/cloud-flow/services/", clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("获取服务列表失败: %w", err)
	}

	var services []string
	for _, kv := range resp.Kvs {
		services = append(services, string(kv.Value))
	}

	return services, nil
}

// GetLeader 获取当前领导者
func (m *Manager) GetLeader() (string, error) {
	resp, err := m.client.Get(context.Background(), m.leaderKey)
	if err != nil {
		return "", fmt.Errorf("获取领导者失败: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return "", nil
	}

	return string(resp.Kvs[0].Value), nil
}

// GetStoragePath 获取存储路径
func (m *Manager) GetStoragePath() string {
	if m.isLeader.Load() {
		return filepath.Join(m.storagePath, "leader")
	}
	return filepath.Join(m.storagePath, "follower")
}