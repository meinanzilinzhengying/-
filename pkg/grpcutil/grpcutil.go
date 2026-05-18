package grpcutil

import (
	"context"
	"crypto/subtle"

	"google.golang.org/grpc/metadata"
)

// WithAuth 将 API Key 注入 gRPC metadata
func WithAuth(ctx context.Context, apiKey string) context.Context {
	md := metadata.Pairs("x-api-key", apiKey)
	return metadata.NewOutgoingContext(ctx, md)
}

// CheckAPIKey 从 context 中提取并校验 API Key
// 使用恒定时间比较，防止时序攻击
func CheckAPIKey(ctx context.Context, expectedAPIKey string) bool {
	if expectedAPIKey == "" {
		return true
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	vals := md.Get("x-api-key")
	if len(vals) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(vals[0]), []byte(expectedAPIKey)) == 1
}
