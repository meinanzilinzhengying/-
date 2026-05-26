// Package authservice gRPC 服务实现
package authservice

import (
	"context"

	"google.golang.org/grpc"

	svcproto "cloud-flow/services/proto"
)

func RegisterAuthService(s *grpc.Server, svc *Service) {
	svcproto.RegisterAuthServiceServer(s, &authGRPC{svc: svc})
}

type authGRPC struct {
	svcproto.UnimplementedAuthServiceServer
	svc *Service
}

func (g *authGRPC) HealthCheck(ctx context.Context, req *svcproto.HealthCheckRequest) (*svcproto.HealthCheckResponse, error) {
	return &svcproto.HealthCheckResponse{Healthy: true, Version: g.svc.config.Version}, nil
}
func (g *authGRPC) Authenticate(ctx context.Context, req *svcproto.AuthenticateRequest) (*svcproto.AuthenticateResponse, error) {
	return g.svc.Authenticate(ctx, req)
}
func (g *authGRPC) ValidateToken(ctx context.Context, req *svcproto.ValidateTokenRequest) (*svcproto.ValidateTokenResponse, error) {
	return g.svc.ValidateToken(ctx, req)
}
func (g *authGRPC) Authorize(ctx context.Context, req *svcproto.AuthorizeRequest) (*svcproto.AuthorizeResponse, error) {
	return g.svc.Authorize(ctx, req)
}
