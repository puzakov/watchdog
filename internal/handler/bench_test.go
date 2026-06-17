package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/puzakov/watchdog/internal/db"
	"github.com/puzakov/watchdog/internal/middleware"
	models "github.com/puzakov/watchdog/internal/model"
	"github.com/puzakov/watchdog/internal/service"
)

func benchmarkBatchPayload(n int) []byte {
	metrics := make([]models.Metrics, 0, n*2)
	for i := 0; i < n; i++ {
		v := float64(i) + 0.5
		d := int64(i)
		metrics = append(metrics,
			models.Metrics{ID: "Alloc", MType: models.Gauge, Value: &v},
			models.Metrics{ID: "PollCount", MType: models.Counter, Delta: &d},
		)
	}
	body, _ := json.Marshal(metrics)
	return body
}

func BenchmarkNormalizeBatch(b *testing.B) {
	in := make([]models.Metrics, 0, 100)
	for i := 0; i < 50; i++ {
		v := float64(i)
		d := int64(1)
		in = append(in,
			models.Metrics{ID: "g", MType: models.Gauge, Value: &v},
			models.Metrics{ID: "c", MType: models.Counter, Delta: &d},
		)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = normalizeBatch(in)
	}
}

func BenchmarkHandleUpdatesJSON(b *testing.B) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)
	body := benchmarkBatchPayload(30)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkHandleIndex(b *testing.B) {
	store := service.NewMemStorage()
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		name := "metric_" + string(rune('a'+i%26))
		_ = store.UpdateGauge(ctx, name, float64(i))
		_ = store.UpdateCounter(ctx, name+"_c", int64(i))
	}

	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkHandleUpdatesWithMiddleware(b *testing.B) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)
	h = middleware.Gzip(h)
	h = middleware.HashSHA256("secret-key", h)

	body := benchmarkBatchPayload(30)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}
