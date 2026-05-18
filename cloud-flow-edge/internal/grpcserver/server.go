// Package grpcserver 提供 gRPC 服务端实现
// 实现 ProbeService 接口，接收探针的注册、心跳、数据上报请求
package grpcserver

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	"cloud-flow-edge/internal/config"
	"cloud-flow-edge/internal/forwarder"
	"cloud-flow-edge/internal/probemgr"
	"cloud-flow-edge/pkg/logger"
	"cloud-flow-edge/pkg/metrics"
	"cloud-flow-edge/pkg/tlsutil"
	"cloud-flow-edge/pkg/validate"
	"cloud-flow/pkg/grpcutil"
	"cloud-flow/pkg/ratelimit"
	"cloud-flow/pkg/trace"
	edge "cloud-flow/proto"
)

// Server gRPC 服务端
// 实现 edge.ProbeServiceServer 接口
type Server struct {
	edge.UnimplementedProbeServiceServer
	manager   *probemgr.Manager
	forwarder *forwarder.Forwarder
	logger    *logger.Logger
	metrics   *metrics.Metrics
	apiKey    string
}

// NewServer 创建 gRPC 服务端实例
func NewServer(manager *probemgr.Manager, fwd *forwarder.Forwarder, log *logger.Logger, metrics *metrics.Metrics, apiKey string) *Server {
	return &Server{
		manager:   manager,
		forwarder: fwd,
		logger:    log,
		metrics:   metrics,
		apiKey:    apiKey,
	}
}



// TraceIDInterceptor Trace ID 拦截器
func TraceIDInterceptor(log *logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 从 metadata 中提取 Trace ID
		md, ok := metadata.FromIncomingContext(ctx)
		var traceID string
		if ok {
			vals := md.Get(trace.TraceIDMDKey)
			if len(vals) > 0 {
				traceID = vals[0]
			}
		}

		// 如果没有 Trace ID，生成一个新的
		if traceID == "" {
			traceID = trace.TraceID()
		}

		// 将 Trace ID 添加到 context 中
		ctx = trace.WithTraceID(ctx, traceID)

		// 继续处理请求
		return handler(ctx, req)
	}
}

// APIKeyAuthInterceptor API Key 认证拦截器
func APIKeyAuthInterceptor(apiKey string, log *logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if apiKey == "" {
			// 未配置 API Key，直接放行
			return handler(ctx, req)
		}

		// 从 metadata 中获取 API Key
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			log.Warnf("API Key 认证失败: 无 metadata")
			return nil, fmt.Errorf("API Key 认证失败")
		}

		vals := md.Get("x-api-key")
		if len(vals) == 0 {
			log.Warnf("API Key 认证失败: 无 x-api-key 头")
			return nil, fmt.Errorf("API Key 认证失败")
		}

		if subtle.ConstantTimeCompare([]byte(vals[0]), []byte(apiKey)) != 1 {
			log.Warnf("API Key 认证失败: 无效的 API Key")
			return nil, fmt.Errorf("API Key 认证失败")
		}

		// 认证通过，继续处理请求
		return handler(ctx, req)
	}
}

// BuildServerOpts 根据配置构建 gRPC Server 选项
func BuildServerOpts(tlsCfg config.TLSConfig, rateLimit config.RateLimitConfig, apiKey string, log *logger.Logger) ([]grpc.ServerOption, error) {
	var opts []grpc.ServerOption

	// 添加 Trace ID 拦截器
	traceInterceptor := TraceIDInterceptor(log)
	
	// 添加限流拦截器
	var rateLimiter *ratelimit.TokenBucket
	if rateLimit.Enabled {
		rateLimiter = ratelimit.NewTokenBucket(rateLimit.BucketSize, rateLimit.RefillRate)
		log.Infof("gRPC 服务端启用速率限制: 桶容量=%d, 填充速率=%d/秒", rateLimit.BucketSize, rateLimit.RefillRate)
	} else {
		log.Info("gRPC 服务端未启用速率限制")
	}
	
	// 添加 API Key 认证拦截器
	if apiKey != "" {
		// 组合拦截器
		combinedInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			// 恢复 panic，防止服务崩溃
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("gRPC 处理 panic: %v", r)
				}
			}()
			// 先处理限流
			if rateLimit.Enabled && !rateLimiter.Allow() {
				return nil, fmt.Errorf("rate limit exceeded")
			}
			// 再处理 Trace ID
			return traceInterceptor(ctx, req, info, func(ctx context.Context, req interface{}) (interface{}, error) {
				// 最后处理 API Key 认证
				apiInterceptor := APIKeyAuthInterceptor(apiKey, log)
				return apiInterceptor(ctx, req, info, handler)
			})
		}
		opts = append(opts, grpc.UnaryInterceptor(combinedInterceptor))
		log.Info("gRPC 服务端启用 API Key 认证、Trace ID 和速率限制")
	} else {
		// 组合 Trace ID 和限流拦截器
		combinedInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			// 恢复 panic，防止服务崩溃
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("gRPC 处理 panic: %v", r)
				}
			}()
			// 先处理限流
			if rateLimit.Enabled && !rateLimiter.Allow() {
				return nil, fmt.Errorf("rate limit exceeded")
			}
			// 再处理 Trace ID
			return traceInterceptor(ctx, req, info, handler)
		}
		opts = append(opts, grpc.UnaryInterceptor(combinedInterceptor))
		log.Info("gRPC 服务端启用 Trace ID 和速率限制")
	}

	if !tlsCfg.Enabled {
		log.Warn("gRPC 服务端未启用 TLS，将使用明文传输")
		return opts, nil
	}

	// 如果未提供证书路径，自动生成自签名证书
	if tlsCfg.ServerCert == "" || tlsCfg.ServerKey == "" {
		log.Info("未配置服务端证书，将自动生成自签名证书")
		cert, err := tlsutil.LoadOrGenSelfSignedCert("", "", "./certs")
		if err != nil {
			return nil, fmt.Errorf("生成自签名证书失败: %w", err)
		}
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{*cert},
			ClientAuth:   tls.NoClientCert,
		}
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.Creds(creds))
		return opts, nil
	}

	// 加载服务端证书和私钥
	cert, err := tlsutil.LoadOrGenSelfSignedCert(tlsCfg.ServerCert, tlsCfg.ServerKey, "")
	if err != nil {
		return nil, fmt.Errorf("加载服务端证书失败: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		ClientAuth:   tls.NoClientCert,
	}

	// 如果配置了 CA 证书，启用客户端证书验证（mTLS）
	if tlsCfg.CACert != "" {
		caPEM, err := os.ReadFile(tlsCfg.CACert)
		if err != nil {
			return nil, fmt.Errorf("读取 CA 证书失败: %w", err)
		}
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("解析 CA 证书失败")
		}
		tlsConfig.ClientCAs = certPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		log.Info("gRPC 服务端启用 mTLS")
	} else {
		log.Info("gRPC 服务端启用 TLS（单向）")
	}

	creds := credentials.NewTLS(tlsConfig)
	opts = append(opts, grpc.Creds(creds))
	return opts, nil
}

// RegisterProbe 处理探针注册请求
func (s *Server) RegisterProbe(ctx context.Context, req *edge.RegisterProbeRequest) (*edge.RegisterProbeResponse, error) {
	if !grpcutil.CheckAPIKey(ctx, s.apiKey) {
		s.logger.Warnf("探针注册被拒绝: API Key 验证失败, probeID=%s", req.GetProbeId())
		return &edge.RegisterProbeResponse{Success: false, Message: "API Key 验证失败"}, nil
	}
	if err := validate.RegisterProbeRequest(req); err != nil {
		s.logger.Warnf("探针注册被拒绝: 参数校验失败: %v", err)
		return &edge.RegisterProbeResponse{Success: false, Message: err.Error()}, nil
	}
	s.manager.Register(req.GetProbeId(), req.GetHostIp(), req.GetHostname(), req.GetVersion())
	return &edge.RegisterProbeResponse{Success: true, Message: "注册成功", HeartbeatInterval: 30}, nil
}

// Heartbeat 处理探针心跳请求
func (s *Server) Heartbeat(ctx context.Context, req *edge.HeartbeatRequest) (*edge.HeartbeatResponse, error) {
	if !grpcutil.CheckAPIKey(ctx, s.apiKey) {
		return &edge.HeartbeatResponse{Success: false}, nil
	}
	if err := validate.HeartbeatRequest(req); err != nil {
		return &edge.HeartbeatResponse{Success: false}, nil
	}
	ok := s.manager.Heartbeat(req.GetProbeId())
	return &edge.HeartbeatResponse{Success: ok}, nil
}

// SendMetrics 接收探针发送的指标数据
func (s *Server) SendMetrics(ctx context.Context, batch *edge.MetricsBatch) (*edge.SendResponse, error) {
	if !grpcutil.CheckAPIKey(ctx, s.apiKey) {
		s.logger.Warnf("指标数据被拒绝: API Key 验证失败, probeID=%s", batch.GetProbeId())
		if s.metrics != nil {
			s.metrics.Receive(0, fmt.Errorf("API Key 验证失败"))
		}
		return &edge.SendResponse{Success: false, Accepted: 0}, nil
	}
	if err := validate.MetricsBatch(batch); err != nil {
		s.logger.Warnf("指标数据被拒绝: 参数校验失败: %v", err)
		if s.metrics != nil {
			s.metrics.Receive(0, err)
		}
		return &edge.SendResponse{Success: false, Accepted: 0}, nil
	}
	if s.metrics != nil {
		s.metrics.Receive(len(batch.GetMetrics())*100, nil) // 估算字节数
	}
	s.forwarder.AddMetrics(batch)
	return &edge.SendResponse{Success: true, Accepted: int32(len(batch.GetMetrics()))}, nil
}

// SendTraces 接收探针发送的链路追踪数据
func (s *Server) SendTraces(ctx context.Context, batch *edge.TraceBatch) (*edge.SendResponse, error) {
	if !grpcutil.CheckAPIKey(ctx, s.apiKey) {
		s.logger.Warnf("链路追踪数据被拒绝: API Key 验证失败, probeID=%s", batch.GetProbeId())
		if s.metrics != nil {
			s.metrics.Receive(0, fmt.Errorf("API Key 验证失败"))
		}
		return &edge.SendResponse{Success: false, Accepted: 0}, nil
	}
	if err := validate.TraceBatch(batch); err != nil {
		s.logger.Warnf("链路追踪数据被拒绝: 参数校验失败: %v", err)
		if s.metrics != nil {
			s.metrics.Receive(0, err)
		}
		return &edge.SendResponse{Success: false, Accepted: 0}, nil
	}
	if s.metrics != nil {
		s.metrics.Receive(len(batch.GetSpans())*200, nil) // 估算字节数
	}
	s.forwarder.AddTraces(batch)
	return &edge.SendResponse{Success: true, Accepted: int32(len(batch.GetSpans()))}, nil
}

// SendProfiling 接收探针发送的性能分析数据
func (s *Server) SendProfiling(ctx context.Context, batch *edge.ProfilingBatch) (*edge.SendResponse, error) {
	if !grpcutil.CheckAPIKey(ctx, s.apiKey) {
		s.logger.Warnf("性能分析数据被拒绝: API Key 验证失败, probeID=%s", batch.GetProbeId())
		if s.metrics != nil {
			s.metrics.Receive(0, fmt.Errorf("API Key 验证失败"))
		}
		return &edge.SendResponse{Success: false, Accepted: 0}, nil
	}
	if err := validate.ProfilingBatch(batch); err != nil {
		s.logger.Warnf("性能分析数据被拒绝: 参数校验失败: %v", err)
		if s.metrics != nil {
			s.metrics.Receive(0, err)
		}
		return &edge.SendResponse{Success: false, Accepted: 0}, nil
	}
	if s.metrics != nil {
		s.metrics.Receive(len(batch.GetProfiles())*500, nil) // 估算字节数
	}
	s.forwarder.AddProfiling(batch)
	return &edge.SendResponse{Success: true, Accepted: int32(len(batch.GetProfiles()))}, nil
}
