//go:build linux

// Package main - Cloud Flow Edge 无状态服务
//
// 特性：
// - 完全无状态，所有数据存储在Redis
// - 支持多实例运行，自动负载均衡
// - gRPC健康检查接口
// - 配置全部从环境变量读取
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"cloud-flow-edge/edge/internal/balancer"
	"cloud-flow-edge/edge/internal/cache"
	"cloud-flow-edge/edge/internal/config"
	"cloud-flow-edge/edge/internal/health"
	"cloud-flow-agent/pkg/logger"
)

func main() {
	// 创建根上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 加载配置
	cfg, err := config.LoadFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	log := logger.NewLogger(cfg.Log.Level, cfg.Log.Format)
	log.Info("Cloud Flow Edge starting...",
		"instance", cfg.GetInstanceKey(),
		"region", cfg.Region,
		"zone", cfg.Zone,
	)

	// 初始化Redis缓存
	var redisCache *cache.RedisCache
	if cfg.Redis.Enabled {
		redisCache, err = cache.NewRedisCache(&cfg.Redis)
		if err != nil {
			log.Error("Failed to connect to Redis", "error", err)
			os.Exit(1)
		}
		defer redisCache.Close()
		log.Info("Redis cache initialized", "address", cfg.Redis.Address)
	}

	// 初始化负载均衡器
	lb := balancer.NewLoadBalancer(&cfg.LoadBalancer, redisCache)
	if err := lb.Start(ctx); err != nil {
		log.Error("Failed to start load balancer", "error", err)
		os.Exit(1)
	}
	defer lb.Stop()

	// 注册实例到Redis
	if redisCache != nil {
		instanceID := cfg.GetInstanceKey()
		endpoint := cfg.GetGRPCAddress()
		if err := lb.RegisterInstance(ctx, instanceID, endpoint); err != nil {
			log.Error("Failed to register instance", "error", err)
		}
		defer lb.UnregisterInstance(context.Background(), instanceID)
		log.Info("Instance registered", "id", instanceID, "endpoint", endpoint)
	}

	// 启动gRPC服务
	grpcServer, err := startGRPCServer(cfg, log)
	if err != nil {
		log.Error("Failed to start gRPC server", "error", err)
		os.Exit(1)
	}
	defer grpcServer.GracefulStop()
	log.Info("gRPC server started", "address", cfg.GetGRPCAddress())

	// 启动健康检查服务
	healthServer := health.NewHealthServer(&cfg.Health, redisCache)
	
	// 注册健康检查器
	if redisCache != nil {
		healthServer.RegisterChecker(health.NewRedisHealthChecker(redisCache))
	}
	healthServer.RegisterChecker(health.NewCenterHealthChecker(cfg.Center.Address))
	
	// 启动gRPC健康检查
	if err := healthServer.StartGRPC(fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Health.GRPCPort)); err != nil {
		log.Error("Failed to start gRPC health server", "error", err)
		os.Exit(1)
	}
	
	// 启动HTTP健康检查
	if err := healthServer.StartHTTP(cfg.GetHealthAddress()); err != nil {
		log.Error("Failed to start HTTP health server", "error", err)
		os.Exit(1)
	}
	defer healthServer.Stop(context.Background())
	log.Info("Health check server started", 
		"grpc_port", cfg.Health.GRPCPort,
		"http_port", cfg.Health.Port,
	)

	// 启动HTTP API服务
	httpServer, err := startHTTPServer(cfg, log, lb)
	if err != nil {
		log.Error("Failed to start HTTP server", "error", err)
		os.Exit(1)
	}
	defer httpServer.Shutdown(context.Background())
	log.Info("HTTP server started", "address", cfg.GetServerAddress())

	// 设置就绪状态
	healthServer.SetReady(true)
	log.Info("Cloud Flow Edge is ready")

	// 启动心跳循环
	if redisCache != nil {
		go heartbeatLoop(ctx, lb, cfg.GetInstanceKey(), cfg.Center.HeartbeatInterval, log)
	}

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Info("Received signal, shutting down...", "signal", sig)
	case <-ctx.Done():
		log.Info("Context cancelled, shutting down...")
	}

	// 优雅关闭
	shutdown(ctx, healthServer, grpcServer, httpServer, lb, cfg.GetInstanceKey(), log)
	log.Info("Cloud Flow Edge stopped")
}

// startGRPCServer 启动gRPC服务
func startGRPCServer(cfg *config.Config, log *logger.Logger) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", cfg.GetGRPCAddress())
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(cfg.GRPC.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(cfg.GRPC.MaxSendMsgSize),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    cfg.GRPC.KeepaliveTime,
			Timeout: cfg.GRPC.KeepaliveTimeout,
		}),
	}

	server := grpc.NewServer(opts...)

	// 注册gRPC反射服务（用于调试）
	reflection.Register(server)

	// TODO: 注册业务服务
	// pb.RegisterEdgeServiceServer(server, edgeService)

	go func() {
		if err := server.Serve(lis); err != nil {
			log.Error("gRPC server error", "error", err)
		}
	}()

	return server, nil
}

// startHTTPServer 启动HTTP服务
func startHTTPServer(cfg *config.Config, log *logger.Logger, lb *balancer.LoadBalancer) (*http.Server, error) {
	mux := http.NewServeMux()

	// 实例信息
	mux.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"instance_id": cfg.GetInstanceKey(),
			"instance_name": cfg.InstanceName,
			"region":      cfg.Region,
			"zone":        cfg.Zone,
			"grpc_address": cfg.GetGRPCAddress(),
			"http_address": cfg.GetServerAddress(),
			"timestamp":   time.Now().UTC(),
		})
	})

	// 实例列表
	mux.HandleFunc("/instances", func(w http.ResponseWriter, r *http.Request) {
		instances := lb.GetAllInstances()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"instances": instances,
			"count":     len(instances),
		})
	})

	// Agent连接端点（用于负载均衡）
	mux.HandleFunc("/connect", func(w http.ResponseWriter, r *http.Request) {
		agentID := r.URL.Query().Get("agent_id")
		if agentID == "" {
			http.Error(w, "agent_id required", http.StatusBadRequest)
			return
		}

		instance, err := lb.SelectInstance(agentID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"agent_id":    agentID,
			"instance_id": instance.ID,
			"endpoint":    instance.Endpoint,
		})
	})

	server := &http.Server{
		Addr:         cfg.GetServerAddress(),
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", "error", err)
		}
	}()

	return server, nil
}

// heartbeatLoop 心跳循环
func heartbeatLoop(ctx context.Context, lb *balancer.LoadBalancer, instanceID string, interval time.Duration, log *logger.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := lb.Heartbeat(ctx, instanceID); err != nil {
				log.Warn("Heartbeat failed", "error", err)
			}
		}
	}
}

// shutdown 优雅关闭
func shutdown(ctx context.Context, healthServer *health.HealthServer, grpcServer *grpc.Server, httpServer *http.Server, lb *balancer.LoadBalancer, instanceID string, log *logger.Logger) {
	// 设置未就绪状态
	if healthServer != nil {
		healthServer.SetReady(false)
	}

	// 注销实例
	if lb != nil && instanceID != "" {
		if err := lb.UnregisterInstance(context.Background(), instanceID); err != nil {
			log.Warn("Failed to unregister instance", "error", err)
		}
	}

	// 创建带超时的关闭上下文
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 关闭HTTP服务
	if httpServer != nil {
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Warn("HTTP server shutdown error", "error", err)
		}
	}

	// 关闭健康检查服务
	if healthServer != nil {
		if err := healthServer.Stop(shutdownCtx); err != nil {
			log.Warn("Health server shutdown error", "error", err)
		}
	}

	// 关闭gRPC服务
	if grpcServer != nil {
		grpcServer.GracefulStop()
	}

	// 停止负载均衡器
	if lb != nil {
		lb.Stop()
	}

	log.Info("Shutdown completed")
}

// writeJSON 写入JSON响应
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	// 忽略编码错误
	_ = json.NewEncoder(w).Encode(v)
}
