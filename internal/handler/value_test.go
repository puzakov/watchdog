package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/puzakov/watchdog/internal/service"
)

func TestValue_Gauge_OK(t *testing.T) {
	store := service.NewMemStorage()
	store.UpdateGauge("Alloc", 1.5)

	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/value/gauge/Alloc", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Body.String(); got != "1.5" {
		t.Fatalf("body = %q, want %q", got, "1.5")
	}
}

func TestValue_Counter_OK(t *testing.T) {
	store := service.NewMemStorage()
	store.UpdateCounter("PollCount", 42)

	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/value/counter/PollCount", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Body.String(); got != "42" {
		t.Fatalf("body = %q, want %q", got, "42")
	}
}

func TestValue_UnknownMetric_NotFound(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/value/gauge/Unknown", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
