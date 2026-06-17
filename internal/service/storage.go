package service

import (
	"context"
	"sync"

	models "github.com/puzakov/watchdog/internal/model"
)

// Storage defines the interface for metrics persistence.
// Implementations must be safe for concurrent use.
type Storage interface {
	// GetGauge returns the gauge value by name and a boolean indicating whether it exists.
	GetGauge(ctx context.Context, name string) (float64, bool)
	// GetCounter returns the counter value by name and a boolean indicating whether it exists.
	GetCounter(ctx context.Context, name string) (int64, bool)
	// UpdateGauge sets a gauge metric to the given value (replaces previous).
	UpdateGauge(ctx context.Context, name string, value float64) error
	// UpdateCounter adds delta to an existing counter or creates it.
	UpdateCounter(ctx context.Context, name string, delta int64) error
	// UpdateBatch atomically applies a batch of metrics.
	// Duplicates are deduplicated: gauges take last value, counters sum deltas.
	UpdateBatch(ctx context.Context, metrics []models.Metrics) error
	// Snapshot returns a consistent copy of all gauges and counters.
	Snapshot(ctx context.Context) (map[string]float64, map[string]int64)
}

// MemStorage is an in-memory implementation of Storage backed by maps with RWMutex.
// Gauges are replaced on update; counters accumulate.
type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

// NewMemStorage creates an in-memory Storage ready for use.
func NewMemStorage() Storage {
	return &MemStorage{
		// Тип gauge, float64 — новое значение должно замещать предыдущее.
		gauges: make(map[string]float64),
		// Тип counter, int64 — новое значение должно добавляться к предыдущему, если какое-то значение уже было известно серверу.
		counters: make(map[string]int64),
	}
}

func (s *MemStorage) GetGauge(ctx context.Context, name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.gauges[name]
	return v, ok
}

func (s *MemStorage) GetCounter(ctx context.Context, name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.counters[name]
	return v, ok
}

func (s *MemStorage) UpdateGauge(ctx context.Context, name string, value float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gauges[name] = value
	return nil
}

func (s *MemStorage) UpdateCounter(ctx context.Context, name string, delta int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[name] += delta
	return nil
}

func (s *MemStorage) UpdateBatch(ctx context.Context, metrics []models.Metrics) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range metrics {
		switch m.MType {
		case models.Gauge:
			if m.Value == nil {
				continue
			}
			s.gauges[m.ID] = *m.Value
		case models.Counter:
			if m.Delta == nil {
				continue
			}
			s.counters[m.ID] += *m.Delta
		default:
			// ignore unknown types to keep behaviour close to UpdateGauge/UpdateCounter (no errors)
		}
	}
	return nil
}

func (s *MemStorage) Snapshot(ctx context.Context) (map[string]float64, map[string]int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	g := make(map[string]float64, len(s.gauges))
	for k, v := range s.gauges {
		g[k] = v
	}
	c := make(map[string]int64, len(s.counters))
	for k, v := range s.counters {
		c[k] = v
	}
	return g, c
}
