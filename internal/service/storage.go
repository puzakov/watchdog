package service

type Storage interface {
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, delta int64)
	Snapshot() (map[string]float64, map[string]int64)
}

type MemStorage struct {
	gauges   map[string]float64
	counters map[string]int64
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		// Тип gauge, float64 — новое значение должно замещать предыдущее.
		gauges: make(map[string]float64),
		// Тип counter, int64 — новое значение должно добавляться к предыдущему, если какое-то значение уже было известно серверу.
		counters: make(map[string]int64),
	}
}

func (s *MemStorage) GetGauge(name string) (float64, bool) {
	v, ok := s.gauges[name]
	return v, ok
}

func (s *MemStorage) GetCounter(name string) (int64, bool) {
	v, ok := s.counters[name]
	return v, ok
}

func (s *MemStorage) UpdateGauge(name string, value float64) {
	s.gauges[name] = value
}

func (s *MemStorage) UpdateCounter(name string, delta int64) {
	s.counters[name] += delta
}

func (s *MemStorage) Snapshot() (map[string]float64, map[string]int64) {
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
