package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/puzakov/watchdog/internal/db"
	"github.com/puzakov/watchdog/internal/service"
)

func TestNewHandler_RoutesToUpdate(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/123", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	g, _ := store.Snapshot(context.Background())
	if got := g["Alloc"]; got != 123 {
		t.Fatalf("Alloc = %v, want %v", got, 123)
	}
}

func TestNewHandler_GetRoot_ReturnsHTMLWithMetrics(t *testing.T) {
	store := service.NewMemStorage()
	_ = store.UpdateGauge(context.Background(), "Alloc", 1.5)
	_ = store.UpdateCounter(context.Background(), "PollCount", 2)

	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	ct := w.Header().Get("Content-Type")
	if ct == "" || ct[:9] != "text/html" {
		t.Fatalf("Content-Type = %q, want prefix %q", ct, "text/html")
	}

	body := w.Body.String()
	if !strings.Contains(body, "Alloc") || !strings.Contains(body, "1.5") {
		t.Fatalf("body does not contain gauge metric, body:\n%s", body)
	}
	if !strings.Contains(body, "PollCount") || !strings.Contains(body, "2") {
		t.Fatalf("body does not contain counter metric, body:\n%s", body)
	}
}

func TestNewHandler_RoutesToValue(t *testing.T) {
	store := service.NewMemStorage()
	_ = store.UpdateGauge(context.Background(), "Alloc", 1.5)

	h := NewHandler(store, &db.DatabaseConnection{}, nil)

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
