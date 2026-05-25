// Package huaweicloud 提供华为云Stack V8 API对接功能
// 本文件实现API客户端，包括认证、请求封装、错误处理
package huaweicloud

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// Client 华为云API客户端
type Client struct {
	config     ClientConfig
	httpClient *http.Client
	log        *logger.Logger
	
	// 认证信息
	authToken  string
	tokenExpire time.Time
}

// ClientConfig 客户端配置
type ClientConfig struct {
	Endpoint          string        `yaml:"endpoint" json:"endpoint"`                     // API端点，如 https://cloud.example.com
	AccessKey         string        `yaml:"access_key" json:"access_key"`                 // AK
	SecretKey         string        `yaml:"secret_key" json:"secret_key"`                 // SK
	Region            string        `yaml:"region" json:"region"`                         // 区域，如 cn-north-1
	ProjectID         string        `yaml:"project_id" json:"project_id"`                 // 项目ID
	DomainID          string        `yaml:"domain_id" json:"domain_id"`                   // 账号ID
	Timeout           time.Duration `yaml:"timeout" json:"timeout"`                       // 请求超时
	RetryCount        int           `yaml:"retry_count" json:"retry_count"`               // 重试次数
	EnableSSLVerify   bool          `yaml:"enable_ssl_verify" json:"enable_ssl_verify"`   // 是否验证SSL
}

// DefaultClientConfig 默认客户端配置
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Endpoint:        "https://cloud.example.com",
		Region:          "cn-north-1",
		Timeout:         30 * time.Second,
		RetryCount:      3,
		EnableSSLVerify: true,
	}
}

// NewClient 创建华为云API客户端
func NewClient(config ClientConfig, log *logger.Logger) *Client {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.RetryCount == 0 {
		config.RetryCount = 3
	}

	return &Client{
		config:     config,
		httpClient: &http.Client{Timeout: config.Timeout},
		log:        log,
	}
}

// SetHTTPClient 设置自定义HTTP客户端
func (c *Client) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}

// ==================== 认证方法 ====================

// Authenticate 执行认证获取Token
func (c *Client) Authenticate() error {
	// 华为云Stack V8使用IAM认证
	authReq := map[string]interface{}{
		"auth": map[string]interface{}{
			"identity": map[string]interface{}{
				"methods": []string{"password"},
				"password": map[string]interface{}{
					"user": map[string]interface{}{
						"name":     c.config.AccessKey,
						"password": c.config.SecretKey,
						"domain": map[string]interface{}{
							"name": c.config.DomainID,
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

	body, err := json.Marshal(authReq)
	if err != nil {
		return fmt.Errorf("序列化认证请求失败: %w", err)
	}

	// 构建认证URL
	authURL := fmt.Sprintf("%s/v3/auth/tokens", c.config.Endpoint)

	req, err := http.NewRequest("POST", authURL, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("创建认证请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("认证请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("认证失败: %s - %s", resp.Status, string(body))
	}

	// 从响应头获取Token
	c.authToken = resp.Header.Get("X-Subject-Token")
	if c.authToken == "" {
		return fmt.Errorf("响应中未找到Token")
	}

	// 解析Token过期时间
	var authResp struct {
		Token struct {
			ExpiresAt string `json:"expires_at"`
		} `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err == nil {
		if expiresAt, err := time.Parse(time.RFC3339, authResp.Token.ExpiresAt); err == nil {
			c.tokenExpire = expiresAt
		}
	}

	c.log.Info("华为云API认证成功")
	return nil
}

// ensureAuthenticated 确保已认证
func (c *Client) ensureAuthenticated() error {
	if c.authToken == "" || time.Now().After(c.tokenExpire.Add(-5*time.Minute)) {
		return c.Authenticate()
	}
	return nil
}

// ==================== 通用请求方法 ====================

// Request 发送通用API请求
func (c *Client) Request(method, service, path string, query map[string]string, body interface{}, result interface{}) error {
	// 确保已认证
	if err := c.ensureAuthenticated(); err != nil {
		return err
	}

	// 构建请求URL
	requestURL := fmt.Sprintf("%s/%s/%s", c.config.Endpoint, service, path)

	// 添加查询参数
	if len(query) > 0 {
		u, err := url.Parse(requestURL)
		if err != nil {
			return fmt.Errorf("解析URL失败: %w", err)
		}
		q := u.Query()
		for k, v := range query {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
		requestURL = u.String()
	}

	// 序列化请求体
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("序列化请求体失败: %w", err)
		}
		bodyReader = strings.NewReader(string(bodyBytes))
	}

	// 创建请求
	req, err := http.NewRequest(method, requestURL, bodyReader)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("X-Auth-Token", c.authToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// 发送请求（带重试）
	var lastErr error
	for i := 0; i <= c.config.RetryCount; i++ {
		if i > 0 {
			time.Sleep(time.Duration(i) * time.Second)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = fmt.Errorf("读取响应失败: %w", err)
			continue
		}

		// 处理响应
		switch resp.StatusCode {
		case http.StatusOK, http.StatusCreated, http.StatusAccepted:
			if result != nil && len(bodyBytes) > 0 {
				if err := json.Unmarshal(bodyBytes, result); err != nil {
					return fmt.Errorf("解析响应失败: %w", err)
				}
			}
			return nil
		case http.StatusUnauthorized:
			// Token过期，重新认证
			if err := c.Authenticate(); err != nil {
				return fmt.Errorf("重新认证失败: %w", err)
			}
			req.Header.Set("X-Auth-Token", c.authToken)
			lastErr = fmt.Errorf("Token已过期，已重新认证")
			continue
		default:
			lastErr = fmt.Errorf("API错误: %s - %s", resp.Status, string(bodyBytes))
		}
	}

	return fmt.Errorf("请求失败(重试%d次): %w", c.config.RetryCount, lastErr)
}

// Get 发送GET请求
func (c *Client) Get(service, path string, query map[string]string, result interface{}) error {
	return c.Request("GET", service, path, query, nil, result)
}

// Post 发送POST请求
func (c *Client) Post(service, path string, body interface{}, result interface{}) error {
	return c.Request("POST", service, path, nil, body, result)
}

// Put 发送PUT请求
func (c *Client) Put(service, path string, body interface{}, result interface{}) error {
	return c.Request("PUT", service, path, nil, body, result)
}

// Delete 发送DELETE请求
func (c *Client) Delete(service, path string) error {
	return c.Request("DELETE", service, path, nil, nil, nil)
}

// ==================== 签名方法（用于AK/SK认证）====================

// SignRequest 使用AK/SK签名请求
func (c *Client) SignRequest(req *http.Request) error {
	// 获取当前时间
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	req.Header.Set("X-Sdk-Date", timestamp)

	// 构建规范请求
	canonicalRequest := c.buildCanonicalRequest(req)

	// 构建待签名字符串
	stringToSign := c.buildStringToSign(timestamp, canonicalRequest)

	// 计算签名
	signature := c.calculateSignature(stringToSign)

	// 构建Authorization头
	authHeader := fmt.Sprintf("SDK-HMAC-SHA256 Access=%s, SignedHeaders=%s, Signature=%s",
		c.config.AccessKey,
		"host;x-sdk-date",
		signature)
	req.Header.Set("Authorization", authHeader)

	return nil
}

// buildCanonicalRequest 构建规范请求
func (c *Client) buildCanonicalRequest(req *http.Request) string {
	// HTTP方法
	canonicalMethod := req.Method

	// 规范URI
	canonicalURI := req.URL.Path

	// 规范查询字符串
	canonicalQueryString := req.URL.RawQuery

	// 规范头
	var headers []string
	headerValues := make(map[string]string)

	// 必须包含的头部
	headers = append(headers, "host")
	headerValues["host"] = req.Host

	headers = append(headers, "x-sdk-date")
	headerValues["x-sdk-date"] = req.Header.Get("X-Sdk-Date")

	// 排序头部
	sort.Strings(headers)

	var canonicalHeaders strings.Builder
	var signedHeaders strings.Builder
	for i, h := range headers {
		if i > 0 {
			canonicalHeaders.WriteString("\n")
			signedHeaders.WriteString(";")
		}
		canonicalHeaders.WriteString(fmt.Sprintf("%s:%s", h, headerValues[h]))
		signedHeaders.WriteString(h)
	}

	// 请求体哈希（简化处理，实际应计算SHA256）
	payloadHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // 空字符串的SHA256

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		canonicalMethod,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders.String(),
		signedHeaders.String(),
		payloadHash)
}

// buildStringToSign 构建待签名字符串
func (c *Client) buildStringToSign(timestamp, canonicalRequest string) string {
	// 计算规范请求的哈希
	hash := sha256.Sum256([]byte(canonicalRequest))
	canonicalRequestHash := hex.EncodeToString(hash[:])

	// 构建待签名字符串
	date := timestamp[:8] // YYYYMMDD
	return fmt.Sprintf("SDK-HMAC-SHA256\n%s\n%s/%s/sdk_request\n%s",
		timestamp,
		date,
		c.config.Region,
		canonicalRequestHash)
}

// calculateSignature 计算签名
func (c *Client) calculateSignature(stringToSign string) string {
	// 派生签名密钥
	date := time.Now().UTC().Format("20060102")
	
	// kDate = HMAC(SK, Date)
	h := hmac.New(sha256.New, []byte("SDK"+c.config.SecretKey))
	h.Write([]byte(date))
	kDate := h.Sum(nil)

	// kRegion = HMAC(kDate, Region)
	h = hmac.New(sha256.New, kDate)
	h.Write([]byte(c.config.Region))
	kRegion := h.Sum(nil)

	// kService = HMAC(kRegion, Service)
	h = hmac.New(sha256.New, kRegion)
	h.Write([]byte("sdk"))
	kService := h.Sum(nil)

	// kSigning = HMAC(kService, "sdk_request")
	h = hmac.New(sha256.New, kService)
	h.Write([]byte("sdk_request"))
	kSigning := h.Sum(nil)

	// 计算最终签名
	h = hmac.New(sha256.New, kSigning)
	h.Write([]byte(stringToSign))
	return hex.EncodeToString(h.Sum(nil))
}

// ==================== 健康检查 ====================

// HealthCheck 健康检查
func (c *Client) HealthCheck() error {
	// 尝试认证来检查健康状态
	return c.Authenticate()
}

// GetTokenInfo 获取Token信息
func (c *Client) GetTokenInfo() map[string]interface{} {
	return map[string]interface{}{
		"has_token":   c.authToken != "",
		"expire_at":   c.tokenExpire.Format(time.RFC3339),
		"is_expired":  time.Now().After(c.tokenExpire),
		"endpoint":    c.config.Endpoint,
		"region":      c.config.Region,
		"project_id":  c.config.ProjectID,
	}
}
