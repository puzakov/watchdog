package service

import (
	"context"
	"testing"
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
