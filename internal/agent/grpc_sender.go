package agent

import (
	"context"
	"fmt"
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
// If caCertFile is non-empty, the server is verified using that CA certificate.
// Otherwise, TLS is used without server verification (dev mode with self-signed certs).
func NewGRPCSender(address string, localIP string, caCertFile string) (*GRPCSender, error) {
	creds, err := crypto.LoadOrGenerateClientCreds(caCertFile)
	if err != nil {
		return nil, fmt.Errorf("client TLS credentials: %w", err)
	}

	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(creds),
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
