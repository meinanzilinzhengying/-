// Package topologyengine gRPC 服务实现
package topologyengine

import (
	"context"

	"google.golang.org/grpc"

	svcproto "cloud-flow/services/proto"
)

func RegisterTopologyService(s *grpc.Server, svc *Service) {
	svcproto.RegisterTopologyServiceServer(s, &topologyGRPC{svc: svc})
}

type topologyGRPC struct {
	svcproto.UnimplementedTopologyServiceServer
	svc *Service
}

func (g *topologyGRPC) HealthCheck(ctx context.Context, req *svcproto.HealthCheckRequest) (*svcproto.HealthCheckResponse, error) {
	return &svcproto.HealthCheckResponse{Healthy: true, Version: g.svc.config.Version}, nil
}
func (g *topologyGRPC) QueryTopology(ctx context.Context, req *svcproto.TopologyQueryRequest) (*svcproto.TopologyQueryResponse, error) {
	return g.svc.QueryTopology(ctx, req)
}
func (g *topologyGRPC) GetServiceGraph(ctx context.Context, req *svcproto.TopologyQueryRequest) (*svcproto.TopologyQueryResponse, error) {
	return g.svc.GetServiceGraph(ctx, req)
}
func (g *topologyGRPC) GetDependencyGraph(ctx context.Context, req *svcproto.TopologyQueryRequest) (*svcproto.TopologyQueryResponse, error) {
	return g.svc.GetDependencyGraph(ctx, req)
}
