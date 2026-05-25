//go:build linux

// Cloud Flow Center - 完全无状态中心控制服务
// 所有数据存储在 TiDB/Redis，支持多实例部署
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud-flow-center/internal/api"
	"cloud-flow-center/internal/cache"
	"cloud-flow-center/internal/config"
	"cloud-flow-center/internal/health"
	"cloud-flow-center/internal/storage"
	"cloud-flow-agent/pkg/logger"
)

var log = logger.NewLogger("center")

func main() {
	log.Info("===========================================")
	log.Info("Cloud Flow Center - 无状态中心控制服务")
	log.Info("===========================================")

	// 1. 加载配置（从环境变量）
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}
	log.Infof("配置加载成功: ID=%s, Mode=%s", cfg.CenterID, cfg.Mode)

	// 2. 初始化日志
	if err := logger.InitLogger(cfg.LogLevel, cfg.LogFormat); err != nil {
		fmt.Fprintf(os.Stderr, "日志初始化失败: %v\n", err)
		os.Exit(1)
	}

	// 3. 初始化存储层（TiDB/PostgreSQL）
	store, err := storage.NewStore(&cfg.Database, log)
	if err != nil {
		log.Fatalf("存储层初始化失败: %v", err)
	}
	defer store.Close()
	log.Info("存储层初始化成功")

	// 4. 初始化缓存层（Redis）
	redisCache, err := cache.NewRedisCache(&cfg.Redis, log)
	if err != nil {
		log.Fatalf("缓存层初始化失败: %v", err)
	}
	defer redisCache.Close()
	log.Info("缓存层初始化成功")

	// 5. 创建无状态存储管理器
	storeManager := storage.NewStatelessManager(store, redisCache, log)
	log.Info("无状态存储管理器初始化成功")

	// 6. 初始化健康检查
	healthServer := health.NewHealthServer(health.Config{
		Store:  store,
		Cache:  redisCache,
		LivenessCheck: health.LivenessConfig{
			Endpoint: "/health",
			Timeout:  5 * time.Second,
		},
		ReadinessCheck: health.ReadinessConfig{
			Endpoint: "/ready",
			Timeout:  10 * time.Second,
		},
	}, log)

	// 7. 初始化API服务
	apiServer := api.NewServer(api.Config{
		Addr:           cfg.Addr,
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
		IdleTimeout:    cfg.IdleTimeout,
		MaxHeaderBytes: cfg.MaxHeaderBytes,
	}, storeManager, redisCache, log)

	// 8. 启动服务
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动健康检查服务器
	go func() {
		addr := fmt.Sprintf(":%d", cfg.HealthPort)
		log.Infof("健康检查服务器启动: %s", addr)
		if err := healthServer.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Errorf("健康检查服务器错误: %v", err)
		}
	}()

	// 启动API服务器
	go func() {
		log.Infof("API服务器启动: %s", cfg.Addr)
		if err := apiServer.Start(); err != nil && err != http.ErrServerClosed {
			log.Errorf("API服务器错误: %v", err)
		}
	}()

	// 9. 等待信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	
	sig := <-sigCh
	log.Infof("收到信号: %v, 开始关闭...", sig)

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// 关闭API服务器
	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		log.Errorf("API服务器关闭错误: %v", err)
	}

	// 关闭健康检查服务器
	if err := healthServer.Stop(shutdownCtx); err != nil {
		log.Errorf("健康检查服务器关闭错误: %v", err)
	}

	// 关闭存储和缓存
	store.Close()
	redisCache.Close()

	log.Info("服务已正常关闭")
}
