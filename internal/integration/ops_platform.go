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

// OpsPlatformClient 一体化运维平台客户端
type OpsPlatformClient struct {
	config     *OpsPlatformConfig
	httpClient *http.Client
}

// OpsPlatformConfig 一体化运维平台配置
type OpsPlatformConfig struct {
	Enabled         bool   `yaml:"enabled" json:"enabled"`
	Endpoint        string `yaml:"endpoint" json:"endpoint"`
	AppKey          string `yaml:"app_key" json:"app_key"`
	AppSecret       string `yaml:"app_secret" json:"app_secret"`
	Timeout         int    `yaml:"timeout" json:"timeout"`
	SyncInterval    int    `yaml:"sync_interval" json:"sync_interval"`
	EventWebhook    string `yaml:"event_webhook" json:"event_webhook"`
	MetricWebhook   string `yaml:"metric_webhook" json:"metric_webhook"`
	AlertWebhook    string `yaml:"alert_webhook" json:"alert_webhook"`
}

// DefaultOpsPlatformConfig 默认配置
func DefaultOpsPlatformConfig() *OpsPlatformConfig {
	return &OpsPlatformConfig{
		Enabled:       false,
		Endpoint:      "http://ops-platform.example.com/api",
		Timeout:       30,
		SyncInterval:  60,
		EventWebhook:  "/v1/events",
		MetricWebhook: "/v1/metrics",
		AlertWebhook:  "/v1/alerts",
	}
}

// NewOpsPlatformClient 创建一体化运维平台客户端
func NewOpsPlatformClient(config *OpsPlatformConfig) *OpsPlatformClient {
	if config == nil {
		config = DefaultOpsPlatformConfig()
	}

	return &OpsPlatformClient{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
	}
}

// Start 启动一体化运维平台对接
func (c *OpsPlatformClient) Start(ctx context.Context) error {
	if !c.config.Enabled {
		logger.Info("Ops Platform integration is disabled")
		return nil
	}

	logger.Info("Starting Ops Platform integration")
	go c.syncLoop(ctx)
	logger.Info("Ops Platform integration started")
	return nil
}

// Stop 停止一体化运维平台对接
func (c *OpsPlatformClient) Stop() error {
	logger.Info("Stopping Ops Platform integration")
	return nil
}

// syncLoop 同步循环
func (c *OpsPlatformClient) syncLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(c.config.SyncInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.syncMetrics(); err != nil {
				logger.Errorf("Failed to sync metrics to Ops Platform: %v", err)
			}
		}
	}
}

// syncMetrics 同步指标数据
func (c *OpsPlatformClient) syncMetrics() error {
	// 获取本地指标并推送到运维平台
	metrics := c.collectLocalMetrics()
	return c.pushMetrics(metrics)
}

// collectLocalMetrics 收集本地指标
func (c *OpsPlatformClient) collectLocalMetrics() []*OpsMetric {
	// 这里应该从本地存储获取指标
	return []*OpsMetric{}
}

// pushMetrics 推送指标到运维平台
func (c *OpsPlatformClient) pushMetrics(metrics []*OpsMetric) error {
	if len(metrics) == 0 {
		return nil
	}

	url := fmt.Sprintf("%s%s", c.config.Endpoint, c.config.MetricWebhook)
	body, _ := json.Marshal(metrics)

	_, err := c.doRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to push metrics: %w", err)
	}

	logger.Debugf("Pushed %d metrics to Ops Platform", len(metrics))
	return nil
}

// SendEvent 发送事件到运维平台
func (c *OpsPlatformClient) SendEvent(event *OpsEvent) error {
	url := fmt.Sprintf("%s%s", c.config.Endpoint, c.config.EventWebhook)
	body, _ := json.Marshal(event)

	_, err := c.doRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to send event: %w", err)
	}

	logger.Debugf("Sent event to Ops Platform: %s", event.EventID)
	return nil
}

// SendAlert 发送告警到运维平台
func (c *OpsPlatformClient) SendAlert(alert *OpsAlert) error {
	url := fmt.Sprintf("%s%s", c.config.Endpoint, c.config.AlertWebhook)
	body, _ := json.Marshal(alert)

	_, err := c.doRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to send alert: %w", err)
	}

	logger.Debugf("Sent alert to Ops Platform: %s", alert.AlertID)
	return nil
}

// doRequest 执行HTTP请求
func (c *OpsPlatformClient) doRequest(method, url string, body []byte) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-App-Key", c.config.AppKey)
	req.Header.Set("X-App-Secret", c.config.AppSecret)
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

// OpsMetric 运维平台指标
type OpsMetric struct {
	MetricName string            `json:"metric_name"`
	Value      float64           `json:"value"`
	Timestamp  int64             `json:"timestamp"`
	Labels     map[string]string `json:"labels"`
	Unit       string            `json:"unit"`
}

// OpsEvent 运维平台事件
type OpsEvent struct {
	EventID     string            `json:"event_id"`
	EventType   string            `json:"event_type"`
	Severity    string            `json:"severity"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Source      string            `json:"source"`
	Labels      map[string]string `json:"labels"`
	Timestamp   int64             `json:"timestamp"`
}

// OpsAlert 运维平台告警
type OpsAlert struct {
	AlertID     string            `json:"alert_id"`
	AlertName   string            `json:"alert_name"`
	Severity    string            `json:"severity"`
	Status      string            `json:"status"`
	Summary     string            `json:"summary"`
	Description string            `json:"description"`
	Source      string            `json:"source"`
	Labels      map[string]string `json:"labels"`
	StartsAt    int64             `json:"starts_at"`
	EndsAt      int64             `json:"ends_at,omitempty"`
}

// CreateIncident 创建故障单
func (c *OpsPlatformClient) CreateIncident(incident *OpsIncident) error {
	url := fmt.Sprintf("%s/v1/incidents", c.config.Endpoint)
	body, _ := json.Marshal(incident)

	_, err := c.doRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to create incident: %w", err)
	}

	logger.Infof("Created incident: %s", incident.IncidentID)
	return nil
}

// UpdateIncident 更新故障单
func (c *OpsPlatformClient) UpdateIncident(incidentID string, incident *OpsIncident) error {
	url := fmt.Sprintf("%s/v1/incidents/%s", c.config.Endpoint, incidentID)
	body, _ := json.Marshal(incident)

	_, err := c.doRequest("PUT", url, body)
	if err != nil {
		return fmt.Errorf("failed to update incident: %w", err)
	}

	return nil
}

// OpsIncident 运维平台故障单
type OpsIncident struct {
	IncidentID   string   `json:"incident_id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Severity     string   `json:"severity"`
	Status       string   `json:"status"`
	Assignee     string   `json:"assignee"`
	RelatedAlerts []string `json:"related_alerts"`
	CreateTime   int64    `json:"create_time"`
	UpdateTime   int64    `json:"update_time"`
}

// GetDashboard 获取运维平台仪表盘数据
func (c *OpsPlatformClient) GetDashboard(dashboardID string) (*OpsDashboard, error) {
	url := fmt.Sprintf("%s/v1/dashboards/%s", c.config.Endpoint, dashboardID)

	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var dashboard OpsDashboard
	if err := json.Unmarshal(resp, &dashboard); err != nil {
		return nil, fmt.Errorf("failed to parse dashboard: %w", err)
	}

	return &dashboard, nil
}

// OpsDashboard 运维平台仪表盘
type OpsDashboard struct {
	DashboardID string      `json:"dashboard_id"`
	Name        string      `json:"name"`
	Panels      []OpsPanel  `json:"panels"`
}

// OpsPanel 仪表盘面板
type OpsPanel struct {
	PanelID string `json:"panel_id"`
	Title   string `json:"title"`
	Type    string `json:"type"`
	Query   string `json:"query"`
}
