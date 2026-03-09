package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/puzakov/watchdog/internal/service"
)

func TestHandleUpdate_BadContentType(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/1", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdate_EmptyContentType_IsOK(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/1.5", nil)
	// Content-Type empty
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleUpdate_UnknownType_BadRequest(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/update/unknown/Alloc/1", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdate_Gauge_OK(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/1.5", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	g, _ := store.Snapshot()
	if got := g["Alloc"]; got != 1.5 {
		t.Fatalf("Alloc = %v, want %v", got, 1.5)
	}
}

func TestHandleUpdate_Gauge_InvalidValue_BadRequest(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/nope", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdate_Counter_OK(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/update/counter/PollCount/10", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	_, c := store.Snapshot()
	if got := c["PollCount"]; got != 10 {
		t.Fatalf("PollCount = %v, want %v", got, 10)
	}
}

func TestHandleUpdate_Counter_InvalidValue_BadRequest(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/update/counter/PollCount/nope", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
