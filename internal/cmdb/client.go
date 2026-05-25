//go:build linux

// Package cmdb 提供CMDB系统对接功能
// 本文件实现CMDB API客户端
package cmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"sync"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// ClientConfig CMDB客户端配置
type ClientConfig struct {
	Endpoint        string        `yaml:"endpoint" json:"endpoint"`               // CMDB API端点
	APIKey          string        `yaml:"api_key" json:"api_key"`                 // API密钥
	APISecret       string        `yaml:"api_secret" json:"api_secret"`           // API密钥
	Timeout         time.Duration `yaml:"timeout" json:"timeout"`                   // 请求超时
	RetryCount      int           `yaml:"retry_count" json:"retry_count"`         // 重试次数
	EnableSSLVerify bool          `yaml:"enable_ssl_verify" json:"enable_ssl_verify"` // 是否验证SSL
}

// DefaultClientConfig 默认客户端配置
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Endpoint:        "http://cmdb.example.com/api/v1",
		Timeout:         30 * time.Second,
		RetryCount:      3,
		EnableSSLVerify: true,
	}
}

// Client CMDB API客户端
type Client struct {
	config     ClientConfig
	httpClient *http.Client
	log        *logger.Logger

	// 认证信息
	authToken   string
	tokenExpire time.Time
	authMu      sync.Mutex
}

// NewClient 创建CMDB客户端
func NewClient(config ClientConfig, log *logger.Logger) *Client {
	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		log: log,
	}
}

// ==================== 认证相关 ====================

// Authenticate 认证获取Token
func (c *Client) Authenticate() error {
	c.authMu.Lock()
	defer c.authMu.Unlock()

	authURL := fmt.Sprintf("%s/auth/token", c.config.Endpoint)

	reqBody := map[string]string{
		"api_key":    c.config.APIKey,
		"api_secret": c.config.APISecret,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal auth request: %w", err)
	}

	req, err := http.NewRequest("POST", authURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("auth failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		Token     string `json:"token"`
		ExpireIn  int    `json:"expire_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode auth response: %w", err)
	}

	c.authToken = result.Token
	c.tokenExpire = time.Now().Add(time.Duration(result.ExpireIn) * time.Second)

	c.log.Info("CMDB认证成功")
	return nil
}

// ensureAuthenticated 确保已认证
func (c *Client) ensureAuthenticated() error {
	c.authMu.Lock()
	defer c.authMu.Unlock()

	// 双重检查
	if c.authToken != "" && time.Now().Before(c.tokenExpire.Add(-5*time.Minute)) {
		return nil
	}

	return c.Authenticate()
}

// getAuthHeader 获取认证头
func (c *Client) getAuthHeader() string {
	c.authMu.Lock()
	defer c.authMu.Unlock()
	return "Bearer " + c.authToken
}

// ==================== 业务系统API ====================

// GetBusinessSystems 获取业务系统列表
func (c *Client) GetBusinessSystems(ctx context.Context) ([]BusinessSystem, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/systems", c.config.Endpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", c.getAuthHeader())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		Code    int               `json:"code"`
		Message string            `json:"message"`
		Data    []BusinessSystem  `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("API error: %s", result.Message)
	}

	return result.Data, nil
}

// GetBusinessSystemByID 根据ID获取业务系统
func (c *Client) GetBusinessSystemByID(ctx context.Context, systemID string) (*BusinessSystem, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/systems/%s", c.config.Endpoint, systemID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", c.getAuthHeader())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    *BusinessSystem `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("API error: %s", result.Message)
	}

	return result.Data, nil
}

// ==================== CI项API ====================

// GetCIItems 获取CI列表
func (c *Client) GetCIItems(ctx context.Context, ciType string) ([]CIItem, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/ci_items", c.config.Endpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 添加查询参数
	q := req.URL.Query()
	if ciType != "" {
		q.Set("ci_type", ciType)
	}
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", c.getAuthHeader())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		Code    int      `json:"code"`
		Message string   `json:"message"`
		Data    []CIItem `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("API error: %s", result.Message)
	}

	return result.Data, nil
}

// GetCIItemByIP 根据IP获取CI
func (c *Client) GetCIItemByIP(ctx context.Context, ip string) (*CIItem, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/ci_items/search", c.config.Endpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("ip", ip)
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", c.getAuthHeader())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		Code    int     `json:"code"`
		Message string  `json:"message"`
		Data    []CIItem `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("API error: %s", result.Message)
	}

	if len(result.Data) == 0 {
		return nil, nil
	}

	return &result.Data[0], nil
}

// GetCIItemByHostname 根据主机名获取CI
func (c *Client) GetCIItemByHostname(ctx context.Context, hostname string) (*CIItem, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/ci_items/search", c.config.Endpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("hostname", hostname)
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", c.getAuthHeader())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		Code    int     `json:"code"`
		Message string  `json:"message"`
		Data    []CIItem `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("API error: %s", result.Message)
	}

	if len(result.Data) == 0 {
		return nil, nil
	}

	return &result.Data[0], nil
}

// QueryCIItems 高级查询CI
func (c *Client) QueryCIItems(ctx context.Context, req QueryRequest) ([]CIItem, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/ci_items/query", c.config.Endpoint)

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Authorization", c.getAuthHeader())
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		Code    int      `json:"code"`
		Message string   `json:"message"`
		Data    []CIItem `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("API error: %s", result.Message)
	}

	return result.Data, nil
}

// ==================== 批量查询 ====================

// BatchGetCIItems 批量获取CI（用于标签注入）
func (c *Client) BatchGetCIItems(ctx context.Context, ips []string, hostnames []string) (map[string]*CIItem, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/ci_items/batch", c.config.Endpoint)

	reqBody := map[string]interface{}{
		"ips":       ips,
		"hostnames": hostnames,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", c.getAuthHeader())
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		Code    int                 `json:"code"`
		Message string              `json:"message"`
		Data    map[string]*CIItem  `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("API error: %s", result.Message)
	}

	return result.Data, nil
}

// bytes 包辅助
type bytes struct{}

func (bytes) NewReader(b []byte) *bytesReader {
	return &bytesReader{data: b, pos: 0}
}

type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

var bytes = bytes{}
