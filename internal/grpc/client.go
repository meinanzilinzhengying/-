/*
 * Cloud Flow Agent - gRPC Client
 *
 * gRPC 客户端实现，用于与边缘服务和中心控制通信
 */

package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

// ClientConfig gRPC 客户端配置
type ClientConfig struct {
	// 服务器地址
	ServerAddress string

	// 连接配置
	DialTimeout       time.Duration
	KeepaliveTime     time.Duration
	KeepaliveTimeout  time.Duration
	MaxRecvMsgSize    int
	MaxSendMsgSize    int

	// TLS 配置
	EnableTLS         bool
	TLSCertFile       string
	TLSKeyFile        string
	TLSCAFile         string
	InsecureSkipVerify bool

	// 认证配置
	AuthToken         string
	AuthInterceptor   bool

	// 重试配置
	MaxRetries        int
	RetryBackoff      time.Duration
}

// DefaultClientConfig 默认配置
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		ServerAddress:      "localhost:50051",
		DialTimeout:        10 * time.Second,
		KeepaliveTime:      30 * time.Second,
		KeepaliveTimeout:   10 * time.Second,
		MaxRecvMsgSize:     64 * 1024 * 1024, // 64MB
		MaxSendMsgSize:     64 * 1024 * 1024, // 64MB
		EnableTLS:          false,
		MaxRetries:         3,
		RetryBackoff:       1 * time.Second,
	}
}

// GRPCClient gRPC 客户端
type GRPCClient struct {
	mu sync.RWMutex

	config *ClientConfig
	conn   *grpc.ClientConn
	
	// 服务客户端
	// agentClient  CloudFlowAgentClient
	// edgeClient   EdgeServiceClient
	// centerClient ControlCenterClient

	// 连接状态
	connected  bool
	lastError  error
	
	// 上下文控制
	ctx    context.Context
	cancel context.CancelFunc
}

// NewGRPCClient 创建 gRPC 客户端
func NewGRPCClient(config *ClientConfig) (*GRPCClient, error) {
	if config == nil {
		config = DefaultClientConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &GRPCClient{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}

	// 建立连接
	if err := client.connect(); err != nil {
		cancel()
		return nil, err
	}

	return client, nil
}

// connect 建立连接
func (c *GRPCClient) connect() error {
	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(c.config.MaxRecvMsgSize),
			grpc.MaxCallSendMsgSize(c.config.MaxSendMsgSize),
		),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                c.config.KeepaliveTime,
			Timeout:             c.config.KeepaliveTimeout,
			PermitWithoutStream: true,
		}),
	}

	// TLS 配置
	if c.config.EnableTLS {
		creds, err := c.loadTLSCredentials()
		if err != nil {
			return fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// 认证拦截器
	if c.config.AuthInterceptor && c.config.AuthToken != "" {
		opts = append(opts, grpc.WithUnaryInterceptor(c.authInterceptor))
		opts = append(opts, grpc.WithStreamInterceptor(c.authStreamInterceptor))
	}

	// 建立连接
	ctx, cancel := context.WithTimeout(c.ctx, c.config.DialTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, c.config.ServerAddress, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", c.config.ServerAddress, err)
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.mu.Unlock()

	return nil
}

// loadTLSCredentials 加载 TLS 凭证
func (c *GRPCClient) loadTLSCredentials() (credentials.TransportCredentials, error) {
	if c.config.TLSCertFile != "" && c.config.TLSKeyFile != "" {
		// 双向 TLS
		cert, err := tls.LoadX509KeyPair(c.config.TLSCertFile, c.config.TLSKeyFile)
		if err != nil {
			return nil, err
		}

		config := &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: c.config.InsecureSkipVerify,
		}

		return credentials.NewTLS(config), nil
	}

	// 单向 TLS
	config := &tls.Config{
		InsecureSkipVerify: c.config.InsecureSkipVerify,
	}

	return credentials.NewTLS(config), nil
}

// authInterceptor 认证拦截器（Unary）
func (c *GRPCClient) authInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	md := metadata.Pairs("authorization", "Bearer "+c.config.AuthToken)
	ctx = metadata.NewOutgoingContext(ctx, md)
	return invoker(ctx, method, req, reply, cc, opts...)
}

// authStreamInterceptor 认证拦截器（Stream）
func (c *GRPCClient) authStreamInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	md := metadata.Pairs("authorization", "Bearer "+c.config.AuthToken)
	ctx = metadata.NewOutgoingContext(ctx, md)
	return streamer(ctx, desc, cc, method, opts...)
}

// GetConnection 获取连接
func (c *GRPCClient) GetConnection() *grpc.ClientConn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

// IsConnected 检查连接状态
func (c *GRPCClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Reconnect 重新连接
func (c *GRPCClient) Reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 关闭旧连接
	if c.conn != nil {
		c.conn.Close()
	}

	c.connected = false

	// 建立新连接
	return c.connect()
}

// Close 关闭连接
func (c *GRPCClient) Close() error {
	c.cancel()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// WithRetry 带重试的调用
func (c *GRPCClient) WithRetry(ctx context.Context, fn func() error) error {
	var lastErr error

	for i := 0; i <= c.config.MaxRetries; i++ {
		if err := fn(); err != nil {
			lastErr = err
			
			// 检查是否需要重试
			if i < c.config.MaxRetries {
				select {
				case <-time.After(c.config.RetryBackoff * time.Duration(i+1)):
					continue
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		} else {
			return nil
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// WaitForReady 等待连接就绪
func (c *GRPCClient) WaitForReady(timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if c.IsConnected() {
				return true
			}
		}
	}
}
