/*
 * Cloud Flow Agent - Probe Authentication & Authorization
 *
 * 探针认证授权模块，确保只有授权探针才能接入
 */

package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrInvalidToken      = errors.New("invalid token")
	ErrTokenExpired      = errors.New("token expired")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrInvalidSignature  = errors.New("invalid signature")
	ErrRateLimited       = errors.New("rate limited")
	ErrProbeNotFound     = errors.New("probe not found")
	ErrProbeDisabled     = errors.New("probe disabled")
)

// AuthConfig 认证配置
type AuthConfig struct {
	Enabled          bool          // 启用认证
	TokenExpiry      time.Duration // Token 有效期
	TokenRefreshInterval time.Duration // Token 刷新间隔
	MaxTokenPerProbe int           // 每个探针最大 Token 数
	RateLimitPerMin  int           // 每分钟请求限制
	RequireTLS       bool          // 要求 TLS
	AllowedCipherSuites []string   // 允许的加密套件
	HMACKey          []byte        // HMAC 签名密钥
	JWTSecret        []byte        // JWT 密钥
	AdminAPIKey      string        // 管理 API 密钥
}

// DefaultAuthConfig 默认配置
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		Enabled:           true,
		TokenExpiry:       24 * time.Hour,
		TokenRefreshInterval: 1 * time.Hour,
		MaxTokenPerProbe:  5,
		RateLimitPerMin:   1000,
		RequireTLS:        true,
		HMACKey:          []byte("default-hmac-key-change-in-production"),
		JWTSecret:        []byte("default-jwt-secret-change-in-production"),
	}
}

// Probe 探针信息
type Probe struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Secret      string            `json:"-"` // 不序列化
	SecretHash  string            `json:"secret_hash"`
	Tags        map[string]string `json:"tags"`
	Permissions []string          `json:"permissions"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	LastSeenAt  *time.Time        `json:"last_seen_at,omitempty"`
	ExpiredAt   *time.Time        `json:"expired_at,omitempty"`
	RateLimit   int               `json:"rate_limit"` // 探针特定限流
}

// Token Token 信息
type Token struct {
	TokenID     string    `json:"token_id"`
	ProbeID     string    `json:"probe_id"`
	Permissions []string `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	LastUsedAt  time.Time `json:"last_used_at"`
	Scope       string    `json:"scope"` // read/write/admin
}

// AuthRequest 认证请求
type AuthRequest struct {
	ProbeID     string `json:"probe_id"`
	TokenID     string `json:"token_id"`
	Signature   string `json:"signature"`
	Timestamp   int64  `json:"timestamp"`
	Nonce       string `json:"nonce"`
}

// AuthResponse 认证响应
type AuthResponse struct {
	Success     bool     `json:"success"`
	TokenID     string   `json:"token_id,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	ExpiresAt   int64    `json:"expires_at,omitempty"`
	Error       string   `json:"error,omitempty"`
	RateLimit   int      `json:"rate_limit"`
	Remaining   int      `json:"remaining"`
}

// Permission 权限
type Permission string

const (
	PermissionRead   Permission = "read"
	PermissionWrite  Permission = "write"
	PermissionAdmin  Permission = "admin"
	PermissionMetric Permission = "metric"
	PermissionTrace  Permission = "trace"
	PermissionLog    Permission = "log"
	PermissionConfig Permission = "config"
)

// AuthManager 认证管理器
type AuthManager struct {
	mu sync.RWMutex
	
	config *AuthConfig
	
	// 探针注册表
	probes map[string]*Probe
	
	// Token 注册表
	tokens map[string]*Token
	
	// 探针 Token 索引
	probeTokens map[string][]string
	
	// HMAC 签名缓存
	signatureCache map[string]time.Time
	
	// 限流器
	rateLimiters map[string]*RateLimiter
	
	// 统计
	stats AuthStats
	
	// 控制
	ctx    context.Context
	cancel context.CancelFunc
}

// AuthStats 认证统计
type AuthStats struct {
	TotalAuths      uint64
	TotalSuccess    uint64
	TotalFailed     uint64
	TotalTokens     uint32
	TotalProbes     uint32
	RateLimitHits   uint64
	InvalidSigHits  uint64
}

// RateLimiter 限流器
type RateLimiter struct {
	tokens    int
	maxTokens int
	refillAt  time.Time
	mu        sync.Mutex
}

// NewRateLimiter 创建限流器
func NewRateLimiter(maxPerMin int) *RateLimiter {
	return &RateLimiter{
		tokens:    maxPerMin,
		maxTokens: maxPerMin,
		refillAt:  time.Now().Add(time.Minute),
	}
}

// Allow 是否允许
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	if time.Now().After(rl.refillAt) {
		rl.tokens = rl.maxTokens
		rl.refillAt = time.Now().Add(time.Minute)
	}
	
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}
	return false
}

// NewAuthManager 创建认证管理器
func NewAuthManager(config *AuthConfig) (*AuthManager, error) {
	if config == nil {
		config = DefaultAuthConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	mgr := &AuthManager{
		config:         config,
		probes:         make(map[string]*Probe),
		tokens:         make(map[string]*Token),
		probeTokens:    make(map[string][]string),
		signatureCache: make(map[string]time.Time),
		rateLimiters:   make(map[string]*RateLimiter),
		ctx:            ctx,
		cancel:         cancel,
	}
	
	// 启动 Token 清理协程
	go mgr.cleanupExpired()
	
	return mgr, nil
}

// RegisterProbe 注册探针
func (m *AuthManager) RegisterProbe(probe *Probe) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.probes[probe.ID]; exists {
		return fmt.Errorf("probe %s already exists", probe.ID)
	}
	
	// 生成 Secret Hash
	probe.SecretHash = m.hashSecret(probe.Secret)
	probe.CreatedAt = time.Now()
	probe.UpdatedAt = time.Now()
	
	m.probes[probe.ID] = probe
	atomic.AddUint32(&m.stats.TotalProbes, 1)
	
	return nil
}

// UnregisterProbe 注销探针
func (m *AuthManager) UnregisterProbe(probeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	probe, exists := m.probes[probeID]
	if !exists {
		return ErrProbeNotFound
	}
	
	// 撤销所有 Token
	for _, tokenID := range m.probeTokens[probeID] {
		delete(m.tokens, tokenID)
	}
	delete(m.probeTokens, probeID)
	delete(m.probes, probeID)
	
	atomic.AddUint32(&m.stats.TotalProbes, ^uint32(0))
	
	return nil
}

// EnableProbe 启用探针
func (m *AuthManager) EnableProbe(probeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	probe, exists := m.probes[probeID]
	if !exists {
		return ErrProbeNotFound
	}
	
	probe.Enabled = true
	probe.UpdatedAt = time.Now()
	return nil
}

// DisableProbe 禁用探针
func (m *AuthManager) DisableProbe(probeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	probe, exists := m.probes[probeID]
	if !exists {
		return ErrProbeNotFound
	}
	
	probe.Enabled = false
	probe.UpdatedAt = time.Now()
	return nil
}

// CreateToken 为探针创建 Token
func (m *AuthManager) CreateToken(probeID string, scope string, permissions []string) (*Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	probe, exists := m.probes[probeID]
	if !exists {
		return nil, ErrProbeNotFound
	}
	
	if !probe.Enabled {
		return nil, ErrProbeDisabled
	}
	
	// 检查 Token 数量限制
	if len(m.probeTokens[probeID]) >= m.config.MaxTokenPerProbe {
		return nil, fmt.Errorf("max tokens per probe reached: %d", m.config.MaxTokenPerProbe)
	}
	
	// 生成 Token
	tokenID := m.generateTokenID()
	token := &Token{
		TokenID:     tokenID,
		ProbeID:     probeID,
		Permissions: append([]string{}, permissions...),
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(m.config.TokenExpiry),
		Scope:       scope,
	}
	
	m.tokens[tokenID] = token
	m.probeTokens[probeID] = append(m.probeTokens[probeID], tokenID)
	atomic.AddUint32(&m.stats.TotalTokens, 1)
	
	return token, nil
}

// RevokeToken 撤销 Token
func (m *AuthManager) RevokeToken(tokenID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	token, exists := m.tokens[tokenID]
	if !exists {
		return fmt.Errorf("token not found")
	}
	
	// 从探针 Token 列表移除
	tokenIDs := m.probeTokens[token.ProbeID]
	for i, id := range tokenIDs {
		if id == tokenID {
			m.probeTokens[token.ProbeID] = append(tokenIDs[:i], tokenIDs[i+1:]...)
			break
		}
	}
	
	delete(m.tokens, tokenID)
	atomic.AddUint32(&m.stats.TotalTokens, ^uint32(0))
	
	return nil
}

// Authenticate 认证请求
func (m *AuthManager) Authenticate(req *AuthRequest) (*AuthResponse, error) {
	atomic.AddUint64(&m.stats.TotalAuths, 1)
	
	// 限流检查
	if !m.checkRateLimit(req.ProbeID) {
		atomic.AddUint64(&m.stats.RateLimitHits, 1)
		return &AuthResponse{
			Success: false,
			Error:   ErrRateLimited.Error(),
		}, ErrRateLimited
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 验证探针
	probe, exists := m.probes[req.ProbeID]
	if !exists {
		atomic.AddUint64(&m.stats.TotalFailed, 1)
		return &AuthResponse{
			Success: false,
			Error:   ErrProbeNotFound.Error(),
		}, ErrProbeNotFound
	}
	
	if !probe.Enabled {
		atomic.AddUint64(&m.stats.TotalFailed, 1)
		return &AuthResponse{
			Success: false,
			Error:   ErrProbeDisabled.Error(),
		}, ErrProbeDisabled
	}
	
	// 验证 Token
	token, exists := m.tokens[req.TokenID]
	if !exists {
		atomic.AddUint64(&m.stats.TotalFailed, 1)
		return &AuthResponse{
			Success: false,
			Error:   ErrInvalidToken.Error(),
		}, ErrInvalidToken
	}
	
	if token.ProbeID != req.ProbeID {
		atomic.AddUint64(&m.stats.TotalFailed, 1)
		return &AuthResponse{
			Success: false,
			Error:   ErrUnauthorized.Error(),
		}, ErrUnauthorized
	}
	
	// 检查 Token 过期
	if time.Now().After(token.ExpiresAt) {
		atomic.AddUint64(&m.stats.TotalFailed, 1)
		return &AuthResponse{
			Success: false,
			Error:   ErrTokenExpired.Error(),
		}, ErrTokenExpired
	}
	
	// 验证签名
	if !m.verifySignature(req, probe.Secret) {
		atomic.AddUint64(&m.stats.InvalidSigHits, 1)
		atomic.AddUint64(&m.stats.TotalFailed, 1)
		return &AuthResponse{
			Success: false,
			Error:   ErrInvalidSignature.Error(),
		}, ErrInvalidSignature
	}
	
	// 更新最后使用时间
	token.LastUsedAt = time.Now()
	probe.LastSeenAt = &time.Time{}
	*probe.LastSeenAt = time.Now()
	
	atomic.AddUint64(&m.stats.TotalSuccess, 1)
	
	// 确定限流
	rateLimit := probe.RateLimit
	if rateLimit == 0 {
		rateLimit = m.config.RateLimitPerMin
	}
	
	return &AuthResponse{
		Success:     true,
		TokenID:     token.TokenID,
		Permissions: token.Permissions,
		ExpiresAt:   token.ExpiresAt.Unix(),
		RateLimit:   rateLimit,
		Remaining:   rateLimit - 1,
	}, nil
}

// checkRateLimit 检查限流
func (m *AuthManager) checkRateLimit(probeID string) bool {
	limiter, exists := m.rateLimiters[probeID]
	if !exists {
		limiter = NewRateLimiter(m.config.RateLimitPerMin)
		m.rateLimiters[probeID] = limiter
	}
	return limiter.Allow()
}

// verifySignature 验证 HMAC 签名
func (m *AuthManager) verifySignature(req *AuthRequest, secret string) bool {
	// 构造签名字符串
	signData := fmt.Sprintf("%s:%s:%d:%s", req.ProbeID, req.TokenID, req.Timestamp, req.Nonce)
	
	// 计算 HMAC
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signData))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	
	// 常数时间比较
	return subtle.ConstantTimeCompare([]byte(req.Signature), []byte(expectedSig)) == 1
}

// GenerateSignature 生成签名（供探针使用）
func (m *AuthManager) GenerateSignature(probeID, tokenID, nonce string, timestamp int64, secret string) string {
	signData := fmt.Sprintf("%s:%s:%d:%s", probeID, tokenID, timestamp, nonce)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signData))
	return hex.EncodeToString(mac.Sum(nil))
}

// hashSecret 哈希密钥
func (m *AuthManager) hashSecret(secret string) string {
	hash := sha256.Sum256([]byte(secret))
	return base64.StdEncoding.EncodeToString(hash[:])
}

// generateTokenID 生成 Token ID
func (m *AuthManager) generateTokenID() string {
	// 使用时间戳 + 随机数
	data := fmt.Sprintf("%d:%s", time.Now().UnixNano(), fmt.Sprintf("%x", time.Now().UnixNano()))
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:32]
}

// cleanupExpired 清理过期 Token
func (m *AuthManager) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			m.cleanExpiredTokens()
		case <-m.ctx.Done():
			return
		}
	}
}

// cleanExpiredTokens 清理过期 Token
func (m *AuthManager) cleanExpiredTokens() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	for tokenID, token := range m.tokens {
		if now.After(token.ExpiresAt) {
			// 从探针 Token 列表移除
			tokenIDs := m.probeTokens[token.ProbeID]
			for i, id := range tokenIDs {
				if id == tokenID {
					m.probeTokens[token.ProbeID] = append(tokenIDs[:i], tokenIDs[i+1:]...)
					break
				}
			}
			delete(m.tokens, tokenID)
			atomic.AddUint32(&m.stats.TotalTokens, ^uint32(0))
		}
	}
}

// GetProbe 获取探针信息
func (m *AuthManager) GetProbe(probeID string) (*Probe, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	probe, exists := m.probes[probeID]
	if !exists {
		return nil, ErrProbeNotFound
	}
	
	// 返回副本
	copy := *probe
	return &copy, nil
}

// ListProbes 列出所有探针
func (m *AuthManager) ListProbes() []*Probe {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	probes := make([]*Probe, 0, len(m.probes))
	for _, probe := range m.probes {
		copy := *probe
		probes = append(probes, &copy)
	}
	return probes
}

// GetStats 获取统计
func (m *AuthManager) GetStats() AuthStats {
	return AuthStats{
		TotalAuths:     atomic.LoadUint64(&m.stats.TotalAuths),
		TotalSuccess:   atomic.LoadUint64(&m.stats.TotalSuccess),
		TotalFailed:    atomic.LoadUint64(&m.stats.TotalFailed),
		TotalTokens:    atomic.LoadUint32(&m.stats.TotalTokens),
		TotalProbes:    atomic.LoadUint32(&m.stats.TotalProbes),
		RateLimitHits:  atomic.LoadUint64(&m.stats.RateLimitHits),
		InvalidSigHits: atomic.LoadUint64(&m.stats.InvalidSigHits),
	}
}

// Close 关闭管理器
func (m *AuthManager) Close() error {
	m.cancel()
	return nil
}
