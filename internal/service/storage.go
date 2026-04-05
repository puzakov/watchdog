package service

import (
	"sync"

	models "github.com/puzakov/watchdog/internal/model"
)

type Storage interface {
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	UpdateGauge(name string, value float64) error
	UpdateCounter(name string, delta int64) error
	UpdateBatch(metrics []models.Metrics) error
	Snapshot() (map[string]float64, map[string]int64)
}

type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

func NewMemStorage() Storage {
	return &MemStorage{
		// Тип gauge, float64 — новое значение должно замещать предыдущее.
		gauges: make(map[string]float64),
		// Тип counter, int64 — новое значение должно добавляться к предыдущему, если какое-то значение уже было известно серверу.
		counters: make(map[string]int64),
	}
}

func (s *MemStorage) GetGauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.gauges[name]
	return v, ok
}

func (s *MemStorage) GetCounter(name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.counters[name]
	return v, ok
}

func (s *MemStorage) UpdateGauge(name string, value float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gauges[name] = value
	return nil
}

func (s *MemStorage) UpdateCounter(name string, delta int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[name] += delta
	return nil
}

func (s *MemStorage) UpdateBatch(metrics []models.Metrics) error {
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

func (s *MemStorage) Snapshot() (map[string]float64, map[string]int64) {
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
