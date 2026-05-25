//go:build linux

// Package nacos 提供 Nacos 配置中心客户端
// 支持配置热更新、环境隔离、动态监听
package nacos

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// NacosConfig Nacos 客户端配置
type NacosConfig struct {
	// 服务端地址
	ServerAddr string // NACOS_SERVER_ADDR (默认: localhost:8848)
	Namespace  string // NACOS_NAMESPACE (默认: public)
	Group      string // NACOS_GROUP (默认: DEFAULT_GROUP)

	// 认证
	Username string // NACOS_USERNAME
	Password string // NACOS_PASSWORD
	AccessKey string // NACOS_ACCESS_KEY
	SecretKey string // NACOS_SECRET_KEY

	// 连接配置
	TimeoutMs   uint64 // NACOS_TIMEOUT_MS (默认: 10000)
	BeatInterval int64 // NACOS_BEAT_INTERVAL (默认: 5000)
	CacheDir    string // NACOS_CACHE_DIR (默认: /tmp/nacos)
	LogDir      string // NACOS_LOG_DIR (默认: /tmp/nacos/log)
	LogLevel    string // NACOS_LOG_LEVEL (默认: info)

	// 应用配置
	AppName     string // NACOS_APP_NAME
	AppEnv      string // NACOS_APP_ENV (dev/test/prod)
	AppVersion  string // NACOS_APP_VERSION
}

// Client Nacos 配置客户端
type Client struct {
	config       *NacosConfig
	client       config_client.IConfigClient
	listeners    map[string][]ConfigListener
	listenersMu  sync.RWMutex
	cache        map[string]string
	cacheMu      sync.RWMutex
	watchedKeys  map[string]bool
	watchedMu    sync.Mutex
}

// ConfigListener 配置变更监听器
type ConfigListener func(dataId, group, content string)

// NewClient 创建 Nacos 客户端
func NewClient(cfg *NacosConfig) (*Client, error) {
	if cfg == nil {
		cfg = DefaultNacosConfig()
	}

	// 解析服务端地址
	serverConfigs := parseServerAddr(cfg.ServerAddr)

	// 客户端配置
	clientConfig := constant.ClientConfig{
		NamespaceId:         cfg.Namespace,
		TimeoutMs:           cfg.TimeoutMs,
		NotLoadCacheAtStart: true,
		UpdateCacheWhenEmpty: true,
		LogDir:              cfg.LogDir,
		CacheDir:            cfg.CacheDir,
		LogLevel:            cfg.LogLevel,
		BeatInterval:        cfg.BeatInterval,
	}

	// 认证配置
	if cfg.Username != "" {
		clientConfig.Username = cfg.Username
		clientConfig.Password = cfg.Password
	}
	if cfg.AccessKey != "" {
		clientConfig.AccessKey = cfg.AccessKey
		clientConfig.SecretKey = cfg.SecretKey
	}

	// 创建配置客户端
	configClient, err := clients.NewConfigClient(
		vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create nacos client: %w", err)
	}

	c := &Client{
		config:      cfg,
		client:      configClient,
		listeners:   make(map[string][]ConfigListener),
		cache:       make(map[string]string),
		watchedKeys: make(map[string]bool),
	}

	return c, nil
}

// DefaultNacosConfig 默认 Nacos 配置
func DefaultNacosConfig() *NacosConfig {
	return &NacosConfig{
		ServerAddr:   getEnv("NACOS_SERVER_ADDR", "localhost:8848"),
		Namespace:    getEnv("NACOS_NAMESPACE", "public"),
		Group:        getEnv("NACOS_GROUP", "DEFAULT_GROUP"),
		Username:     getEnv("NACOS_USERNAME", ""),
		Password:     getEnv("NACOS_PASSWORD", ""),
		AccessKey:    getEnv("NACOS_ACCESS_KEY", ""),
		SecretKey:    getEnv("NACOS_SECRET_KEY", ""),
		TimeoutMs:    getEnvUint64("NACOS_TIMEOUT_MS", 10000),
		BeatInterval: getEnvInt64("NACOS_BEAT_INTERVAL", 5000),
		CacheDir:     getEnv("NACOS_CACHE_DIR", "/tmp/nacos"),
		LogDir:       getEnv("NACOS_LOG_DIR", "/tmp/nacos/log"),
		LogLevel:     getEnv("NACOS_LOG_LEVEL", "info"),
		AppName:      getEnv("NACOS_APP_NAME", "cloud-flow"),
		AppEnv:       getEnv("NACOS_APP_ENV", "dev"),
		AppVersion:   getEnv("NACOS_APP_VERSION", "1.0.0"),
	}
}

// ==================== 配置读取 ====================

// GetConfig 获取配置
func (c *Client) GetConfig(dataId string) (string, error) {
	// 先检查缓存
	c.cacheMu.RLock()
	if content, ok := c.cache[dataId]; ok {
		c.cacheMu.RUnlock()
		return content, nil
	}
	c.cacheMu.RUnlock()

	// 从 Nacos 获取
	content, err := c.client.GetConfig(vo.ConfigParam{
		DataId: dataId,
		Group:  c.config.Group,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get config %s: %w", dataId, err)
	}

	// 更新缓存
	c.cacheMu.Lock()
	c.cache[dataId] = content
	c.cacheMu.Unlock()

	return content, nil
}

// GetConfigJSON 获取 JSON 配置并解析
func (c *Client) GetConfigJSON(dataId string, v interface{}) error {
	content, err := c.GetConfig(dataId)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(content), v); err != nil {
		return fmt.Errorf("failed to unmarshal config %s: %w", dataId, err)
	}

	return nil
}

// GetConfigYAML 获取 YAML 配置（简化实现，实际需要 YAML 解析库）
func (c *Client) GetConfigYAML(dataId string) (map[string]string, error) {
	content, err := c.GetConfig(dataId)
	if err != nil {
		return nil, err
	}

	// 简单的 key=value 解析
	result := make(map[string]string)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// 去除引号
			value = strings.Trim(value, `"'`)
			result[key] = value
		}
	}

	return result, nil
}

// ==================== 配置监听（热更新）====================

// ListenConfig 监听配置变更
func (c *Client) ListenConfig(dataId string, listener ConfigListener) error {
	c.listenersMu.Lock()
	c.listeners[dataId] = append(c.listeners[dataId], listener)
	c.listenersMu.Unlock()

	// 检查是否已注册监听
	c.watchedMu.Lock()
	if c.watchedKeys[dataId] {
		c.watchedMu.Unlock()
		return nil
	}
	c.watchedKeys[dataId] = true
	c.watchedMu.Unlock()

	// 注册 Nacos 监听
	err := c.client.ListenConfig(vo.ConfigParam{
		DataId: dataId,
		Group:  c.config.Group,
		OnChange: func(namespace, group, dataId, data string) {
			c.handleConfigChange(dataId, group, data)
		},
	})

	if err != nil {
		// 回滚
		c.watchedMu.Lock()
		delete(c.watchedKeys, dataId)
		c.watchedMu.Unlock()
		return fmt.Errorf("failed to listen config %s: %w", dataId, err)
	}

	return nil
}

// ListenConfigs 批量监听多个配置
func (c *Client) ListenConfigs(dataIds []string, listener ConfigListener) error {
	for _, dataId := range dataIds {
		if err := c.ListenConfig(dataId, listener); err != nil {
			return err
		}
	}
	return nil
}

// handleConfigChange 处理配置变更
func (c *Client) handleConfigChange(dataId, group, content string) {
	// 更新缓存
	c.cacheMu.Lock()
	c.cache[dataId] = content
	c.cacheMu.Unlock()

	// 通知监听器
	c.listenersMu.RLock()
	listeners := c.listeners[dataId]
	c.listenersMu.RUnlock()

	for _, listener := range listeners {
		go listener(dataId, group, content)
	}
}

// StopListenConfig 停止监听配置
func (c *Client) StopListenConfig(dataId string) error {
	c.watchedMu.Lock()
	if !c.watchedKeys[dataId] {
		c.watchedMu.Unlock()
		return nil
	}
	delete(c.watchedKeys, dataId)
	c.watchedMu.Unlock()

	c.listenersMu.Lock()
	delete(c.listeners, dataId)
	c.listenersMu.Unlock()

	return c.client.CancelListenConfig(vo.ConfigParam{
		DataId: dataId,
		Group:  c.config.Group,
	})
}

// ==================== 配置发布 ====================

// PublishConfig 发布配置
func (c *Client) PublishConfig(dataId, content string) error {
	_, err := c.client.PublishConfig(vo.ConfigParam{
		DataId:  dataId,
		Group:   c.config.Group,
		Content: content,
	})
	return err
}

// PublishConfigJSON 发布 JSON 配置
func (c *Client) PublishConfigJSON(dataId string, v interface{}) error {
	content, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return c.PublishConfig(dataId, string(content))
}

// DeleteConfig 删除配置
func (c *Client) DeleteConfig(dataId string) error {
	_, err := c.client.DeleteConfig(vo.ConfigParam{
		DataId: dataId,
		Group:  c.config.Group,
	})
	return err
}

// ==================== 配置搜索 ====================

// SearchConfig 搜索配置
func (c *Client) SearchConfig(dataIdPattern string) ([]vo.ConfigItem, error) {
	page, err := c.client.SearchConfig(vo.SearchConfigParam{
		Search:   "blur",
		DataId:   dataIdPattern,
		Group:    c.config.Group,
		PageNo:   1,
		PageSize: 100,
	})
	if err != nil {
		return nil, err
	}
	return page.PageItems, nil
}

// ==================== 获取配置列表 ====================

// GetConfigKeys 获取所有配置 key
func (c *Client) GetConfigKeys() ([]string, error) {
	items, err := c.SearchConfig("*")
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(items))
	for _, item := range items {
		keys = append(keys, item.DataId)
	}
	return keys, nil
}

// ==================== 辅助方法 ====================

// GetCache 获取缓存的配置
func (c *Client) GetCache(dataId string) (string, bool) {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	content, ok := c.cache[dataId]
	return content, ok
}

// ClearCache 清空缓存
func (c *Client) ClearCache() {
	c.cacheMu.Lock()
	c.cache = make(map[string]string)
	c.cacheMu.Unlock()
}

// GetConfigInfo 获取配置信息
func (c *Client) GetConfigInfo() *NacosConfig {
	return c.config
}

// Close 关闭客户端
func (c *Client) Close() {
	// 停止所有监听
	c.watchedMu.Lock()
	for dataId := range c.watchedKeys {
		_ = c.client.CancelListenConfig(vo.ConfigParam{
			DataId: dataId,
			Group:  c.config.Group,
		})
	}
	c.watchedKeys = make(map[string]bool)
	c.watchedMu.Unlock()
}

// ==================== 工具函数 ====================

func parseServerAddr(addr string) []constant.ServerConfig {
	parts := strings.Split(addr, ",")
	configs := make([]constant.ServerConfig, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		hostPort := strings.Split(part, ":")
		if len(hostPort) != 2 {
			continue
		}

		port, _ := strconv.ParseUint(hostPort[1], 10, 64)
		configs = append(configs, constant.ServerConfig{
			IpAddr: hostPort[0],
			Port:   port,
		})
	}

	return configs
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvUint64(key string, defaultVal uint64) uint64 {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.ParseUint(val, 10, 64); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func getEnvInt64(key string, defaultVal int64) int64 {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.ParseInt(val, 10, 64); err == nil {
			return intVal
		}
	}
	return defaultVal
}
