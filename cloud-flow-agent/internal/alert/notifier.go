//go:build linux

// Package alert 提供告警推送功能
// 支持: Kafka推送、API推送、Webhook推送
package alert

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// KafkaNotifier Kafka通知器
type KafkaNotifier struct {
	config    KafkaConfig
	producer  interface{} // Kafka producer (简化实现)
	client    *http.Client
	log       *logger.Logger
	
	// 统计
	stats struct {
		sentCount   atomic.Uint64
		failedCount atomic.Uint64
	}
}

// KafkaConfig Kafka配置
type KafkaConfig struct {
	Enabled   bool     `yaml:"enabled" json:"enabled"`
	Brokers   []string `yaml:"brokers" json:"brokers"`
	Topic     string   `yaml:"topic" json:"topic"`
	Partition int      `yaml:"partition" json:"partition"`
	
	// 认证
	SASLEnabled bool   `yaml:"sasl_enabled" json:"sasl_enabled"`
	SASLUser    string `yaml:"sasl_user" json:"sasl_user"`
	SASLPass    string `yaml:"sasl_pass" json:"sasl_pass"`
	
	// TLS
	TLSEnabled bool   `yaml:"tls_enabled" json:"tls_enabled"`
	CACert     string `yaml:"ca_cert" json:"ca_cert"`
	
	// 批量发送
	BatchSize    int           `yaml:"batch_size" json:"batch_size"`
	BatchTimeout time.Duration `yaml:"batch_timeout" json:"batch_timeout"`
}

// NewKafkaNotifier 创建Kafka通知器
func NewKafkaNotifier(config KafkaConfig, log *logger.Logger) *KafkaNotifier {
	return &KafkaNotifier{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
		log:    log,
	}
}

// Notify 发送通知
func (n *KafkaNotifier) Notify(ctx context.Context, event *AlertEvent) error {
	if !n.config.Enabled {
		return nil
	}
	
	// 序列化事件
	data, err := json.Marshal(event.ToMap())
	if err != nil {
		return fmt.Errorf("序列化告警事件失败: %w", err)
	}
	
	// 发送到Kafka (简化实现，使用HTTP REST API)
	if err := n.sendToKafka(ctx, data); err != nil {
		n.stats.failedCount.Add(1)
		return err
	}
	
	n.stats.sentCount.Add(1)
	n.log.Debugf("Kafka告警发送成功: %s", event.ID)
	
	return nil
}

// sendToKafka 发送到Kafka
func (n *KafkaNotifier) sendToKafka(ctx context.Context, data []byte) error {
	// 简化实现: 使用Kafka REST Proxy
	// 实际生产环境应使用 github.com/IBM/sarama 或 github.com/confluentinc/confluent-kafka-go
	
	for _, broker := range n.config.Brokers {
		url := fmt.Sprintf("http://%s/topics/%s", broker, n.config.Topic)
		
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
		if err != nil {
			continue
		}
		
		req.Header.Set("Content-Type", "application/vnd.kafka.json.v2+json")
		req.Header.Set("Accept", "application/vnd.kafka.v2+json")
		
		resp, err := n.client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Kafka发送失败: %s - %s", resp.Status, string(body))
	}
	
	return fmt.Errorf("所有Kafka broker都不可用")
}

// GetStats 获取统计
func (n *KafkaNotifier) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"sent_count":   n.stats.sentCount.Load(),
		"failed_count": n.stats.failedCount.Load(),
	}
}

// APINotifier API通知器
type APINotifier struct {
	config APINotifierConfig
	client *http.Client
	log    *logger.Logger
	
	// 统计
	stats struct {
		sentCount   atomic.Uint64
		failedCount atomic.Uint64
	}
}

// APINotifierConfig API通知器配置
type APINotifierConfig struct {
	Enabled  bool              `yaml:"enabled" json:"enabled"`
	URL      string            `yaml:"url" json:"url"`
	Method   string            `yaml:"method" json:"method"`
	Headers  map[string]string `yaml:"headers" json:"headers"`
	Timeout  time.Duration     `yaml:"timeout" json:"timeout"`
	
	// 认证
	AuthType  string `yaml:"auth_type" json:"auth_type"`   // none, basic, bearer, apikey
	AuthUser  string `yaml:"auth_user" json:"auth_user"`
	AuthPass  string `yaml:"auth_pass" json:"auth_pass"`
	AuthToken string `yaml:"auth_token" json:"auth_token"`
	APIKey    string `yaml:"api_key" json:"api_key"`
	
	// 重试
	MaxRetries int           `yaml:"max_retries" json:"max_retries"`
	RetryDelay time.Duration `yaml:"retry_delay" json:"retry_delay"`
	
	// TLS
	TLSEnabled    bool   `yaml:"tls_enabled" json:"tls_enabled"`
	SkipTLSVerify bool   `yaml:"skip_tls_verify" json:"skip_tls_verify"`
	CACert        string `yaml:"ca_cert" json:"ca_cert"`
}

// NewAPINotifier 创建API通知器
func NewAPINotifier(config APINotifierConfig, log *logger.Logger) *APINotifier {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.Method == "" {
		config.Method = "POST"
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = time.Second
	}
	
	// 创建HTTP客户端
	client := &http.Client{
		Timeout: config.Timeout,
	}
	
	if config.TLSEnabled {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: config.SkipTLSVerify,
		}
		client.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	}
	
	return &APINotifier{
		config: config,
		client: client,
		log:    log,
	}
}

// Notify 发送通知
func (n *APINotifier) Notify(ctx context.Context, event *AlertEvent) error {
	if !n.config.Enabled {
		return nil
	}
	
	// 序列化事件
	data, err := json.Marshal(event.ToMap())
	if err != nil {
		return fmt.Errorf("序列化告警事件失败: %w", err)
	}
	
	// 重试发送
	var lastErr error
	for i := 0; i < n.config.MaxRetries; i++ {
		if err := n.sendRequest(ctx, data); err != nil {
			lastErr = err
			time.Sleep(n.config.RetryDelay)
			continue
		}
		
		n.stats.sentCount.Add(1)
		n.log.Debugf("API告警发送成功: %s -> %s", event.ID, n.config.URL)
		return nil
	}
	
	n.stats.failedCount.Add(1)
	return fmt.Errorf("API发送失败(重试%d次): %w", n.config.MaxRetries, lastErr)
}

// sendRequest 发送HTTP请求
func (n *APINotifier) sendRequest(ctx context.Context, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, n.config.Method, n.config.URL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	
	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	for k, v := range n.config.Headers {
		req.Header.Set(k, v)
	}
	
	// 设置认证
	switch n.config.AuthType {
	case "basic":
		req.SetBasicAuth(n.config.AuthUser, n.config.AuthPass)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+n.config.AuthToken)
	case "apikey":
		req.Header.Set("X-API-Key", n.config.APIKey)
	}
	
	// 发送请求
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	// 检查响应
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
}

// GetStats 获取统计
func (n *APINotifier) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"sent_count":   n.stats.sentCount.Load(),
		"failed_count": n.stats.failedCount.Load(),
	}
}

// WebhookNotifier Webhook通知器
type WebhookNotifier struct {
	config WebhookConfig
	client *http.Client
	log    *logger.Logger
	
	stats struct {
		sentCount   atomic.Uint64
		failedCount atomic.Uint64
	}
}

// WebhookConfig Webhook配置
type WebhookConfig struct {
	Enabled  bool              `yaml:"enabled" json:"enabled"`
	URL      string            `yaml:"url" json:"url"`
	Secret   string            `yaml:"secret" json:"secret"`   // 签名密钥
	Headers  map[string]string `yaml:"headers" json:"headers"`
	Timeout  time.Duration     `yaml:"timeout" json:"timeout"`
}

// NewWebhookNotifier 创建Webhook通知器
func NewWebhookNotifier(config WebhookConfig, log *logger.Logger) *WebhookNotifier {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	
	return &WebhookNotifier{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
		log:    log,
	}
}

// Notify 发送通知
func (n *WebhookNotifier) Notify(ctx context.Context, event *AlertEvent) error {
	if !n.config.Enabled {
		return nil
	}
	
	// 构建Webhook负载
	payload := map[string]interface{}{
		"event":     "alert",
		"timestamp": time.Now().Unix(),
		"data":      event.ToMap(),
	}
	
	if n.config.Secret != "" {
		payload["signature"] = n.sign(event)
	}
	
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", n.config.URL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	
	req.Header.Set("Content-Type", "application/json")
	for k, v := range n.config.Headers {
		req.Header.Set(k, v)
	}
	
	resp, err := n.client.Do(req)
	if err != nil {
		n.stats.failedCount.Add(1)
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		n.stats.sentCount.Add(1)
		return nil
	}
	
	body, _ := io.ReadAll(resp.Body)
	n.stats.failedCount.Add(1)
	return fmt.Errorf("Webhook失败: %d - %s", resp.StatusCode, string(body))
}

// sign 生成签名
func (n *WebhookNotifier) sign(event *AlertEvent) string {
	// 简化签名实现
	return fmt.Sprintf("%x", len(event.ID)+len(n.config.Secret))
}

// GetStats 获取统计
func (n *WebhookNotifier) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"sent_count":   n.stats.sentCount.Load(),
		"failed_count": n.stats.failedCount.Load(),
	}
}

// LogNotifier 日志通知器（用于调试）
type LogNotifier struct {
	log *logger.Logger
}

// NewLogNotifier 创建日志通知器
func NewLogNotifier(log *logger.Logger) *LogNotifier {
	return &LogNotifier{log: log}
}

// Notify 记录日志
func (n *LogNotifier) Notify(ctx context.Context, event *AlertEvent) error {
	n.log.Infof("[ALERT] [%s] %s - %s: %s (阈值=%s, 实际=%s)",
		event.Level.String(),
		event.State.String(),
		event.RuleName,
		event.Metric,
		FormatValue(event.Threshold),
		FormatValue(event.Value))
	return nil
}

// NotifierFactory 通知器工厂
type NotifierFactory struct {
	mu        sync.RWMutex
	notifiers map[string]Notifier
}

// NewNotifierFactory 创建通知器工厂
func NewNotifierFactory() *NotifierFactory {
	return &NotifierFactory{
		notifiers: make(map[string]Notifier),
	}
}

// Register 注册通知器
func (f *NotifierFactory) Register(name string, notifier Notifier) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notifiers[name] = notifier
}

// Get 获取通知器
func (f *NotifierFactory) Get(name string) Notifier {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.notifiers[name]
}

// NotifyAll 通知所有
func (f *NotifierFactory) NotifyAll(ctx context.Context, event *AlertEvent) error {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	var lastErr error
	for name, notifier := range f.notifiers {
		if err := notifier.Notify(ctx, event); err != nil {
			lastErr = fmt.Errorf("[%s] %w", name, err)
		}
	}
	return lastErr
}

// NotifyConfig 通知配置
type NotifyConfig struct {
	Kafka   KafkaConfig      `yaml:"kafka" json:"kafka"`
	API     APINotifierConfig `yaml:"api" json:"api"`
	Webhook WebhookConfig    `yaml:"webhook" json:"webhook"`
}

// BuildNotifiers 构建通知器
func BuildNotifiers(config NotifyConfig, log *logger.Logger) *MultiNotifier {
	multi := NewMultiNotifier()
	
	if config.Kafka.Enabled {
		multi.AddNotifier("kafka", NewKafkaNotifier(config.Kafka, log))
	}
	
	if config.API.Enabled {
		multi.AddNotifier("api", NewAPINotifier(config.API, log))
	}
	
	if config.Webhook.Enabled {
		multi.AddNotifier("webhook", NewWebhookNotifier(config.Webhook, log))
	}
	
	// 始终添加日志通知器
	multi.AddNotifier("log", NewLogNotifier(log))
	
	return multi
}
