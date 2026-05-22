/*
 * Copyright (c) 2025 Yunlong Liao. All rights reserved.
 */

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// CMDBClient CMDB客户端
type CMDBClient struct {
	config     *CMDBConfig
	httpClient *http.Client
}

// CMDBConfig CMDB配置
type CMDBConfig struct {
	Enabled      bool   `yaml:"enabled" json:"enabled"`
	Endpoint     string `yaml:"endpoint" json:"endpoint"`
	APIKey       string `yaml:"api_key" json:"api_key"`
	APISecret    string `yaml:"api_secret" json:"api_secret"`
	Timeout      int    `yaml:"timeout" json:"timeout"`
	SyncInterval int    `yaml:"sync_interval" json:"sync_interval"`
	ModelMapping map[string]string `yaml:"model_mapping" json:"model_mapping"`
}

// DefaultCMDBConfig 默认配置
func DefaultCMDBConfig() *CMDBConfig {
	return &CMDBConfig{
		Enabled:      false,
		Endpoint:     "http://cmdb.example.com/api",
		Timeout:      30,
		SyncInterval: 300,
		ModelMapping: map[string]string{
			"server":    "host",
			"database":  "db_instance",
			"network":   "network_device",
			"container": "container",
		},
	}
}

// NewCMDBClient 创建CMDB客户端
func NewCMDBClient(config *CMDBConfig) *CMDBClient {
	if config == nil {
		config = DefaultCMDBConfig()
	}

	return &CMDBClient{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
	}
}

// Start 启动CMDB对接
func (c *CMDBClient) Start(ctx context.Context) error {
	if !c.config.Enabled {
		logger.Info("CMDB integration is disabled")
		return nil
	}

	logger.Info("Starting CMDB integration")
	go c.syncLoop(ctx)
	logger.Info("CMDB integration started")
	return nil
}

// Stop 停止CMDB对接
func (c *CMDBClient) Stop() error {
	logger.Info("Stopping CMDB integration")
	return nil
}

// syncLoop 同步循环
func (c *CMDBClient) syncLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(c.config.SyncInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.syncAll(); err != nil {
				logger.Errorf("Failed to sync with CMDB: %v", err)
			}
		}
	}
}

// syncAll 同步所有数据
func (c *CMDBClient) syncAll() error {
	// 同步服务器
	if err := c.syncServers(); err != nil {
		logger.Errorf("Failed to sync servers: %v", err)
	}

	// 同步数据库
	if err := c.syncDatabases(); err != nil {
		logger.Errorf("Failed to sync databases: %v", err)
	}

	// 同步网络设备
	if err := c.syncNetworkDevices(); err != nil {
		logger.Errorf("Failed to sync network devices: %v", err)
	}

	// 同步容器
	if err := c.syncContainers(); err != nil {
		logger.Errorf("Failed to sync containers: %v", err)
	}

	return nil
}

// syncServers 同步服务器
func (c *CMDBClient) syncServers() error {
	servers, err := c.getServers()
	if err != nil {
		return err
	}

	for _, server := range servers {
		c.processServer(server)
	}

	return nil
}

// getServers 获取服务器列表
func (c *CMDBClient) getServers() ([]*CMDBServer, error) {
	url := fmt.Sprintf("%s/v1/servers", c.config.Endpoint)
	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []*CMDBServer `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse servers response: %w", err)
	}

	return result.Data, nil
}

// syncDatabases 同步数据库
func (c *CMDBClient) syncDatabases() error {
	databases, err := c.getDatabases()
	if err != nil {
		return err
	}

	for _, db := range databases {
		c.processDatabase(db)
	}

	return nil
}

// getDatabases 获取数据库列表
func (c *CMDBClient) getDatabases() ([]*CMDBDatabase, error) {
	url := fmt.Sprintf("%s/v1/databases", c.config.Endpoint)
	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []*CMDBDatabase `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse databases response: %w", err)
	}

	return result.Data, nil
}

// syncNetworkDevices 同步网络设备
func (c *CMDBClient) syncNetworkDevices() error {
	devices, err := c.getNetworkDevices()
	if err != nil {
		return err
	}

	for _, device := range devices {
		c.processNetworkDevice(device)
	}

	return nil
}

// getNetworkDevices 获取网络设备列表
func (c *CMDBClient) getNetworkDevices() ([]*CMDBNetworkDevice, error) {
	url := fmt.Sprintf("%s/v1/network-devices", c.config.Endpoint)
	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []*CMDBNetworkDevice `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse network devices response: %w", err)
	}

	return result.Data, nil
}

// syncContainers 同步容器
func (c *CMDBClient) syncContainers() error {
	containers, err := c.getContainers()
	if err != nil {
		return err
	}

	for _, container := range containers {
		c.processContainer(container)
	}

	return nil
}

// getContainers 获取容器列表
func (c *CMDBClient) getContainers() ([]*CMDBContainer, error) {
	url := fmt.Sprintf("%s/v1/containers", c.config.Endpoint)
	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []*CMDBContainer `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse containers response: %w", err)
	}

	return result.Data, nil
}

// doRequest 执行HTTP请求
func (c *CMDBClient) doRequest(method, url string, body []byte) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-Key", c.config.APIKey)
	req.Header.Set("X-API-Secret", c.config.APISecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed: %s, body: %s", resp.Status, string(respBody))
	}

	return respBody, nil
}

// processServer 处理服务器数据
func (c *CMDBClient) processServer(server *CMDBServer) {
	logger.Debugf("Processing server: %s (%s)", server.Hostname, server.IP)
}

// processDatabase 处理数据库数据
func (c *CMDBClient) processDatabase(db *CMDBDatabase) {
	logger.Debugf("Processing database: %s (%s)", db.Name, db.Type)
}

// processNetworkDevice 处理网络设备数据
func (c *CMDBClient) processNetworkDevice(device *CMDBNetworkDevice) {
	logger.Debugf("Processing network device: %s (%s)", device.Name, device.Type)
}

// processContainer 处理容器数据
func (c *CMDBClient) processContainer(container *CMDBContainer) {
	logger.Debugf("Processing container: %s (%s)", container.Name, container.Image)
}

// CMDBServer CMDB服务器
type CMDBServer struct {
	ID          string            `json:"id"`
	Hostname    string            `json:"hostname"`
	IP          string            `json:"ip"`
	OS          string            `json:"os"`
	CPU         int               `json:"cpu"`
	Memory      int64             `json:"memory"`
	Disk        int64             `json:"disk"`
	Status      string            `json:"status"`
	Tags        map[string]string `json:"tags"`
	CreateTime  time.Time         `json:"create_time"`
	UpdateTime  time.Time         `json:"update_time"`
}

// CMDBDatabase CMDB数据库
type CMDBDatabase struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Version    string            `json:"version"`
	Host       string            `json:"host"`
	Port       int               `json:"port"`
	Status     string            `json:"status"`
	Tags       map[string]string `json:"tags"`
	CreateTime time.Time         `json:"create_time"`
	UpdateTime time.Time         `json:"update_time"`
}

// CMDBNetworkDevice CMDB网络设备
type CMDBNetworkDevice struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	IP         string            `json:"ip"`
	Location   string            `json:"location"`
	Status     string            `json:"status"`
	Tags       map[string]string `json:"tags"`
	CreateTime time.Time         `json:"create_time"`
	UpdateTime time.Time         `json:"update_time"`
}

// CMDBContainer CMDB容器
type CMDBContainer struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Image      string            `json:"image"`
	Namespace  string            `json:"namespace"`
	Pod        string            `json:"pod"`
	Node       string            `json:"node"`
	Status     string            `json:"status"`
	Tags       map[string]string `json:"tags"`
	CreateTime time.Time         `json:"create_time"`
	UpdateTime time.Time         `json:"update_time"`
}

// CreateServer 创建服务器
func (c *CMDBClient) CreateServer(server *CMDBServer) error {
	url := fmt.Sprintf("%s/v1/servers", c.config.Endpoint)
	body, _ := json.Marshal(server)
	_, err := c.doRequest("POST", url, body)
	return err
}

// UpdateServer 更新服务器
func (c *CMDBClient) UpdateServer(serverID string, server *CMDBServer) error {
	url := fmt.Sprintf("%s/v1/servers/%s", c.config.Endpoint, serverID)
	body, _ := json.Marshal(server)
	_, err := c.doRequest("PUT", url, body)
	return err
}

// DeleteServer 删除服务器
func (c *CMDBClient) DeleteServer(serverID string) error {
	url := fmt.Sprintf("%s/v1/servers/%s", c.config.Endpoint, serverID)
	_, err := c.doRequest("DELETE", url, nil)
	return err
}

// SearchCI 搜索配置项
func (c *CMDBClient) SearchCI(model string, query map[string]interface{}) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/v1/search", c.config.Endpoint)

	searchReq := map[string]interface{}{
		"model": model,
		"query": query,
	}

	body, _ := json.Marshal(searchReq)
	resp, err := c.doRequest("POST", url, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	return result.Data, nil
}
