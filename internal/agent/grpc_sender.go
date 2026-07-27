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
	pbMetrics := make([]*proto.Metric, 0, len(metrics))

	for _, m := range metrics {
		var typ proto.Metric_MType
		var delta int64
		var value float64
		hasDelta := false
		hasValue := false

		switch m.MType {
		case models.Gauge:
			typ = proto.Metric_GAUGE
			if m.Value != nil {
				value = *m.Value
				hasValue = true
			}
		case models.Counter:
			typ = proto.Metric_COUNTER
			if m.Delta != nil {
				delta = *m.Delta
				hasDelta = true
			}
		default:
			continue
		}

		b := proto.Metric_builder{
			Id:   m.ID,
			Type: typ,
		}
		if hasValue {
			b.Value = value
		}
		if hasDelta {
			b.Delta = delta
		}
		pbMetrics = append(pbMetrics, b.Build())
	}

	if len(pbMetrics) == 0 {
		return nil
	}

	req := proto.UpdateMetricsRequest_builder{
		Metrics: pbMetrics,
	}.Build()

	md := metadata.Pairs("x-real-ip", s.localIP)
	grpcCtx := metadata.NewOutgoingContext(ctx, md)

	// Use a reasonable timeout for the gRPC call.
	grpcCtx, cancel := context.WithTimeout(grpcCtx, 10*time.Second)
	defer cancel()

	_, err := s.client.UpdateMetrics(grpcCtx, req)
	return err
}
