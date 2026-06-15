package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/puzakov/watchdog/internal/sign"
)

func BenchmarkGzip(b *testing.B) {
	payload := bytes.Repeat([]byte(`{"id":"Alloc","type":"gauge","value":123.45}`), 50)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	})
	h := Gzip(next)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkHashSHA256(b *testing.B) {
	payload := bytes.Repeat([]byte(`{"id":"Alloc","type":"gauge","value":123.45}`), 50)
	key := "secret-key"
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	})
	h := HashSHA256(key, next)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(sign.HeaderHashSHA256, sign.SumSHA256(payload, key))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}
