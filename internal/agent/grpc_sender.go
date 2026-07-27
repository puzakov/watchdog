package agent

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/puzakov/watchdog/internal/crypto"
	models "github.com/puzakov/watchdog/internal/model"
	proto "github.com/puzakov/watchdog/internal/proto"
)

// GRPCSender sends metrics to a gRPC server.
type GRPCSender struct {
	client  proto.MetricsClient
	conn    *grpc.ClientConn
	localIP string
}

// NewGRPCSender creates a gRPC client connection and sender.
// Uses TLS with an embedded self-signed certificate for encryption.
func NewGRPCSender(address string, localIP string) (*GRPCSender, error) {
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(crypto.GRPCClientCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &GRPCSender{
		client:  proto.NewMetricsClient(conn),
		conn:    conn,
		localIP: localIP,
	}, nil
}

// Close closes the gRPC connection.
func (s *GRPCSender) Close() error {
	return s.conn.Close()
}

// SendBatch sends metrics via gRPC. Returns an error if the send fails.
func (s *GRPCSender) SendBatch(ctx context.Context, metrics []models.Metrics) error {
	req := &proto.UpdateMetricsRequest{
		Metrics: make([]*proto.Metric, 0, len(metrics)),
	}

	for _, m := range metrics {
		pbMetric := &proto.Metric{
			Id: m.ID,
		}
		switch m.MType {
		case models.Gauge:
			pbMetric.Type = proto.Metric_GAUGE
			if m.Value != nil {
				pbMetric.Value = *m.Value
			}
		case models.Counter:
			pbMetric.Type = proto.Metric_COUNTER
			if m.Delta != nil {
				pbMetric.Delta = *m.Delta
			}
		default:
			continue
		}
		req.Metrics = append(req.Metrics, pbMetric)
	}

	if len(req.Metrics) == 0 {
		return nil
	}

	md := metadata.Pairs("x-real-ip", s.localIP)
	grpcCtx := metadata.NewOutgoingContext(ctx, md)

	// Use a reasonable timeout for the gRPC call.
	grpcCtx, cancel := context.WithTimeout(grpcCtx, 10*time.Second)
	defer cancel()

	_, err := s.client.UpdateMetrics(grpcCtx, req)
	return err
}
