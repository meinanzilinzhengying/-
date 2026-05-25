//go:build linux

// Package alert 提供告警管理功能
// 本文件实现告警通知器，支持Kafka推送和API推送
package alert

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"cloud-flow-agent/pkg/logger"
	"github.com/IBM/sarama"
)

// ==================== NotifierConfig 通知器配置 ====================

// NotifierConfig 通知器配置
type NotifierConfig struct {
	// Kafka配置
	Kafka KafkaNotifierConfig `yaml:"kafka" json:"kafka"`
	// API配置
	API APINotifierConfig `yaml:"api" json:"api"`
	// Webhook配置
	Webhook WebhookNotifierConfig `yaml:"webhook" json:"webhook"`
}

// KafkaNotifierConfig Kafka通知器配置
type KafkaNotifierConfig struct {
	Enabled   bool     `yaml:"enabled" json:"enabled"`
	Brokers   []string `yaml:"brokers" json:"brokers"`
	Topic     string   `yaml:"topic" json:"topic"`
	Username  string   `yaml:"username" json:"username"`
	Password  string   `yaml:"password" json:"password"`
	TLS       bool     `yaml:"tls" json:"tls"`
	SASL      bool     `yaml:"sasl" json:"sasl"`
	Partition int32    `yaml:"partition" json:"partition"`
	// 重试配置
	MaxRetries int           `yaml:"max_retries" json:"max_retries"`
	RetryDelay time.Duration `yaml:"retry_delay" json:"retry_delay"`
}

// APINotifierConfig API通知器配置
type APINotifierConfig struct {
	Enabled    bool              `yaml:"enabled" json:"enabled"`
	Endpoint   string            `yaml:"endpoint" json:"endpoint"`
	Method     string            `yaml:"method" json:"method"`
	Headers    map[string]string `yaml:"headers" json:"headers"`
	Timeout    time.Duration     `yaml:"timeout" json:"timeout"`
	RetryCount int               `yaml:"retry_count" json:"retry_count"`
	// 认证配置
	AuthType   string `yaml:"auth_type" json:"auth_type"` // none, basic, bearer
	Username   string `yaml:"username" json:"username"`
	Password   string `yaml:"password" json:"password"`
	Token      string `yaml:"token" json:"token"`
}

// WebhookNotifierConfig Webhook通知器配置
type WebhookNotifierConfig struct {
	Enabled    bool              `yaml:"enabled" json:"enabled"`
	URL        string            `yaml:"url" json:"url"`
	Method     string            `yaml:"method" json:"method"`
	Headers    map[string]string `yaml:"headers" json:"headers"`
	Timeout    time.Duration     `yaml:"timeout" json:"timeout"`
	RetryCount int               `yaml:"retry_count" json:"retry_count"`
}

// DefaultNotifierConfig 默认通知器配置
func DefaultNotifierConfig() NotifierConfig {
	return NotifierConfig{
		Kafka: KafkaNotifierConfig{
			Enabled:    false,
			Brokers:    []string{"localhost:9092"},
			Topic:      "alerts",
			TLS:        false,
			SASL:       false,
			Partition:  0,
			MaxRetries: 3,
			RetryDelay: 2 * time.Second,
		},
		API: APINotifierConfig{
			Enabled:    false,
			Method:     "POST",
			Timeout:    10 * time.Second,
			RetryCount: 3,
			Headers:    map[string]string{"Content-Type": "application/json"},
		},
		Webhook: WebhookNotifierConfig{
			Enabled:    false,
			Method:     "POST",
			Timeout:    10 * time.Second,
			RetryCount: 3,
			Headers:    map[string]string{"Content-Type": "application/json"},
		},
	}
}

// ==================== AlertMessage 告警消息格式 ====================

// AlertMessage 告警消息（用于Kafka/API推送）
type AlertMessage struct {
	// 基础信息
	ID          string `json:"id"`
	RuleID      string `json:"rule_id"`
	RuleName    string `json:"rule_name"`
	Level       string `json:"level"`   // critical, warning, info
	State       string `json:"state"`   // firing, resolved
	Status      string `json:"status"`  // 兼容字段：firing, resolved
	Fingerprint string `json:"fingerprint"`

	// 指标信息
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`

	// 标签和注释
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`

	// 时间信息
	FiredAt    time.Time     `json:"fired_at"`
	ResolvedAt *time.Time    `json:"resolved_at,omitempty"`
	Duration   time.Duration `json:"duration"`

	// 来源信息
	Source    string `json:"source"`
	AgentID   string `json:"agent_id"`
	AgentHost string `json:"agent_host"`

	// 额外信息
	Description string `json:"description"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// ToJSON 转换为JSON
func (m *AlertMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// ==================== KafkaNotifier Kafka通知器 ====================

// KafkaNotifier Kafka通知器
type KafkaNotifier struct {
	config   KafkaNotifierConfig
	producer sarama.SyncProducer
	log      *logger.Logger
	mu       sync.RWMutex
	ready    bool
}

// NewKafkaNotifier 创建Kafka通知器
func NewKafkaNotifier(config KafkaNotifierConfig, log *logger.Logger) (*KafkaNotifier, error) {
	if !config.Enabled {
		return &KafkaNotifier{config: config, log: log, ready: false}, nil
	}

	n := &KafkaNotifier{
		config: config,
		log:    log,
	}

	if err := n.connect(); err != nil {
		return nil, fmt.Errorf("连接Kafka失败: %w", err)
	}

	n.ready = true
	return n, nil
}

// connect 连接Kafka
func (n *KafkaNotifier) connect() error {
	cfg := sarama.NewConfig()
	cfg.Producer.RequiredAcks = sarama.WaitForLocal
	cfg.Producer.Retry.Max = n.config.MaxRetries
	cfg.Producer.Return.Successes = true

	// 配置TLS
	if n.config.TLS {
		cfg.Net.TLS.Enable = true
		cfg.Net.TLS.Config = &tls.Config{
			InsecureSkipVerify: false,
		}
	}

	// 配置SASL
	if n.config.SASL {
		cfg.Net.SASL.Enable = true
		cfg.Net.SASL.User = n.config.Username
		cfg.Net.SASL.Password = n.config.Password
		cfg.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	}

	producer, err := sarama.NewSyncProducer(n.config.Brokers, cfg)
	if err != nil {
		return err
	}

	n.producer = producer
	n.log.Infof("Kafka通知器已连接: brokers=%v, topic=%s", n.config.Brokers, n.config.Topic)
	return nil
}

// Notify 发送通知到Kafka
func (n *KafkaNotifier) Notify(ctx context.Context, event *AlertEvent) error {
	if !n.config.Enabled {
		return nil
	}

	n.mu.RLock()
	if !n.ready {
		n.mu.RUnlock()
		return fmt.Errorf("Kafka通知器未就绪")
	}
	n.mu.RUnlock()

	msg := n.buildMessage(event)
	data, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("序列化告警消息失败: %w", err)
	}

	// 重试机制
	var lastErr error
	for i := 0; i <= n.config.MaxRetries; i++ {
		if i > 0 {
			time.Sleep(n.config.RetryDelay)
			n.log.Debugf("Kafka推送重试 %d/%d", i, n.config.MaxRetries)
		}

		err = n.send(ctx, data)
		if err == nil {
			return nil
		}
		lastErr = err

		// 检查是否是可重试错误
		if !n.isRetryableError(err) {
			break
		}
	}

	return fmt.Errorf("Kafka推送失败(重试%d次): %w", n.config.MaxRetries, lastErr)
}

// send 发送消息到Kafka
func (n *KafkaNotifier) send(ctx context.Context, data []byte) error {
	msg := &sarama.ProducerMessage{
		Topic:     n.config.Topic,
		Partition: n.config.Partition,
		Value:     sarama.ByteEncoder(data),
		Timestamp: time.Now(),
	}

	// 添加上下文超时控制
	done := make(chan error, 1)
	go func() {
		_, _, err := n.producer.SendMessage(msg)
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// isRetryableError 判断是否是可重试的错误
func (n *KafkaNotifier) isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// 网络错误、超时错误等可以重试
	// 配置错误、认证错误等不应该重试
	errStr := err.Error()
	retryablePatterns := []string{
		"connection refused",
		"timeout",
		"network",
		"EOF",
		"broken pipe",
	}
	for _, pattern := range retryablePatterns {
		if bytes.Contains([]byte(errStr), []byte(pattern)) {
			return true
		}
	}
	return false
}

// buildMessage 构建告警消息
func (n *KafkaNotifier) buildMessage(event *AlertEvent) *AlertMessage {
	status := "firing"
	if event.State == AlertStateResolved {
		status = "resolved"
	}

	return &AlertMessage{
		ID:          event.ID,
		RuleID:      event.RuleID,
		RuleName:    event.RuleName,
		Level:       event.Level.String(),
		State:       event.State.String(),
		Status:      status,
		Fingerprint: event.GenerateFingerprint(),
		Metric:      event.Metric,
		Value:       event.Value,
		Threshold:   event.Threshold,
		Labels:      event.Labels,
		Annotations: event.Annotations,
		FiredAt:     event.FiredAt,
		Duration:    event.Duration,
		Source:      "cloud-flow-agent",
		Description: event.Annotations["description"],
		Suggestion:  event.Annotations["suggestion"],
	}
}

// Close 关闭Kafka连接
func (n *KafkaNotifier) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.producer != nil {
		n.ready = false
		return n.producer.Close()
	}
	return nil
}

// IsReady 检查是否就绪
func (n *KafkaNotifier) IsReady() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.ready
}

// ==================== APINotifier API通知器 ====================

// APINotifier API通知器
type APINotifier struct {
	config APINotifierConfig
	client *http.Client
	log    *logger.Logger
}

// NewAPINotifier 创建API通知器
func NewAPINotifier(config APINotifierConfig, log *logger.Logger) *APINotifier {
	if !config.Enabled {
		return &APINotifier{config: config, log: log}
	}

	return &APINotifier{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: false,
				},
			},
		},
		log: log,
	}
}

// Notify 发送通知到API
func (n *APINotifier) Notify(ctx context.Context, event *AlertEvent) error {
	if !n.config.Enabled {
		return nil
	}

	if n.config.Endpoint == "" {
		return fmt.Errorf("API endpoint未配置")
	}

	msg := n.buildMessage(event)
	data, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("序列化告警消息失败: %w", err)
	}

	// 重试机制
	var lastErr error
	for i := 0; i <= n.config.RetryCount; i++ {
		if i > 0 {
			time.Sleep(time.Second * time.Duration(i))
			n.log.Debugf("API推送重试 %d/%d", i, n.config.RetryCount)
		}

		err = n.send(ctx, data)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("API推送失败(重试%d次): %w", n.config.RetryCount, lastErr)
}

// send 发送HTTP请求
func (n *APINotifier) send(ctx context.Context, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, n.config.Method, n.config.Endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	for k, v := range n.config.Headers {
		req.Header.Set(k, v)
	}

	// 设置认证
	switch n.config.AuthType {
	case "basic":
		req.SetBasicAuth(n.config.Username, n.config.Password)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+n.config.Token)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API返回错误状态码: %d", resp.StatusCode)
	}

	return nil
}

// buildMessage 构建告警消息
func (n *APINotifier) buildMessage(event *AlertEvent) *AlertMessage {
	status := "firing"
	if event.State == AlertStateResolved {
		status = "resolved"
	}

	var resolvedAt *time.Time
	if !event.ResolvedAt.IsZero() {
		resolvedAt = &event.ResolvedAt
	}

	return &AlertMessage{
		ID:          event.ID,
		RuleID:      event.RuleID,
		RuleName:    event.RuleName,
		Level:       event.Level.String(),
		State:       event.State.String(),
		Status:      status,
		Fingerprint: event.GenerateFingerprint(),
		Metric:      event.Metric,
		Value:       event.Value,
		Threshold:   event.Threshold,
		Labels:      event.Labels,
		Annotations: event.Annotations,
		FiredAt:     event.FiredAt,
		ResolvedAt:  resolvedAt,
		Duration:    event.Duration,
		Source:      "cloud-flow-agent",
		Description: event.Annotations["description"],
		Suggestion:  event.Annotations["suggestion"],
	}
}

// ==================== WebhookNotifier Webhook通知器 ====================

// WebhookNotifier Webhook通知器
type WebhookNotifier struct {
	config WebhookNotifierConfig
	client *http.Client
	log    *logger.Logger
}

// NewWebhookNotifier 创建Webhook通知器
func NewWebhookNotifier(config WebhookNotifierConfig, log *logger.Logger) *WebhookNotifier {
	if !config.Enabled {
		return &WebhookNotifier{config: config, log: log}
	}

	return &WebhookNotifier{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		log: log,
	}
}

// Notify 发送Webhook通知
func (n *WebhookNotifier) Notify(ctx context.Context, event *AlertEvent) error {
	if !n.config.Enabled {
		return nil
	}

	if n.config.URL == "" {
		return fmt.Errorf("Webhook URL未配置")
	}

	// 验证URL
	if _, err := url.Parse(n.config.URL); err != nil {
		return fmt.Errorf("无效的Webhook URL: %w", err)
	}

	msg := n.buildMessage(event)
	data, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("序列化告警消息失败: %w", err)
	}

	// 重试机制
	var lastErr error
	for i := 0; i <= n.config.RetryCount; i++ {
		if i > 0 {
			time.Sleep(time.Second * time.Duration(i))
		}

		err = n.send(ctx, data)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("Webhook推送失败(重试%d次): %w", n.config.RetryCount, lastErr)
}

// send 发送HTTP请求
func (n *WebhookNotifier) send(ctx context.Context, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, n.config.Method, n.config.URL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	for k, v := range n.config.Headers {
		req.Header.Set(k, v)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Webhook返回错误状态码: %d", resp.StatusCode)
	}

	return nil
}

// buildMessage 构建告警消息
func (n *WebhookNotifier) buildMessage(event *AlertEvent) *AlertMessage {
	status := "firing"
	if event.State == AlertStateResolved {
		status = "resolved"
	}

	var resolvedAt *time.Time
	if !event.ResolvedAt.IsZero() {
		resolvedAt = &event.ResolvedAt
	}

	return &AlertMessage{
		ID:          event.ID,
		RuleID:      event.RuleID,
		RuleName:    event.RuleName,
		Level:       event.Level.String(),
		State:       event.State.String(),
		Status:      status,
		Fingerprint: event.GenerateFingerprint(),
		Metric:      event.Metric,
		Value:       event.Value,
		Threshold:   event.Threshold,
		Labels:      event.Labels,
		Annotations: event.Annotations,
		FiredAt:     event.FiredAt,
		ResolvedAt:  resolvedAt,
		Duration:    event.Duration,
		Source:      "cloud-flow-agent",
		Description: event.Annotations["description"],
		Suggestion:  event.Annotations["suggestion"],
	}
}

// ==================== NotifierFactory 通知器工厂 ====================

// NotifierFactory 通知器工厂
type NotifierFactory struct {
	config NotifierConfig
	log    *logger.Logger
}

// NewNotifierFactory 创建通知器工厂
func NewNotifierFactory(config NotifierConfig, log *logger.Logger) *NotifierFactory {
	return &NotifierFactory{
		config: config,
		log:    log,
	}
}

// CreateMultiNotifier 创建多通道通知器
func (f *NotifierFactory) CreateMultiNotifier() (*MultiNotifier, error) {
	multi := NewMultiNotifier()

	// Kafka通知器
	if f.config.Kafka.Enabled {
		kafkaNotifier, err := NewKafkaNotifier(f.config.Kafka, f.log)
		if err != nil {
			return nil, fmt.Errorf("创建Kafka通知器失败: %w", err)
		}
		multi.AddNotifier("kafka", kafkaNotifier)
		f.log.Info("Kafka通知器已启用")
	}

	// API通知器
	if f.config.API.Enabled {
		apiNotifier := NewAPINotifier(f.config.API, f.log)
		multi.AddNotifier("api", apiNotifier)
		f.log.Info("API通知器已启用")
	}

	// Webhook通知器
	if f.config.Webhook.Enabled {
		webhookNotifier := NewWebhookNotifier(f.config.Webhook, f.log)
		multi.AddNotifier("webhook", webhookNotifier)
		f.log.Info("Webhook通知器已启用")
	}

	return multi, nil
}

// CreateFromConfig 从配置创建通知器（兼容旧版配置格式）
func CreateFromConfig(config map[string]interface{}, log *logger.Logger) (Notifier, error) {
	factory := NewNotifierFactory(DefaultNotifierConfig(), log)

	// 解析Kafka配置
	if kafkaCfg, ok := config["kafka"].(map[string]interface{}); ok {
		factory.config.Kafka.Enabled = getBool(kafkaCfg, "enabled", false)
		factory.config.Kafka.Brokers = getStringSlice(kafkaCfg, "brokers")
		factory.config.Kafka.Topic = getString(kafkaCfg, "topic", "alerts")
		factory.config.Kafka.Username = getString(kafkaCfg, "username", "")
		factory.config.Kafka.Password = getString(kafkaCfg, "password", "")
		factory.config.Kafka.TLS = getBool(kafkaCfg, "tls", false)
		factory.config.Kafka.SASL = getBool(kafkaCfg, "sasl", false)
	}

	// 解析API配置
	if apiCfg, ok := config["api"].(map[string]interface{}); ok {
		factory.config.API.Enabled = getBool(apiCfg, "enabled", false)
		factory.config.API.Endpoint = getString(apiCfg, "endpoint", "")
		factory.config.API.Method = getString(apiCfg, "method", "POST")
		factory.config.API.AuthType = getString(apiCfg, "auth_type", "none")
		factory.config.API.Token = getString(apiCfg, "token", "")
		factory.config.API.Username = getString(apiCfg, "username", "")
		factory.config.API.Password = getString(apiCfg, "password", "")
		if headers, ok := apiCfg["headers"].(map[string]string); ok {
			factory.config.API.Headers = headers
		}
	}

	// 解析Webhook配置
	if webhookCfg, ok := config["webhook"].(map[string]interface{}); ok {
		factory.config.Webhook.Enabled = getBool(webhookCfg, "enabled", false)
		factory.config.Webhook.URL = getString(webhookCfg, "url", "")
		factory.config.Webhook.Method = getString(webhookCfg, "method", "POST")
		if headers, ok := webhookCfg["headers"].(map[string]string); ok {
			factory.config.Webhook.Headers = headers
		}
	}

	return factory.CreateMultiNotifier()
}

// 辅助函数
func getBool(m map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return defaultVal
}

func getString(m map[string]interface{}, key, defaultVal string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultVal
}

func getStringSlice(m map[string]interface{}, key string) []string {
	if v, ok := m[key].([]string); ok {
		return v
	}
	if v, ok := m[key].([]interface{}); ok {
		result := make([]string, len(v))
		for i, item := range v {
			if s, ok := item.(string); ok {
				result[i] = s
			}
		}
		return result
	}
	return nil
}
