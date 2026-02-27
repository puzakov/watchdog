package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/puzakov/watchdog/internal/service"
)

func TestHandleUpdate_BadContentType(t *testing.T) {
	store := service.NewMemStorage()
	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/1", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	HandleUpdate(&UpdateArgs{mType: "gauge", name: "Alloc", value: "1"}, store, w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdate_UnknownType_BadRequest(t *testing.T) {
	store := service.NewMemStorage()
	req := httptest.NewRequest(http.MethodPost, "/update/unknown/Alloc/1", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	HandleUpdate(&UpdateArgs{mType: "unknown", name: "Alloc", value: "1"}, store, w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdate_Gauge_OK(t *testing.T) {
	store := service.NewMemStorage()
	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/1.5", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	HandleUpdate(&UpdateArgs{mType: "gauge", name: "Alloc", value: "1.5"}, store, w, req)
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
	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/nope", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	HandleUpdate(&UpdateArgs{mType: "gauge", name: "Alloc", value: "nope"}, store, w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdate_Counter_OK(t *testing.T) {
	store := service.NewMemStorage()
	req := httptest.NewRequest(http.MethodPost, "/update/counter/PollCount/10", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	HandleUpdate(&UpdateArgs{mType: "counter", name: "PollCount", value: "10"}, store, w, req)
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
	req := httptest.NewRequest(http.MethodPost, "/update/counter/PollCount/nope", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	HandleUpdate(&UpdateArgs{mType: "counter", name: "PollCount", value: "nope"}, store, w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
