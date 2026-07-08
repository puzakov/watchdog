package service

import (
	"context"
	"testing"

	models "github.com/puzakov/watchdog/internal/model"
)

func TestMemStorage_GaugeReplacesValue(t *testing.T) {
	s := NewMemStorage()
	_ = s.UpdateGauge(context.Background(), "g1", 1.25)
	_ = s.UpdateGauge(context.Background(), "g1", 2.5)

	g, _ := s.Snapshot(context.Background())
	if got := g["g1"]; got != 2.5 {
		t.Fatalf("g1 = %v, want %v", got, 2.5)
	}
}

func TestMemStorage_CounterAddsDelta(t *testing.T) {
	s := NewMemStorage()
	_ = s.UpdateCounter(context.Background(), "c1", 10)
	_ = s.UpdateCounter(context.Background(), "c1", 5)

	_, c := s.Snapshot(context.Background())
	if got := c["c1"]; got != 15 {
		t.Fatalf("c1 = %v, want %v", got, 15)
	}
}

func TestMemStorage_UpdateBatch(t *testing.T) {
	s := NewMemStorage()
	ctx := context.Background()

	v := 3.14
	d1, d2 := int64(10), int64(5)
	err := s.UpdateBatch(ctx, []models.Metrics{
		{ID: "Alloc", MType: models.Gauge, Value: &v},
		{ID: "PollCount", MType: models.Counter, Delta: &d1},
		{ID: "PollCount", MType: models.Counter, Delta: &d2},
	})
	if err != nil {
		t.Fatalf("UpdateBatch: %v", err)
	}

	g, c := s.Snapshot(ctx)
	if g["Alloc"] != 3.14 {
		t.Fatalf("Alloc = %v, want 3.14", g["Alloc"])
	}
	if c["PollCount"] != 15 {
		t.Fatalf("PollCount = %v, want 15", c["PollCount"])
	}
}

func TestMemStorage_SnapshotReturnsCopies(t *testing.T) {
	s := NewMemStorage()
	_ = s.UpdateGauge(context.Background(), "g1", 1)
	_ = s.UpdateCounter(context.Background(), "c1", 1)

	g1, c1 := s.Snapshot(context.Background())
	g1["g1"] = 999
	c1["c1"] = 999

	g2, c2 := s.Snapshot(context.Background())
	if g2["g1"] != 1 {
		t.Fatalf("g1 after external mutation = %v, want %v", g2["g1"], 1)
	}
	if c2["c1"] != 1 {
		t.Fatalf("c1 after external mutation = %v, want %v", c2["c1"], 1)
	}
}

func TestMemStorage_UpdateBatch_UnknownType(t *testing.T) {
	s := NewMemStorage()
	ctx := context.Background()

	err := s.UpdateBatch(ctx, []models.Metrics{
		{ID: "test", MType: "unknown"},
	})
	if err != nil {
		t.Fatalf("UpdateBatch with unknown type should not error, got %v", err)
	}
}

func TestMemStorage_UpdateBatch_NilValueAndDelta(t *testing.T) {
	s := NewMemStorage()
	ctx := context.Background()

	// Metrics with nil Value/Delta should be skipped, not crash.
	err := s.UpdateBatch(ctx, []models.Metrics{
		{ID: "g1", MType: models.Gauge, Value: nil},
		{ID: "c1", MType: models.Counter, Delta: nil},
	})
	if err != nil {
		t.Fatalf("UpdateBatch with nil Value/Delta should not error, got %v", err)
	}

	// Verify nothing was stored.
	g, c := s.Snapshot(ctx)
	if _, ok := g["g1"]; ok {
		t.Fatal("g1 should not be stored")
	}
	if _, ok := c["c1"]; ok {
		t.Fatal("c1 should not be stored")
	}
}
