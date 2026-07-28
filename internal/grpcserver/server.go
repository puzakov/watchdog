package grpcserver

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/puzakov/watchdog/internal/audit"
	"github.com/puzakov/watchdog/internal/logger"
	proto "github.com/puzakov/watchdog/internal/proto"
	"github.com/puzakov/watchdog/internal/service"
	"go.uber.org/zap"

	models "github.com/puzakov/watchdog/internal/model"
)

// MetricsServer implements the proto.MetricsServer gRPC interface.
// It receives metric updates and stores them via the Storage backend,
// optionally notifying audit subscribers.
type MetricsServer struct {
	proto.UnimplementedMetricsServer
	store   service.Storage
	auditor *audit.Subject
}

// NewMetricsServer creates a new gRPC metrics server with the given storage
// and optional audit subject.
func NewMetricsServer(store service.Storage, auditor *audit.Subject) *MetricsServer {
	return &MetricsServer{
		store:   store,
		auditor: auditor,
	}
}

// UpdateMetrics receives a batch of metrics, converts them to the internal model,
// stores them via Storage.UpdateBatch, and notifies the audit subject.
func (s *MetricsServer) UpdateMetrics(ctx context.Context, req *proto.UpdateMetricsRequest) (*proto.UpdateMetricsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	metrics := req.GetMetrics()
	batch := make([]models.Metrics, 0, len(metrics))
	ids := make([]string, 0, len(metrics))

	for _, m := range metrics {
		if m == nil {
			continue
		}

		mm := models.Metrics{
			ID: m.GetId(),
		}

		switch m.GetType() {
		case proto.Metric_GAUGE:
			mm.MType = models.Gauge
			v := m.GetValue()
			mm.Value = &v
		case proto.Metric_COUNTER:
			mm.MType = models.Counter
			d := m.GetDelta()
			mm.Delta = &d
		default:
			logger.Log.Warn("unknown metric type in gRPC request",
				zap.String("id", m.GetId()),
				zap.Int32("type", int32(m.GetType())),
			)
			continue
		}

		batch = append(batch, mm)
		ids = append(ids, m.GetId())
	}

	if len(batch) == 0 {
		return &proto.UpdateMetricsResponse{}, nil
	}

	if err := s.store.UpdateBatch(ctx, batch); err != nil {
		logger.Log.Error("gRPC batch update error", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to update metrics")
	}

	if s.auditor != nil {
		s.auditor.Notify(ids, "")
	}

	return &proto.UpdateMetricsResponse{}, nil
}
