//go:build linux

// Package grpc 提供 gRPC 客户端功能
package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/meinanzilinzhengying/cloud-flow-agent/pkg/api"
	"github.com/meinanzilinzhengying/cloud-flow-agent/pkg/models"
)

// Client gRPC 客户端
type Client struct {
	config    *models.EdgeConfig
	conn      *grpc.ClientConn
	client    api.AgentServiceClient
	mu        sync.RWMutex
	connected bool

	// 重连相关
	retryCount int
	stopChan   chan struct{}
}

// NewClient 创建 gRPC 客户端
func NewClient(config *models.EdgeConfig) *Client {
	return &Client{
		config:   config,
		stopChan: make(chan struct{}),
	}
}

// Connect 连接到 Edge 服务
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	address := fmt.Sprintf("%s:%d", c.config.Address, c.config.Port)

	var opts []grpc.DialOption

	// 配置 TLS
	if c.config.TLSEnabled {
		creds, err := c.loadTLSCredentials()
		if err != nil {
			return fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// 配置 keepalive
	kaParams := keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             time.Duration(c.config.Timeout) * time.Second,
		PermitWithoutStream: true,
	}
	opts = append(opts, grpc.WithKeepaliveParams(kaParams))

	// 配置重试
	opts = append(opts, grpc.WithDefaultCallOptions(
		grpc.MaxRetry(3),
	))

	// 建立连接
	conn, err := grpc.DialContext(ctx, address, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	c.conn = conn
	c.client = api.NewAgentServiceClient(conn)
	c.connected = true

	return nil
}

// loadTLSCredentials 加载 TLS 凭证
func (c *Client) loadTLSCredentials() (credentials.TransportCredentials, error) {
	// 加载 CA 证书
	caCert, err := os.ReadFile(c.config.CAFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCert) {
		return nil, errors.New("failed to append CA certificate")
	}

	// 加载客户端证书
	cert, err := tls.LoadX509KeyPair(c.config.CertFile, c.config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	config := &tls.Config{
		RootCAs:      caPool,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(config), nil
}

// Close 关闭连接
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	close(c.stopChan)

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsConnected 检查是否已连接
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Heartbeat 发送心跳
func (c *Client) Heartbeat(ctx context.Context, req *api.HeartbeatRequest) (*api.HeartbeatResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.client == nil {
		return nil, errors.New("not connected")
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	return c.client.Heartbeat(ctx, req)
}

// ReportNetworkFlow 上报网络流量
func (c *Client) ReportNetworkFlow(ctx context.Context, flows []*api.NetworkFlow) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.client == nil {
		return errors.New("not connected")
	}

	stream, err := c.client.ReportNetworkFlow(ctx)
	if err != nil {
		return err
	}

	// 批量发送
	batchSize := 100
	for i := 0; i < len(flows); i += batchSize {
		end := i + batchSize
		if end > len(flows) {
			end = len(flows)
		}

		req := &api.NetworkFlowRequest{
			Flows: flows[i:end],
		}

		if err := stream.Send(req); err != nil {
			return err
		}
	}

	_, err = stream.CloseAndRecv()
	return err
}

// ReportSystemMetrics 上报系统指标
func (c *Client) ReportSystemMetrics(ctx context.Context, metrics []*api.SystemMetric) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.client == nil {
		return errors.New("not connected")
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	req := &api.SystemMetricsRequest{
		Metrics: metrics,
	}

	_, err := c.client.ReportSystemMetrics(ctx, req)
	return err
}

// ReportProcessEvents 上报进程事件
func (c *Client) ReportProcessEvents(ctx context.Context, events []*api.ProcessEvent) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.client == nil {
		return errors.New("not connected")
	}

	stream, err := c.client.ReportProcessEvents(ctx)
	if err != nil {
		return err
	}

	batchSize := 100
	for i := 0; i < len(events); i += batchSize {
		end := i + batchSize
		if end > len(events) {
			end = len(events)
		}

		req := &api.ProcessEventRequest{
			Events: events[i:end],
		}

		if err := stream.Send(req); err != nil {
			return err
		}
	}

	_, err = stream.CloseAndRecv()
	return err
}

// GetConfig 获取配置
func (c *Client) GetConfig(ctx context.Context, agentID, currentHash string) ([]byte, string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.client == nil {
		return nil, "", errors.New("not connected")
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	req := &api.GetConfigRequest{
		AgentId:          agentID,
		CurrentConfigHash: currentHash,
	}

	resp, err := c.client.GetConfig(ctx, req)
	if err != nil {
		return nil, "", err
	}

	return resp.ConfigData, resp.ConfigHash, nil
}

// UpdateConfig 更新配置
func (c *Client) UpdateConfig(ctx context.Context, agentID string, configData []byte) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.client == nil {
		return "", errors.New("not connected")
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	req := &api.UpdateConfigRequest{
		AgentId:    agentID,
		ConfigData: configData,
	}

	resp, err := c.client.UpdateConfig(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.NewConfigHash, nil
}

// StreamNetworkFlow 流式上报网络流量
func (c *Client) StreamNetworkFlow(ctx context.Context, flowChan <-chan *api.NetworkFlow) error {
	c.mu.RLock()
	if c.client == nil {
		c.mu.RUnlock()
		return errors.New("not connected")
	}
	c.mu.RUnlock()

	stream, err := c.client.ReportNetworkFlow(ctx)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var batch []*api.NetworkFlow

	for {
		select {
		case <-ctx.Done():
			// 发送剩余数据
			if len(batch) > 0 {
				req := &api.NetworkFlowRequest{Flows: batch}
				if err := stream.Send(req); err != nil && err != io.EOF {
					return err
				}
			}
			_, _ = stream.CloseAndRecv()
			return ctx.Err()

		case flow, ok := <-flowChan:
			if !ok {
				// 通道关闭，发送剩余数据
				if len(batch) > 0 {
					req := &api.NetworkFlowRequest{Flows: batch}
					_ = stream.Send(req)
				}
				_, _ = stream.CloseAndRecv()
				return nil
			}

			batch = append(batch, flow)
			if len(batch) >= 100 {
				req := &api.NetworkFlowRequest{Flows: batch}
				if err := stream.Send(req); err != nil {
					return err
				}
				batch = batch[:0]
			}

		case <-ticker.C:
			// 定期发送
			if len(batch) > 0 {
				req := &api.NetworkFlowRequest{Flows: batch}
				if err := stream.Send(req); err != nil {
					return err
				}
				batch = batch[:0]
			}
		}
	}
}

// Reconnect 重新连接
func (c *Client) Reconnect(ctx context.Context) error {
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.connected = false
	c.mu.Unlock()

	return c.Connect(ctx)
}

// StartHeartbeatLoop 启动心跳循环
func (c *Client) StartHeartbeatLoop(ctx context.Context, agentID string, getStatus func() *api.AgentStatus) <-chan error {
	errChan := make(chan error, 1)

	go func() {
		defer close(errChan)

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stopChan:
				return
			case <-ticker.C:
				status := getStatus()
				req := &api.HeartbeatRequest{
					AgentId: agentID,
					Status:  status,
				}

				_, err := c.Heartbeat(ctx, req)
				if err != nil {
					// 尝试重连
					c.retryCount++
					if c.retryCount > c.config.RetryMax {
						errChan <- fmt.Errorf("max retries exceeded: %w", err)
						return
					}

					time.Sleep(time.Duration(c.config.RetryDelay) * time.Second)
					if err := c.Reconnect(ctx); err != nil {
						errChan <- err
						return
					}
				} else {
					c.retryCount = 0
				}
			}
		}
	}()

	return errChan
}
