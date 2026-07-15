package handler

import (
	"bytes"
	"context"
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

func TestValue_Counter_OK(t *testing.T) {
	store := service.NewMemStorage()
	_ = store.UpdateCounter(context.Background(), "PollCount", 42)

	h := NewHandler(store, &db.DatabaseConnection{}, nil)
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
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/value/gauge/Unknown", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestValueJSON_Counter_NotFound(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

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
	_ = store.UpdateCounter(context.Background(), "c1", 42)
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

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

func TestHandleValue_UnknownType(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/value/unknown/test", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleValue_Gauge_NotFound(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/value/gauge/nonexistent", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleValue_Counter_NotFound(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/value/counter/nonexistent", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleValueJSON_BadContentType(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/value/", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleValueJSON_InvalidJSON(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleValueJSON_UnknownType(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	body, _ := json.Marshal(&models.Metrics{ID: "test", MType: "unknown"})
	req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleValueJSON_Gauge_OK(t *testing.T) {
	store := service.NewMemStorage()
	_ = store.UpdateGauge(context.Background(), "Alloc", 5.5)
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	body, _ := json.Marshal(&models.Metrics{ID: "Alloc", MType: models.Gauge})
	req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp models.Metrics
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Value == nil || *resp.Value != 5.5 {
		t.Fatalf("Value = %v, want 5.5", resp.Value)
	}
}

func TestHandleValueJSON_Gauge_NotFound(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{}, nil)

	body, _ := json.Marshal(&models.Metrics{ID: "nonexistent", MType: models.Gauge})
	req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
