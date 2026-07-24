package grpcserver

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestTrustedSubnetInterceptor_EmptySubnet_Noop(t *testing.T) {
	interceptor := TrustedSubnetInterceptor("")
	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	resp, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected 'ok', got %v", resp)
	}
}

func TestTrustedSubnetInterceptor_InvalidCIDR_ReturnsInternal(t *testing.T) {
	interceptor := TrustedSubnetInterceptor("not-a-cidr")
	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	_, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{}, handler)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected code Internal, got %v", status.Code(err))
	}
}

func TestTrustedSubnetInterceptor_MissingMetadata_ReturnsPermissionDenied(t *testing.T) {
	interceptor := TrustedSubnetInterceptor("192.168.1.0/24")
	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	_, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{}, handler)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected code PermissionDenied, got %v", status.Code(err))
	}
}

func TestTrustedSubnetInterceptor_MissingXRealIP_ReturnsPermissionDenied(t *testing.T) {
	interceptor := TrustedSubnetInterceptor("192.168.1.0/24")
	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	md := metadata.Pairs("some-other-header", "value")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{}, handler)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected code PermissionDenied, got %v", status.Code(err))
	}
}

func TestTrustedSubnetInterceptor_InvalidIP_ReturnsPermissionDenied(t *testing.T) {
	interceptor := TrustedSubnetInterceptor("192.168.1.0/24")
	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	md := metadata.Pairs("x-real-ip", "not-an-ip")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{}, handler)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected code PermissionDenied, got %v", status.Code(err))
	}
}

func TestTrustedSubnetInterceptor_IPOutsideSubnet_ReturnsPermissionDenied(t *testing.T) {
	interceptor := TrustedSubnetInterceptor("192.168.1.0/24")
	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	md := metadata.Pairs("x-real-ip", "10.0.0.1")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{}, handler)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected code PermissionDenied, got %v", status.Code(err))
	}
}

func TestTrustedSubnetInterceptor_IPInsideSubnet_Passes(t *testing.T) {
	interceptor := TrustedSubnetInterceptor("192.168.1.0/24")
	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	md := metadata.Pairs("x-real-ip", "192.168.1.100")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	resp, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected 'ok', got %v", resp)
	}
}

func TestTrustedSubnetInterceptor_SameSubnetDifferentPrefix_Passes(t *testing.T) {
	// Use a broader subnet that includes many IPs.
	interceptor := TrustedSubnetInterceptor("0.0.0.0/0")
	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	md := metadata.Pairs("x-real-ip", "1.2.3.4")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	resp, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected 'ok', got %v", resp)
	}
}
