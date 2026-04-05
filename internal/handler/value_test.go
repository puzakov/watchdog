package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/puzakov/watchdog/internal/db"
	models "github.com/puzakov/watchdog/internal/model"
	"github.com/puzakov/watchdog/internal/service"
)

func TestValue_Gauge_OK(t *testing.T) {
	store := service.NewMemStorage()
	_ = store.UpdateGauge("Alloc", 1.5)

	h := NewHandler(store, &db.DatabaseConnection{})
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
	_ = store.UpdateCounter("PollCount", 42)

	h := NewHandler(store, &db.DatabaseConnection{})
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
	h := NewHandler(store, &db.DatabaseConnection{})

	req := httptest.NewRequest(http.MethodGet, "/value/gauge/Unknown", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestValueJSON_Counter_NotFound(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{})

	reqBody, _ := json.Marshal(&models.Metrics{ID: "missing", MType: models.Counter})
	req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestValueJSON_Counter_OK_ReturnsJSON(t *testing.T) {
	store := service.NewMemStorage()
	_ = store.UpdateCounter("c1", 42)
	h := NewHandler(store, &db.DatabaseConnection{})

	reqBody, _ := json.Marshal(&models.Metrics{ID: "c1", MType: models.Counter})
	req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct == "" || ct[:16] != "application/json" {
		t.Fatalf("Content-Type = %q, want prefix %q", ct, "application/json")
	}

	var resp models.Metrics
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v, body=%q", err, w.Body.String())
	}
	if resp.ID != "c1" || resp.MType != models.Counter || resp.Delta == nil || *resp.Delta != 42 || resp.Value != nil {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
