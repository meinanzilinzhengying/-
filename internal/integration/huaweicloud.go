/*
 * Copyright (c) 2025 Yunlong Liao. All rights reserved.
 */

package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// HuaweiCloudClient 华为云客户端
type HuaweiCloudClient struct {
	config     *HuaweiCloudConfig
	httpClient *http.Client
	authToken  string
	tokenExpiry time.Time
	mu         sync.RWMutex
}

// HuaweiCloudConfig 华为云配置
type HuaweiCloudConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	Region        string `yaml:"region" json:"region"`
	AccessKey     string `yaml:"access_key" json:"access_key"`
	SecretKey     string `yaml:"secret_key" json:"secret_key"`
	ProjectID     string `yaml:"project_id" json:"project_id"`
	Endpoint      string `yaml:"endpoint" json:"endpoint"`
	IAMEndpoint   string `yaml:"iam_endpoint" json:"iam_endpoint"`
	Timeout       int    `yaml:"timeout" json:"timeout"`
	RetryCount    int    `yaml:"retry_count" json:"retry_count"`
	SyncInterval  int    `yaml:"sync_interval" json:"sync_interval"`
}

// DefaultHuaweiCloudConfig 默认配置
func DefaultHuaweiCloudConfig() *HuaweiCloudConfig {
	return &HuaweiCloudConfig{
		Enabled:      false,
		Region:       "cn-north-4",
		Endpoint:     "https://ces.cn-north-4.myhuaweicloud.com",
		IAMEndpoint:  "https://iam.cn-north-4.myhuaweicloud.com",
		Timeout:      30,
		RetryCount:   3,
		SyncInterval: 60,
	}
}

// NewHuaweiCloudClient 创建华为云客户端
func NewHuaweiCloudClient(config *HuaweiCloudConfig) *HuaweiCloudClient {
	if config == nil {
		config = DefaultHuaweiCloudConfig()
	}

	return &HuaweiCloudClient{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
	}
}

// Start 启动华为云对接
func (c *HuaweiCloudClient) Start(ctx context.Context) error {
	if !c.config.Enabled {
		logger.Info("Huawei Cloud integration is disabled")
		return nil
	}

	logger.Info("Starting Huawei Cloud integration")

	// 获取初始认证Token
	if err := c.authenticate(); err != nil {
		return fmt.Errorf("failed to authenticate with Huawei Cloud: %w", err)
	}

	// 启动同步循环
	go c.syncLoop(ctx)

	logger.Info("Huawei Cloud integration started")
	return nil
}

// Stop 停止华为云对接
func (c *HuaweiCloudClient) Stop() error {
	logger.Info("Stopping Huawei Cloud integration")
	return nil
}

// authenticate 获取认证Token
func (c *HuaweiCloudClient) authenticate() error {
	authReq := map[string]interface{}{
		"auth": map[string]interface{}{
			"identity": map[string]interface{}{
				"methods": []string{"password"},
				"password": map[string]interface{}{
					"user": map[string]interface{}{
						"name":     c.config.AccessKey,
						"password": c.config.SecretKey,
						"domain": map[string]interface{}{
							"name": c.config.AccessKey,
						},
					},
				},
			},
			"scope": map[string]interface{}{
				"project": map[string]interface{}{
					"id": c.config.ProjectID,
				},
			},
		},
	}

	body, _ := json.Marshal(authReq)
	url := fmt.Sprintf("%s/v3/auth/tokens", c.config.IAMEndpoint)

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("authentication request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("authentication failed: %s, body: %s", resp.Status, string(body))
	}

	// 获取Token
	token := resp.Header.Get("X-Subject-Token")
	if token == "" {
		return fmt.Errorf("no token in response")
	}

	c.mu.Lock()
	c.authToken = token
	c.tokenExpiry = time.Now().Add(24 * time.Hour) // Token有效期24小时
	c.mu.Unlock()

	logger.Info("Successfully authenticated with Huawei Cloud")
	return nil
}

// getToken 获取有效Token
func (c *HuaweiCloudClient) getToken() (string, error) {
	c.mu.RLock()
	token := c.authToken
	expiry := c.tokenExpiry
	c.mu.RUnlock()

	// Token即将过期，重新认证
	if time.Now().After(expiry.Add(-5 * time.Minute)) {
		if err := c.authenticate(); err != nil {
			return "", err
		}
		c.mu.RLock()
		token = c.authToken
		c.mu.RUnlock()
	}

	return token, nil
}

// syncLoop 同步循环
func (c *HuaweiCloudClient) syncLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(c.config.SyncInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.syncMetrics(); err != nil {
				logger.Errorf("Failed to sync metrics from Huawei Cloud: %v", err)
			}
			if err := c.syncAlarms(); err != nil {
				logger.Errorf("Failed to sync alarms from Huawei Cloud: %v", err)
			}
		}
	}
}

// syncMetrics 同步指标数据
func (c *HuaweiCloudClient) syncMetrics() error {
	// 获取ECS指标
	metrics, err := c.listECSMetrics()
	if err != nil {
		return fmt.Errorf("failed to list ECS metrics: %w", err)
	}

	for _, metric := range metrics {
		data, err := c.getMetricData(metric)
		if err != nil {
			logger.Warnf("Failed to get metric data for %s: %v", metric.MetricName, err)
			continue
		}
		// 处理指标数据
		c.processMetricData(data)
	}

	return nil
}

// listECSMetrics 列出ECS指标
func (c *HuaweiCloudClient) listECSMetrics() ([]*HuaweiMetric, error) {
	url := fmt.Sprintf("%s/V1.0/%s/metrics", c.config.Endpoint, c.config.ProjectID)

	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Metrics []*HuaweiMetric `json:"metrics"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse metrics response: %w", err)
	}

	return result.Metrics, nil
}

// getMetricData 获取指标数据
func (c *HuaweiCloudClient) getMetricData(metric *HuaweiMetric) (*HuaweiMetricData, error) {
	url := fmt.Sprintf("%s/V1.0/%s/metric-data", c.config.Endpoint, c.config.ProjectID)

	// 构建查询参数
	now := time.Now()
	from := now.Add(-5 * time.Minute)

	query := map[string]interface{}{
		"namespace":  metric.Namespace,
		"metric_name": metric.MetricName,
		"dimensions": metric.Dimensions,
		"from":       from.UnixMilli(),
		"to":         now.UnixMilli(),
		"period":     300,
		"filter":     "average",
	}

	body, _ := json.Marshal(query)
	resp, err := c.doRequest("POST", url, body)
	if err != nil {
		return nil, err
	}

	var data HuaweiMetricData
	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, fmt.Errorf("failed to parse metric data: %w", err)
	}

	return &data, nil
}

// syncAlarms 同步告警数据
func (c *HuaweiCloudClient) syncAlarms() error {
	alarms, err := c.listAlarms()
	if err != nil {
		return fmt.Errorf("failed to list alarms: %w", err)
	}

	for _, alarm := range alarms {
		c.processAlarm(alarm)
	}

	return nil
}

// listAlarms 列出告警
func (c *HuaweiCloudClient) listAlarms() ([]*HuaweiAlarm, error) {
	url := fmt.Sprintf("%s/V1.0/%s/alarms", c.config.Endpoint, c.config.ProjectID)

	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Alarms []*HuaweiAlarm `json:"alarms"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse alarms response: %w", err)
	}

	return result.Alarms, nil
}

// doRequest 执行HTTP请求
func (c *HuaweiCloudClient) doRequest(method, url string, body []byte) ([]byte, error) {
	token, err := c.getToken()
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Auth-Token", token)
	req.Header.Set("Content-Type", "application/json")

	// 重试机制
	var resp *http.Response
	for i := 0; i <= c.config.RetryCount; i++ {
		resp, err = c.httpClient.Do(req)
		if err == nil && resp.StatusCode < 500 {
			break
		}
		if i < c.config.RetryCount {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("request failed after retries: %w", err)
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

// processMetricData 处理指标数据
func (c *HuaweiCloudClient) processMetricData(data *HuaweiMetricData) {
	// 转换为内部格式并存储
	logger.Debugf("Processing metric data: %+v", data)
}

// processAlarm 处理告警
func (c *HuaweiCloudClient) processAlarm(alarm *HuaweiAlarm) {
	// 转换为内部告警格式
	logger.Debugf("Processing alarm: %s - %s", alarm.AlarmName, alarm.AlarmState)
}

// HuaweiMetric 华为云指标
type HuaweiMetric struct {
	Namespace   string            `json:"namespace"`
	MetricName  string            `json:"metric_name"`
	Dimensions  []Dimension       `json:"dimensions"`
	Unit        string            `json:"unit"`
}

// Dimension 维度
type Dimension struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HuaweiMetricData 指标数据
type HuaweiMetricData struct {
	MetricName string      `json:"metric_name"`
	Datapoints []Datapoint `json:"datapoints"`
}

// Datapoint 数据点
type Datapoint struct {
	Average   float64 `json:"average"`
	Timestamp int64   `json:"timestamp"`
	Unit      string  `json:"unit"`
}

// HuaweiAlarm 华为云告警
type HuaweiAlarm struct {
	AlarmID     string `json:"alarm_id"`
	AlarmName   string `json:"alarm_name"`
	AlarmState  string `json:"alarm_state"`
	Severity    string `json:"severity"`
	Namespace   string `json:"namespace"`
	MetricName  string `json:"metric_name"`
	Description string `json:"description"`
	CreateTime  string `json:"create_time"`
	UpdateTime  string `json:"update_time"`
}

// PushMetrics 推送指标到华为云
func (c *HuaweiCloudClient) PushMetrics(metrics []*HuaweiMetricData) error {
	url := fmt.Sprintf("%s/V1.0/%s/metric-data", c.config.Endpoint, c.config.ProjectID)

	body, _ := json.Marshal(metrics)
	_, err := c.doRequest("POST", url, body)
	return err
}

// CreateAlarm 创建告警规则
func (c *HuaweiCloudClient) CreateAlarm(alarm *HuaweiAlarm) error {
	url := fmt.Sprintf("%s/V1.0/%s/alarms", c.config.Endpoint, c.config.ProjectID)

	body, _ := json.Marshal(alarm)
	_, err := c.doRequest("POST", url, body)
	return err
}

// DeleteAlarm 删除告警规则
func (c *HuaweiCloudClient) DeleteAlarm(alarmID string) error {
	url := fmt.Sprintf("%s/V1.0/%s/alarms/%s", c.config.Endpoint, c.config.ProjectID, alarmID)
	_, err := c.doRequest("DELETE", url, nil)
	return err
}
