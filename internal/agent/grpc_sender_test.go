package agent

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"

	"github.com/puzakov/watchdog/internal/audit"
	"github.com/puzakov/watchdog/internal/crypto"
	grpcserver "github.com/puzakov/watchdog/internal/grpcserver"
	models "github.com/puzakov/watchdog/internal/model"
	proto "github.com/puzakov/watchdog/internal/proto"
	"github.com/puzakov/watchdog/internal/service"
)

// startTestGRPCServer starts a gRPC MetricsServer on a random port and returns
// the address and a cleanup function.
func startTestGRPCServer(t *testing.T, store service.Storage) (addr string, cleanup func()) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	srv := grpc.NewServer(
		grpc.Creds(crypto.GRPCServerCredentials()),
	)

	// Use a real auditor with capacity=0 observers so Notify is a no-op
	// (Subject.Notify returns early when len(observers) == 0).
	auditor := audit.NewSubject()
	metricsSrv := grpcserver.NewMetricsServer(store, auditor)
	proto.RegisterMetricsServer(srv, metricsSrv)

	go func() {
		_ = srv.Serve(lis)
	}()

	addr = lis.Addr().String()

	cleanup = func() {
		srv.Stop()
		lis.Close()
		auditor.Shutdown()
	}

	return addr, cleanup
}

func TestGRPCSender_SendBatch_Success(t *testing.T) {
	store := service.NewMemStorage()
	addr, cleanup := startTestGRPCServer(t, store)
	defer cleanup()

	sender, err := NewGRPCSender(addr, "10.0.0.1")
	if err != nil {
		t.Fatalf("NewGRPCSender failed: %v", err)
	}
	defer sender.Close()

	metrics := []models.Metrics{
		{ID: "gauge1", MType: models.Gauge, Value: float64Ptr(42.5)},
		{ID: "counter1", MType: models.Counter, Delta: int64Ptr(10)},
	}

	err = sender.SendBatch(context.Background(), metrics)
	if err != nil {
		t.Fatalf("SendBatch failed: %v", err)
	}

	g, ok := store.GetGauge(context.Background(), "gauge1")
	if !ok {
		t.Fatal("expected gauge1 to exist")
	}
	if g != 42.5 {
		t.Fatalf("expected gauge value 42.5, got %f", g)
	}

	c, ok := store.GetCounter(context.Background(), "counter1")
	if !ok {
		t.Fatal("expected counter1 to exist")
	}
	if c != 10 {
		t.Fatalf("expected counter value 10, got %d", c)
	}
}

func TestGRPCSender_SendBatch_EmptyList(t *testing.T) {
	store := service.NewMemStorage()
	addr, cleanup := startTestGRPCServer(t, store)
	defer cleanup()

	sender, err := NewGRPCSender(addr, "10.0.0.1")
	if err != nil {
		t.Fatalf("NewGRPCSender failed: %v", err)
	}
	defer sender.Close()

	err = sender.SendBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("SendBatch with nil should succeed: %v", err)
	}

	err = sender.SendBatch(context.Background(), []models.Metrics{})
	if err != nil {
		t.Fatalf("SendBatch with empty slice should succeed: %v", err)
	}
}

func TestGRPCSender_SendBatch_SingleGauge(t *testing.T) {
	store := service.NewMemStorage()
	addr, cleanup := startTestGRPCServer(t, store)
	defer cleanup()

	sender, err := NewGRPCSender(addr, "10.0.0.1")
	if err != nil {
		t.Fatalf("NewGRPCSender failed: %v", err)
	}
	defer sender.Close()

	err = sender.SendBatch(context.Background(), []models.Metrics{
		{ID: "only_gauge", MType: models.Gauge, Value: float64Ptr(99.9)},
	})
	if err != nil {
		t.Fatalf("SendBatch failed: %v", err)
	}

	g, ok := store.GetGauge(context.Background(), "only_gauge")
	if !ok {
		t.Fatal("expected only_gauge to exist")
	}
	if g != 99.9 {
		t.Fatalf("expected gauge value 99.9, got %f", g)
	}
}

func TestGRPCSender_SendBatch_SingleCounter(t *testing.T) {
	store := service.NewMemStorage()
	addr, cleanup := startTestGRPCServer(t, store)
	defer cleanup()

	sender, err := NewGRPCSender(addr, "10.0.0.1")
	if err != nil {
		t.Fatalf("NewGRPCSender failed: %v", err)
	}
	defer sender.Close()

	err = sender.SendBatch(context.Background(), []models.Metrics{
		{ID: "only_counter", MType: models.Counter, Delta: int64Ptr(777)},
	})
	if err != nil {
		t.Fatalf("SendBatch failed: %v", err)
	}

	c, ok := store.GetCounter(context.Background(), "only_counter")
	if !ok {
		t.Fatal("expected only_counter to exist")
	}
	if c != 777 {
		t.Fatalf("expected counter value 777, got %d", c)
	}
}

func TestGRPCSender_SendBatch_UnknownType_Skipped(t *testing.T) {
	store := service.NewMemStorage()
	addr, cleanup := startTestGRPCServer(t, store)
	defer cleanup()

	sender, err := NewGRPCSender(addr, "10.0.0.1")
	if err != nil {
		t.Fatalf("NewGRPCSender failed: %v", err)
	}
	defer sender.Close()

	// Metric with empty MType will be skipped in SendBatch conversion.
	err = sender.SendBatch(context.Background(), []models.Metrics{
		{ID: "skip_me", MType: "unknown", Value: float64Ptr(1.0)},
	})
	if err != nil {
		t.Fatalf("SendBatch failed: %v", err)
	}
}

func TestGRPCSender_NewGRPCSender_InvalidAddress(t *testing.T) {
	sender, err := NewGRPCSender("invalid:address!!!", "10.0.0.1")
	if err != nil {
		// In some environments the gRPC client may error immediately.
		return
	}
	defer sender.Close()

	// grpc.NewClient is lazy, so an invalid address may not error until the first RPC.
	err = sender.SendBatch(context.Background(), []models.Metrics{
		{ID: "test", MType: models.Gauge, Value: float64Ptr(1.0)},
	})
	if err == nil {
		t.Fatal("expected error when sending to invalid address, got nil")
	}
}

func TestGRPCSender_Close_Multiple(t *testing.T) {
	store := service.NewMemStorage()
	addr, cleanup := startTestGRPCServer(t, store)
	defer cleanup()

	sender, err := NewGRPCSender(addr, "10.0.0.1")
	if err != nil {
		t.Fatalf("NewGRPCSender failed: %v", err)
	}

	// Close multiple times should not panic.
	if err := sender.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := sender.Close(); err == nil {
		t.Fatal("expected error on second close, got nil")
	}
}

func float64Ptr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}
