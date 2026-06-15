package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/puzakov/watchdog/internal/sign"
)

func TestHashSHA256_ValidRequest(t *testing.T) {
	body := []byte(`{"id":"Alloc","type":"gauge","value":1}`)
	key := "secret"

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	h := HashSHA256(key, next)

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(sign.HeaderHashSHA256, sign.SumSHA256(body, key))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Header().Get(sign.HeaderHashSHA256) == "" {
		t.Fatal("response hash header is empty")
	}
}

func TestHashSHA256_InvalidHash(t *testing.T) {
	body := []byte(`{"id":"Alloc","type":"gauge","value":1}`)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := HashSHA256("secret", next)

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.Header.Set(sign.HeaderHashSHA256, "bad")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHashSHA256_NoKeyPassthrough(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	h := HashSHA256("", next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if !called {
		t.Fatal("next handler was not called")
	}
}
