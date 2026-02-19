package service

type Storage interface {
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, delta int64)
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

func (s *MemStorage) UpdateGauge(name string, value float64) {
	s.gauges[name] = value
}

func (s *MemStorage) UpdateCounter(name string, delta int64) {
	s.counters[name] += delta
}
