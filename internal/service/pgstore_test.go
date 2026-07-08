package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	models "github.com/puzakov/watchdog/internal/model"
)

func TestPostgresStorage_NilPool_ReturnsDefaults(t *testing.T) {
	var s *PostgresStorage

	// Snapshot on nil receiver returns empty maps.
	g, c := s.Snapshot(context.Background())
	if len(g) != 0 || len(c) != 0 {
		t.Fatalf("Snapshot on nil receiver should return empty maps, got gauges=%d counters=%d", len(g), len(c))
	}

	// GetGauge on nil receiver returns 0, false.
	v, ok := s.GetGauge(context.Background(), "test")
	if ok || v != 0 {
		t.Fatalf("GetGauge on nil receiver = (%v, %v), want (0, false)", v, ok)
	}

	// GetCounter on nil receiver returns 0, false.
	d, ok := s.GetCounter(context.Background(), "test")
	if ok || d != 0 {
		t.Fatalf("GetCounter on nil receiver = (%v, %v), want (0, false)", d, ok)
	}

	// UpdateGauge on nil receiver returns nil.
	if err := s.UpdateGauge(context.Background(), "test", 1); err != nil {
		t.Fatalf("UpdateGauge on nil receiver should return nil, got %v", err)
	}

	// UpdateCounter on nil receiver returns nil.
	if err := s.UpdateCounter(context.Background(), "test", 1); err != nil {
		t.Fatalf("UpdateCounter on nil receiver should return nil, got %v", err)
	}

	// UpdateBatch on nil receiver returns nil.
	if err := s.UpdateBatch(context.Background(), []models.Metrics{}); err != nil {
		t.Fatalf("UpdateBatch on nil receiver should return nil, got %v", err)
	}
}

func TestPostgresStorage_NilPoolField_ReturnsDefaults(t *testing.T) {
	s := NewPostgresStorage(nil)

	// Methods on a PostgresStorage with nil pool should return safe defaults.
	if err := s.UpdateGauge(context.Background(), "test", 1); err != nil {
		t.Fatalf("UpdateGauge with nil pool should return nil, got %v", err)
	}
	if err := s.UpdateCounter(context.Background(), "test", 1); err != nil {
		t.Fatalf("UpdateCounter with nil pool should return nil, got %v", err)
	}
	if err := s.UpdateBatch(context.Background(), nil); err != nil {
		t.Fatalf("UpdateBatch with nil pool and empty metrics should return nil, got %v", err)
	}

	v, ok := s.GetGauge(context.Background(), "test")
	if ok || v != 0 {
		t.Fatalf("GetGauge with nil pool = (%v, %v), want (0, false)", v, ok)
	}

	d, ok := s.GetCounter(context.Background(), "test")
	if ok || d != 0 {
		t.Fatalf("GetCounter with nil pool = (%v, %v), want (0, false)", d, ok)
	}

	g, c := s.Snapshot(context.Background())
	if len(g) != 0 || len(c) != 0 {
		t.Fatalf("Snapshot with nil pool should return empty maps")
	}
}

func TestPostgresStorage_UpdateBatch_EmptyMetrics(t *testing.T) {
	s := NewPostgresStorage(nil)
	if err := s.UpdateBatch(context.Background(), []models.Metrics{}); err != nil {
		t.Fatalf("UpdateBatch with empty metrics should return nil, got %v", err)
	}
	if err := s.UpdateBatch(context.Background(), nil); err != nil {
		t.Fatalf("UpdateBatch with nil metrics should return nil, got %v", err)
	}
}

func TestPostgresStorage_NewPostgresStorage(t *testing.T) {
	s := NewPostgresStorage(nil)
	if s == nil {
		t.Fatal("NewPostgresStorage should return non-nil")
	}
}

func TestIsRetriablePGConnError(t *testing.T) {
	// Not a PG error at all.
	if isRetriablePGConnError(errors.New("plain error")) {
		t.Error("plain error should not be retriable")
	}

	// PG error but not connection-related.
	pgErr := &pgconn.PgError{Code: "42P01"} // undefined_table
	if isRetriablePGConnError(pgErr) {
		t.Error("non-connection PG error should not be retriable")
	}

	// PG error with connection exception code.
	connErr := &pgconn.PgError{Code: "08006"} // connection_failure
	if !isRetriablePGConnError(connErr) {
		t.Error("connection exception PG error should be retriable")
	}

	// Another connection exception code.
	connErr2 := &pgconn.PgError{Code: "08001"} // sqlclient_unable_to_establish_sqlconnection
	if !isRetriablePGConnError(connErr2) {
		t.Error("08001 should be retriable")
	}

	// Verify pgerrcode.IsConnectionException is used correctly.
	if !pgerrcode.IsConnectionException("08006") {
		t.Error("08006 should be a connection exception")
	}
	if !pgerrcode.IsConnectionException("08001") {
		t.Error("08001 should be a connection exception")
	}
	if pgerrcode.IsConnectionException("42P01") {
		t.Error("42P01 should not be a connection exception")
	}
}
