package service

import (
	"context"
	"testing"

	models "github.com/puzakov/watchdog/internal/model"
)

func benchmarkMetrics(n int) []models.Metrics {
	metrics := make([]models.Metrics, 0, n*2)
	for i := 0; i < n; i++ {
		name := "gauge_" + string(rune('A'+i%26)) + string(rune('0'+i%10))
		v := float64(i)
		metrics = append(metrics, models.Metrics{ID: name, MType: models.Gauge, Value: &v})

		counterName := "counter_" + string(rune('A'+i%26)) + string(rune('0'+i%10))
		d := int64(i)
		metrics = append(metrics, models.Metrics{ID: counterName, MType: models.Counter, Delta: &d})
	}
	return metrics
}

func BenchmarkMemStorage_UpdateGauge(b *testing.B) {
	store := NewMemStorage()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.UpdateGauge(ctx, "Alloc", float64(i))
	}
}

func BenchmarkMemStorage_UpdateCounter(b *testing.B) {
	store := NewMemStorage()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.UpdateCounter(ctx, "PollCount", 1)
	}
}

func BenchmarkMemStorage_UpdateBatch(b *testing.B) {
	store := NewMemStorage()
	ctx := context.Background()
	batch := benchmarkMetrics(50)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.UpdateBatch(ctx, batch)
	}
}

func BenchmarkMemStorage_Snapshot(b *testing.B) {
	store := NewMemStorage()
	ctx := context.Background()
	_ = store.UpdateBatch(ctx, benchmarkMetrics(100))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Snapshot(ctx)
	}
}
