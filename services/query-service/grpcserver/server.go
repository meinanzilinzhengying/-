// Package queryservice gRPC 服务实现
package queryservice

import (
	"context"

	"google.golang.org/grpc"

	svcproto "cloud-flow/services/proto"
)

func RegisterQueryService(s *grpc.Server, svc *Service) {
	svcproto.RegisterQueryServiceServer(s, &queryGRPC{svc: svc})
}

type queryGRPC struct {
	svcproto.UnimplementedQueryServiceServer
	svc *Service
}

func (g *queryGRPC) HealthCheck(ctx context.Context, req *svcproto.HealthCheckRequest) (*svcproto.HealthCheckResponse, error) {
	return &svcproto.HealthCheckResponse{Healthy: true, Version: g.svc.config.Version}, nil
}
func (g *queryGRPC) QueryFlows(ctx context.Context, req *svcproto.QueryFlowRequest) (*svcproto.QueryFlowResponse, error) {
	return g.svc.QueryFlows(ctx, req)
}
func (g *queryGRPC) QueryMetrics(ctx context.Context, req *svcproto.QueryFlowRequest) (*svcproto.QueryFlowResponse, error) {
	return g.svc.QueryMetrics(ctx, req)
}
func (g *queryGRPC) QueryTraces(ctx context.Context, req *svcproto.QueryFlowRequest) (*svcproto.QueryFlowResponse, error) {
	return g.svc.QueryTraces(ctx, req)
}
func (g *queryGRPC) QueryDashboard(ctx context.Context, req *svcproto.QueryFlowRequest) (*svcproto.QueryFlowResponse, error) {
	return g.svc.QueryDashboard(ctx, req)
}
