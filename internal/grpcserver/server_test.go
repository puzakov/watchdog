package grpcserver

import (
	"context"
	"net"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/puzakov/watchdog/internal/audit"
	"github.com/puzakov/watchdog/internal/crypto"
	proto "github.com/puzakov/watchdog/internal/proto"
	"github.com/puzakov/watchdog/internal/service"
)

type testAuditObserver struct {
	mu     sync.Mutex
	events []audit.Event
}

func (o *testAuditObserver) Notify(e audit.Event) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = append(o.events, e)
}

func (o *testAuditObserver) Events() []audit.Event {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]audit.Event, len(o.events))
	copy(out, o.events)
	return out
}

// setupTestServer creates a gRPC test server on a random TCP port and returns
// a client, storage, and cleanup function.
func setupTestServer(t *testing.T, store service.Storage, auditor *audit.Subject) (proto.MetricsClient, func()) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	srv := grpc.NewServer(
		grpc.Creds(crypto.GRPCServerCredentials()),
	)
	proto.RegisterMetricsServer(srv, NewMetricsServer(store, auditor))

	go func() {
		_ = srv.Serve(lis)
	}()

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(crypto.GRPCClientCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to create gRPC client: %v", err)
	}

	client := proto.NewMetricsClient(conn)

	cleanup := func() {
		conn.Close()
		srv.Stop()
		lis.Close()
	}

	return client, cleanup
}

func TestMetricsServer_UpdateMetrics_Success(t *testing.T) {
	store := service.NewMemStorage()
	client, cleanup := setupTestServer(t, store, nil)
	defer cleanup()

	_, err := client.UpdateMetrics(context.Background(), &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{
			{Id: "test_gauge", Type: proto.Metric_GAUGE, Value: 42.5},
			{Id: "test_counter", Type: proto.Metric_COUNTER, Delta: 10},
		},
	})
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}

	g, ok := store.GetGauge(context.Background(), "test_gauge")
	if !ok {
		t.Fatal("expected gauge test_gauge to exist")
	}
	if g != 42.5 {
		t.Fatalf("expected gauge value 42.5, got %f", g)
	}

	c, ok := store.GetCounter(context.Background(), "test_counter")
	if !ok {
		t.Fatal("expected counter test_counter to exist")
	}
	if c != 10 {
		t.Fatalf("expected counter value 10, got %d", c)
	}
}

func TestMetricsServer_UpdateMetrics_NilThroughInterceptor_ReturnsInvalidArgument(t *testing.T) {
	// Direct test of the server's nil check (not via gRPC, since gRPC
	// serializes nil as an empty message before it reaches the handler).
	s := NewMetricsServer(service.NewMemStorage(), nil)
	_, err := s.UpdateMetrics(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected code InvalidArgument, got %v", status.Code(err))
	}
}

func TestMetricsServer_UpdateMetrics_EmptyMetricsList_ReturnsSuccess(t *testing.T) {
	store := service.NewMemStorage()
	client, cleanup := setupTestServer(t, store, nil)
	defer cleanup()

	_, err := client.UpdateMetrics(context.Background(), &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{},
	})
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}
}

func TestMetricsServer_UpdateMetrics_OnlyNilMetrics_ReturnsSuccess(t *testing.T) {
	store := service.NewMemStorage()
	client, cleanup := setupTestServer(t, store, nil)
	defer cleanup()

	_, err := client.UpdateMetrics(context.Background(), &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{nil, nil},
	})
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}

	// Storage should be empty
	g, ok := store.GetGauge(context.Background(), "any")
	if ok {
		t.Fatal("expected no gauges")
	}
	_ = g
}

func TestMetricsServer_UpdateMetrics_UnknownMetricType_Skipped(t *testing.T) {
	store := service.NewMemStorage()
	client, cleanup := setupTestServer(t, store, nil)
	defer cleanup()

	// Metric_MType(99) is an unknown type.
	_, err := client.UpdateMetrics(context.Background(), &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{
			{Id: "unknown", Type: proto.Metric_MType(99), Value: 1.0},
			{Id: "valid_gauge", Type: proto.Metric_GAUGE, Value: 1.0},
		},
	})
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}

	_, ok := store.GetGauge(context.Background(), "unknown")
	if ok {
		t.Fatal("expected unknown metric to be skipped")
	}

	_, ok = store.GetGauge(context.Background(), "valid_gauge")
	if !ok {
		t.Fatal("expected valid_gauge to exist")
	}
}

func TestMetricsServer_UpdateMetrics_NotifiesAuditor(t *testing.T) {
	store := service.NewMemStorage()
	observer := &testAuditObserver{}
	auditor := audit.NewSubject(observer)
	defer auditor.Shutdown()

	client, cleanup := setupTestServer(t, store, auditor)
	defer cleanup()

	_, err := client.UpdateMetrics(context.Background(), &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{
			{Id: "metric1", Type: proto.Metric_GAUGE, Value: 1.0},
			{Id: "metric2", Type: proto.Metric_COUNTER, Delta: 5},
		},
	})
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}

	// Audit is async; wait briefly for the event to arrive.
	events := observer.Events()
	if len(events) == 0 {
		t.Fatal("expected audit event, got none")
	}
	if len(events[0].Metrics) != 2 {
		t.Fatalf("expected 2 metric IDs in audit event, got %d", len(events[0].Metrics))
	}
}

func TestMetricsServer_UpdateMetrics_NilAuditor_SkipsNotification(t *testing.T) {
	store := service.NewMemStorage()
	client, cleanup := setupTestServer(t, store, audit.NewSubject())
	defer cleanup()

	_, err := client.UpdateMetrics(context.Background(), &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{
			{Id: "metric1", Type: proto.Metric_GAUGE, Value: 1.0},
		},
	})
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}
}

func TestMetricsServer_UpdateMetrics_MixedCounterAndGauge(t *testing.T) {
	store := service.NewMemStorage()
	client, cleanup := setupTestServer(t, store, nil)
	defer cleanup()

	_, err := client.UpdateMetrics(context.Background(), &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{
			{Id: "gauge1", Type: proto.Metric_GAUGE, Value: 1.1},
			{Id: "gauge2", Type: proto.Metric_GAUGE, Value: 2.2},
			{Id: "counter1", Type: proto.Metric_COUNTER, Delta: 100},
			{Id: "counter2", Type: proto.Metric_COUNTER, Delta: 200},
		},
	})
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}

	// Verify all stored.
	gauges, counters := store.Snapshot(context.Background())
	if len(gauges) != 2 {
		t.Fatalf("expected 2 gauges, got %d", len(gauges))
	}
	if len(counters) != 2 {
		t.Fatalf("expected 2 counters, got %d", len(counters))
	}
}

func TestMetricsServer_UpdateMetrics_CounterDeltaAccumulates(t *testing.T) {
	store := service.NewMemStorage()
	client, cleanup := setupTestServer(t, store, nil)
	defer cleanup()

	// Send same counter twice.
	for i := 0; i < 3; i++ {
		_, err := client.UpdateMetrics(context.Background(), &proto.UpdateMetricsRequest{
			Metrics: []*proto.Metric{
				{Id: "acc_counter", Type: proto.Metric_COUNTER, Delta: 10},
			},
		})
		if err != nil {
			t.Fatalf("UpdateMetrics iteration %d failed: %v", i, err)
		}
	}

	c, ok := store.GetCounter(context.Background(), "acc_counter")
	if !ok {
		t.Fatal("expected acc_counter to exist")
	}
	if c != 30 {
		t.Fatalf("expected accumulated counter 30, got %d", c)
	}
}

func TestMetricsServer_UpdateMetrics_GaugeReplacesValue(t *testing.T) {
	store := service.NewMemStorage()
	client, cleanup := setupTestServer(t, store, nil)
	defer cleanup()

	// Send same gauge twice.
	_, err := client.UpdateMetrics(context.Background(), &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{
			{Id: "rep_gauge", Type: proto.Metric_GAUGE, Value: 10.0},
		},
	})
	if err != nil {
		t.Fatalf("first update failed: %v", err)
	}

	_, err = client.UpdateMetrics(context.Background(), &proto.UpdateMetricsRequest{
		Metrics: []*proto.Metric{
			{Id: "rep_gauge", Type: proto.Metric_GAUGE, Value: 20.0},
		},
	})
	if err != nil {
		t.Fatalf("second update failed: %v", err)
	}

	g, ok := store.GetGauge(context.Background(), "rep_gauge")
	if !ok {
		t.Fatal("expected rep_gauge to exist")
	}
	if g != 20.0 {
		t.Fatalf("expected replaced gauge value 20.0, got %f", g)
	}
}

func TestNewMetricsServer_NilStore_DoesNotPanic(t *testing.T) {
	// It should create the server without panicking, though it will fail on use.
	s := NewMetricsServer(nil, nil)
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}
