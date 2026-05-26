// Package auth JWT + OIDC 认证模块
//
// 支持:
//   - JWT 签发与验证 (HS256)
//   - OIDC Discovery + Token Exchange
//   - API Key 认证
//   - Token 刷新
//
// JWT Claims 结构:
//
//	{
//	  "sub": "user_id",
//	  "tenant_id": "tenant_xxx",
//	  "username": "admin",
//	  "role": "admin",
//	  "project_id": "",
//	  "namespaces": ["ns1", "ns2"],
//	  "iat": 1234567890,
//	  "exp": 1234654290,
//	  "iss": "cloudflow"
//	}
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

// ==================== Claims ====================

// Claims JWT 自定义声明，嵌入 jwt.RegisteredClaims
type Claims struct {
	TenantID       string   `json:"tenant_id"`
	Username       string   `json:"username"`
	Role           string   `json:"role"`
	ProjectID      string   `json:"project_id,omitempty"`
	Namespaces     []string `json:"namespaces,omitempty"`
	IsPlatformAdmin bool    `json:"is_platform_admin,omitempty"`
	jwt.RegisteredClaims
}

// ==================== JWTManager ====================

// JWTManager JWT 签发与验证管理器
type JWTManager struct {
	secret         []byte
	issuer         string
	expireDuration time.Duration
	refreshDuration time.Duration
	signingMethod  jwt.SigningMethod
}

// NewJWTManager 创建 JWT 管理器
//
//   - secret: HMAC 签名密钥
//   - issuer: token 签发者标识 (如 "cloudflow")
//   - expireSec: access token 有效期 (秒)
//   - refreshSec: refresh token 有效期 (秒)
func NewJWTManager(secret, issuer string, expireSec, refreshSec int64) *JWTManager {
	return &JWTManager{
		secret:          []byte(secret),
		issuer:          issuer,
		expireDuration:  time.Duration(expireSec) * time.Second,
		refreshDuration: time.Duration(refreshSec) * time.Second,
		signingMethod:   jwt.SigningMethodHS256,
	}
}

// GenerateToken 签发 access token
//
// 将用户身份信息编码到 JWT Claims 中，使用 HS256 签名。
func (m *JWTManager) GenerateToken(userID, tenantID, username, role string, projectID string, namespaces []string, isPlatformAdmin bool) (string, error) {
	now := time.Now()
	claims := &Claims{
		TenantID:        tenantID,
		Username:        username,
		Role:            role,
		ProjectID:       projectID,
		Namespaces:      namespaces,
		IsPlatformAdmin: isPlatformAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.expireDuration)),
			NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Second)), // 允许 5 秒时钟偏差
		},
	}

	token := jwt.NewWithClaims(m.signingMethod, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

// GenerateRefreshToken 签发 refresh token
//
// refresh token 仅包含 subject (userID)，有效期更长。
// 用于后续调用 RefreshToken 换取新的 access token。
func (m *JWTManager) GenerateRefreshToken(userID string) (string, error) {
	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshDuration)),
			NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Second)),
		},
	}

	token := jwt.NewWithClaims(m.signingMethod, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign refresh token: %w", err)
	}
	return signed, nil
}

// ValidateToken 解析并验证 JWT token，返回 Claims
//
// 验证项: 签名算法 (HS256)、签名有效性、issuer、过期时间。
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return m.secret, nil
		},
		jwt.WithIssuer(m.issuer),
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// RefreshToken 验证 refresh token 并签发新的 access token
//
// 从 refresh token 中提取 userID (subject)，然后签发新的 access token。
// 注意: 新 token 仅包含 userID，其余字段 (tenantID, role 等) 需要从持久化存储中补全。
// 此处提供基础实现，调用方可在获取 Claims 后补充额外信息。
func (m *JWTManager) RefreshToken(refreshTokenString string) (string, error) {
	claims, err := m.ValidateToken(refreshTokenString)
	if err != nil {
		return "", fmt.Errorf("validate refresh token: %w", err)
	}

	// 使用 refresh token 中的 subject 作为 userID 签发新 access token
	// 调用方可通过 GenerateToken 生成包含完整信息的 token
	newToken, err := m.GenerateToken(
		claims.Subject,
		claims.TenantID,
		claims.Username,
		claims.Role,
		claims.ProjectID,
		claims.Namespaces,
		claims.IsPlatformAdmin,
	)
	if err != nil {
		return "", fmt.Errorf("generate new access token: %w", err)
	}

	return newToken, nil
}

// ==================== OIDC ====================

// OIDCConfig OIDC 提供商配置
type OIDCConfig struct {
	Issuer       string // OIDC Issuer URL (如 https://accounts.google.com)
	ClientID     string // OAuth2 Client ID
	ClientSecret string // OAuth2 Client Secret
	RedirectURL  string // 回调地址
	Scopes       string // 空格分隔的 scope 列表 (如 "openid profile email")
}

// OIDCProvider OIDC 认证提供者
type OIDCProvider struct {
	config   *OIDCConfig
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth2   *oauth2.Config
	httpClient *http.Client
}

// NewOIDCProvider 创建 OIDC 提供者，自动发现端点
//
// 通过 OIDC Discovery 协议自动获取 issuer、jwks_uri、authorization_endpoint 等元数据。
func NewOIDCProvider(config *OIDCConfig) (*OIDCProvider, error) {
	if config == nil || config.Issuer == "" {
		return nil, errors.New("oidc: issuer is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	provider, err := oidc.NewProvider(ctx, config.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc: discover provider: %w", err)
	}

	scopes := strings.Fields(config.Scopes)
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID}
	}

	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	return &OIDCProvider{
		config:     config,
		provider:   provider,
		verifier:   provider.Verifier(&oidc.Config{ClientID: config.ClientID}),
		oauth2:     oauth2Config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// ExchangeCode 使用授权码交换 token 并提取 Claims
//
// OAuth2 Authorization Code Flow 的最后一步: 用授权码换取 access_token / id_token，
// 然后从 id_token 中解析用户身份信息。
func (p *OIDCProvider) ExchangeCode(ctx context.Context, code string) (*Claims, error) {
	oauth2Token, err := p.oauth2.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oidc: exchange code: %w", err)
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, errors.New("oidc: no id_token in response")
	}

	return p.VerifyIDToken(ctx, rawIDToken)
}

// VerifyIDToken 验证 ID Token 并提取 Claims
//
// 验证签名、issuer、audience、过期时间等，然后从标准 OIDC claims 中提取用户信息。
func (p *OIDCProvider) VerifyIDToken(ctx context.Context, idToken string) (*Claims, error) {
	token, err := p.verifier.Verify(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("oidc: verify id_token: %w", err)
	}

	// 提取标准 OIDC claims
	var rawClaims struct {
		Subject    string   `json:"sub"`
		TenantID   string   `json:"tenant_id"`
		Username   string   `json:"username"`
		Preferred  string   `json:"preferred_username"`
		Email      string   `json:"email"`
		Role       string   `json:"role"`
		ProjectID  string   `json:"project_id"`
		Namespaces []string `json:"namespaces"`
	}
	if err := token.Claims(&rawClaims); err != nil {
		return nil, fmt.Errorf("oidc: extract claims: %w", err)
	}

	// 优先使用 username，回退到 preferred_username
	username := rawClaims.Username
	if username == "" {
		username = rawClaims.Preferred
	}
	if username == "" {
		username = rawClaims.Email
	}

	claims := &Claims{
		TenantID:   rawClaims.TenantID,
		Username:   username,
		Role:       rawClaims.Role,
		ProjectID:  rawClaims.ProjectID,
		Namespaces: rawClaims.Namespaces,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   rawClaims.Subject,
			Issuer:    token.Issuer,
			IssuedAt:  jwt.NewNumericDate(token.IssuedAt),
			ExpiresAt: jwt.NewNumericDate(token.Expiry),
		},
	}

	return claims, nil
}

// ==================== API Key ====================

// APIKeyInfo API Key 元信息
type APIKeyInfo struct {
	KeyHash   string `json:"key_hash"`
	KeyPrefix string `json:"key_prefix"` // 前 8 字符，用于展示
	UserID    string `json:"user_id"`
	TenantID  string `json:"tenant_id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
}

// APIKeyManager API Key 管理器
type APIKeyManager struct {
	keys sync.Map // apiKeyHash -> *APIKeyInfo
}

// NewAPIKeyManager 创建 API Key 管理器
func NewAPIKeyManager() *APIKeyManager {
	return &APIKeyManager{}
}

// apikeyPrefix API Key 前缀标识
const apikeyPrefix = "cfk_"

// GenerateAPIKey 生成 API Key
//
// 生成格式: "cfk_" + 32 字节随机 hex (共 69 字符)。
// 存储时仅保留 SHA-256 哈希，原始密钥仅在生成时返回一次。
func (m *APIKeyManager) GenerateAPIKey(userID, tenantID, name string, expiresIn time.Duration) (string, error) {
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return "", fmt.Errorf("generate api key: read random: %w", err)
	}
	rawKey := apikeyPrefix + hex.EncodeToString(rawBytes)

	// 计算 SHA-256 哈希用于存储
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	// 前缀用于展示 (cfk_ + 前 4 字节 hex)
	prefixLen := len(apikeyPrefix) + 8
	if len(rawKey) < prefixLen {
		prefixLen = len(rawKey)
	}

	now := time.Now()
	info := &APIKeyInfo{
		KeyHash:   keyHash,
		KeyPrefix: rawKey[:prefixLen],
		UserID:    userID,
		TenantID:  tenantID,
		Name:      name,
		CreatedAt: now.Unix(),
		ExpiresAt: now.Add(expiresIn).Unix(),
	}

	m.keys.Store(keyHash, info)
	return rawKey, nil
}

// ValidateAPIKey 验证 API Key
//
// 对输入 key 计算 SHA-256 哈希后查找，并检查是否过期。
func (m *APIKeyManager) ValidateAPIKey(apiKey string) (*APIKeyInfo, error) {
	if !strings.HasPrefix(apiKey, apikeyPrefix) {
		return nil, errors.New("api key: invalid prefix")
	}

	hash := sha256.Sum256([]byte(apiKey))
	keyHash := hex.EncodeToString(hash[:])

	val, ok := m.keys.Load(keyHash)
	if !ok {
		return nil, errors.New("api key: not found")
	}

	info, ok := val.(*APIKeyInfo)
	if !ok {
		return nil, errors.New("api key: invalid stored data")
	}

	// 检查过期
	if info.ExpiresAt > 0 && time.Now().Unix() > info.ExpiresAt {
		m.keys.Delete(keyHash)
		return nil, errors.New("api key: expired")
	}

	return info, nil
}

// RevokeAPIKey 撤销 API Key
func (m *APIKeyManager) RevokeAPIKey(apiKey string) error {
	if !strings.HasPrefix(apiKey, apikeyPrefix) {
		return errors.New("api key: invalid prefix")
	}

	hash := sha256.Sum256([]byte(apiKey))
	keyHash := hex.EncodeToString(hash[:])

	if _, ok := m.keys.Load(keyHash); !ok {
		return errors.New("api key: not found")
	}

	m.keys.Delete(keyHash)
	return nil
}

// ==================== Authenticator (统一认证入口) ====================

// Authenticator 统一认证入口，自动识别认证方式
//
// 支持三种认证方式:
//   - API Key (以 "cfk_" 开头)
//   - OIDC ID Token (当 OIDC provider 已配置且 token 为三段式时优先尝试)
//   - JWT (默认)
type Authenticator struct {
	jwtManager   *JWTManager
	oidcProvider *OIDCProvider // 可选，为 nil 时不启用 OIDC
	apiKeyManager *APIKeyManager
}

// NewAuthenticator 创建统一认证器
//
//   - jwtSecret, jwtIssuer: JWT 签名配置
//   - jwtExpireSec, jwtRefreshSec: token 有效期 (秒)
//   - oidcConfig: OIDC 配置，传 nil 则不启用 OIDC
func NewAuthenticator(jwtSecret, jwtIssuer string, jwtExpireSec, jwtRefreshSec int64, oidcConfig *OIDCConfig) *Authenticator {
	a := &Authenticator{
		jwtManager:    NewJWTManager(jwtSecret, jwtIssuer, jwtExpireSec, jwtRefreshSec),
		apiKeyManager: NewAPIKeyManager(),
	}

	if oidcConfig != nil && oidcConfig.Issuer != "" {
		if provider, err := NewOIDCProvider(oidcConfig); err == nil {
			a.oidcProvider = provider
		}
	}

	return a
}

// Authenticate 统一认证方法，自动检测认证方式
//
// 检测逻辑:
//  1. token 以 "cfk_" 开头 → API Key 认证
//  2. OIDC provider 已配置且 token 为三段式 (xxx.yyy.zzz) → 先尝试 OIDC，失败则回退 JWT
//  3. 其他 → JWT 认证
func (a *Authenticator) Authenticate(ctx context.Context, token string) (*Claims, error) {
	// 1. API Key 认证
	if strings.HasPrefix(token, apikeyPrefix) {
		info, err := a.apiKeyManager.ValidateAPIKey(token)
		if err != nil {
			return nil, fmt.Errorf("api key auth: %w", err)
		}

		// 从 API Key 信息构造 Claims
		return &Claims{
			TenantID: info.TenantID,
			Username: info.Name,
			Role:     "api_key",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   info.UserID,
				Issuer:    "cloudflow",
				IssuedAt:  jwt.NewNumericDate(time.Unix(info.CreatedAt, 0)),
				ExpiresAt: jwt.NewNumericDate(time.Unix(info.ExpiresAt, 0)),
			},
		}, nil
	}

	// 2. OIDC 认证 (如果已配置且 token 为三段式)
	if a.oidcProvider != nil && isThreePartToken(token) {
		claims, err := a.oidcProvider.VerifyIDToken(ctx, token)
		if err == nil {
			return claims, nil
		}
		// OIDC 验证失败，回退到 JWT
	}

	// 3. JWT 认证
	claims, err := a.jwtManager.ValidateToken(token)
	if err != nil {
		return nil, fmt.Errorf("jwt auth: %w", err)
	}

	return claims, nil
}

// JWTManager 暴露 JWT 管理器，用于直接 JWT 操作 (签发、刷新等)
func (a *Authenticator) JWTManager() *JWTManager {
	return a.jwtManager
}

// isThreePartToken 检查 token 是否为三段式格式 (xxx.yyy.zzz)
func isThreePartToken(token string) bool {
	parts := strings.Split(token, ".")
	return len(parts) == 3
}
