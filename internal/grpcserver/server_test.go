package grpcserver

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/puzakov/watchdog/internal/audit"
	"github.com/puzakov/watchdog/internal/crypto"
	proto "github.com/puzakov/watchdog/internal/proto"
	"github.com/puzakov/watchdog/internal/service"
)

// channelAuditObserver sends each received event on a channel.
// Tests can wait on the channel to synchronise with async audit delivery.
type channelAuditObserver struct {
	ch chan audit.Event
}

func newChannelAuditObserver() *channelAuditObserver {
	return &channelAuditObserver{ch: make(chan audit.Event, 16)}
}

func (o *channelAuditObserver) Notify(e audit.Event) {
	select {
	case o.ch <- e:
	default:
		// drop if buffer full — same as Subject behaviour
	}
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
		grpc.Creds(crypto.MustLoadOrGenerateServerCreds()),
	)
	proto.RegisterMetricsServer(srv, NewMetricsServer(store, auditor))

	go func() {
		_ = srv.Serve(lis)
	}()

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(crypto.MustLoadOrGenerateClientCreds()),
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

	_, err := client.UpdateMetrics(context.Background(), proto.UpdateMetricsRequest_builder{
		Metrics: []*proto.Metric{
			proto.Metric_builder{Id: "test_gauge", Type: proto.Metric_GAUGE, Value: 42.5}.Build(),
			proto.Metric_builder{Id: "test_counter", Type: proto.Metric_COUNTER, Delta: 10}.Build(),
		},
	}.Build())
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

	_, err := client.UpdateMetrics(context.Background(), proto.UpdateMetricsRequest_builder{
		Metrics: []*proto.Metric{},
	}.Build())
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}
}

func TestMetricsServer_UpdateMetrics_OnlyNilMetrics_ReturnsSuccess(t *testing.T) {
	store := service.NewMemStorage()
	client, cleanup := setupTestServer(t, store, nil)
	defer cleanup()

	_, err := client.UpdateMetrics(context.Background(), proto.UpdateMetricsRequest_builder{
		Metrics: []*proto.Metric{nil, nil},
	}.Build())
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}

	_, ok := store.GetGauge(context.Background(), "any")
	if ok {
		t.Fatal("expected no gauges")
	}
}

func TestMetricsServer_UpdateMetrics_UnknownMetricType_Skipped(t *testing.T) {
	store := service.NewMemStorage()
	client, cleanup := setupTestServer(t, store, nil)
	defer cleanup()

	_, err := client.UpdateMetrics(context.Background(), proto.UpdateMetricsRequest_builder{
		Metrics: []*proto.Metric{
			proto.Metric_builder{Id: "unknown", Type: proto.Metric_MType(99)}.Build(),
			proto.Metric_builder{Id: "valid_gauge", Type: proto.Metric_GAUGE, Value: 1.0}.Build(),
		},
	}.Build())
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
	chObserver := newChannelAuditObserver()
	auditor := audit.NewSubject(chObserver)
	defer auditor.Shutdown()

	client, cleanup := setupTestServer(t, store, auditor)
	defer cleanup()

	_, err := client.UpdateMetrics(context.Background(), proto.UpdateMetricsRequest_builder{
		Metrics: []*proto.Metric{
			proto.Metric_builder{Id: "metric1", Type: proto.Metric_GAUGE, Value: 1.0}.Build(),
			proto.Metric_builder{Id: "metric2", Type: proto.Metric_COUNTER, Delta: 5}.Build(),
		},
	}.Build())
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}

	select {
	case event := <-chObserver.ch:
		if len(event.Metrics) != 2 {
			t.Fatalf("expected 2 metric IDs in audit event, got %d", len(event.Metrics))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for audit event")
	}
}

func TestMetricsServer_UpdateMetrics_NilAuditor_SkipsNotification(t *testing.T) {
	store := service.NewMemStorage()
	client, cleanup := setupTestServer(t, store, audit.NewSubject())
	defer cleanup()

	_, err := client.UpdateMetrics(context.Background(), proto.UpdateMetricsRequest_builder{
		Metrics: []*proto.Metric{
			proto.Metric_builder{Id: "metric1", Type: proto.Metric_GAUGE, Value: 1.0}.Build(),
		},
	}.Build())
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}
}

func TestMetricsServer_UpdateMetrics_MixedCounterAndGauge(t *testing.T) {
	store := service.NewMemStorage()
	client, cleanup := setupTestServer(t, store, nil)
	defer cleanup()

	_, err := client.UpdateMetrics(context.Background(), proto.UpdateMetricsRequest_builder{
		Metrics: []*proto.Metric{
			proto.Metric_builder{Id: "gauge1", Type: proto.Metric_GAUGE, Value: 1.1}.Build(),
			proto.Metric_builder{Id: "gauge2", Type: proto.Metric_GAUGE, Value: 2.2}.Build(),
			proto.Metric_builder{Id: "counter1", Type: proto.Metric_COUNTER, Delta: 100}.Build(),
			proto.Metric_builder{Id: "counter2", Type: proto.Metric_COUNTER, Delta: 200}.Build(),
		},
	}.Build())
	if err != nil {
		t.Fatalf("UpdateMetrics failed: %v", err)
	}

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

	for i := 0; i < 3; i++ {
		_, err := client.UpdateMetrics(context.Background(), proto.UpdateMetricsRequest_builder{
			Metrics: []*proto.Metric{
				proto.Metric_builder{Id: "acc_counter", Type: proto.Metric_COUNTER, Delta: 10}.Build(),
			},
		}.Build())
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

	_, err := client.UpdateMetrics(context.Background(), proto.UpdateMetricsRequest_builder{
		Metrics: []*proto.Metric{
			proto.Metric_builder{Id: "rep_gauge", Type: proto.Metric_GAUGE, Value: 10.0}.Build(),
		},
	}.Build())
	if err != nil {
		t.Fatalf("first update failed: %v", err)
	}

	_, err = client.UpdateMetrics(context.Background(), proto.UpdateMetricsRequest_builder{
		Metrics: []*proto.Metric{
			proto.Metric_builder{Id: "rep_gauge", Type: proto.Metric_GAUGE, Value: 20.0}.Build(),
		},
	}.Build())
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
	s := NewMetricsServer(nil, nil)
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}
