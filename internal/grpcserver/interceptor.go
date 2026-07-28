// Package grpcserver provides the gRPC server implementation for the Metrics service,
// including a trusted-subnet interceptor for access control.
package grpcserver

import (
	"context"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/puzakov/watchdog/internal/logger"
	"go.uber.org/zap"
)

// TrustedSubnetInterceptor returns a UnaryServerInterceptor that validates the
// client IP (from the x-real-ip metadata key) against the trusted subnet (CIDR).
// If trustedSubnet is empty, the interceptor is a no-op.
func TrustedSubnetInterceptor(trustedSubnet string) grpc.UnaryServerInterceptor {
	if trustedSubnet == "" {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}

	_, cidr, err := net.ParseCIDR(trustedSubnet)
	if err != nil {
		logger.Log.Error("invalid trusted subnet for gRPC", zap.String("subnet", trustedSubnet), zap.Error(err))
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return nil, status.Error(codes.Internal, "invalid trusted subnet configuration")
		}
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			logger.Log.Debug("gRPC request missing metadata")
			return nil, status.Error(codes.PermissionDenied, "missing metadata")
		}

		values := md.Get("x-real-ip")
		if len(values) == 0 {
			logger.Log.Debug("gRPC request missing x-real-ip metadata")
			return nil, status.Error(codes.PermissionDenied, "missing x-real-ip")
		}

		ipStr := values[0]
		ip := net.ParseIP(ipStr)
		if ip == nil {
			logger.Log.Debug("invalid x-real-ip in gRPC metadata", zap.String("ip", ipStr))
			return nil, status.Error(codes.PermissionDenied, "invalid x-real-ip")
		}

		if !cidr.Contains(ip) {
			logger.Log.Debug("gRPC request from untrusted IP",
				zap.String("ip", ipStr),
				zap.String("trusted_subnet", trustedSubnet),
			)
			return nil, status.Errorf(codes.PermissionDenied, "IP %s not in trusted subnet", ipStr)
		}

		return handler(ctx, req)
	}
}
