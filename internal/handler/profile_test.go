package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/puzakov/watchdog/internal/db"
	"github.com/puzakov/watchdog/internal/middleware"
	"github.com/puzakov/watchdog/internal/service"
)

func simulateLoad(b *testing.B, h http.Handler, body []byte) {
	b.Helper()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		req = httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w = httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

// go test -bench=BenchmarkHeapProfile -benchtime=3s -memprofile=profiles/base.pprof ./internal/handler/
func BenchmarkHeapProfile(b *testing.B) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)
	h = middleware.Gzip(h)
	h = middleware.HashSHA256("profile-key", h)

	body := benchmarkBatchPayload(50)
	runtime.GC()

	b.ResetTimer()
	simulateLoad(b, h, body)
}
