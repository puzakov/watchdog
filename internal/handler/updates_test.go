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

func TestHandleUpdatesJSON_BadContentType(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{})

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader([]byte("[]")))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdatesJSON_OK(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store, &db.DatabaseConnection{})

	v := 1.5
	d1 := int64(10)
	d2 := int64(5)
	reqBody, _ := json.Marshal([]models.Metrics{
		{ID: "Alloc", MType: models.Gauge, Value: &v},
		{ID: "PollCount", MType: models.Counter, Delta: &d1},
		// duplicate counter in same batch: should be accepted and applied
		{ID: "PollCount", MType: models.Counter, Delta: &d2},
	})

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	g, c := store.Snapshot(context.Background())
	if got := g["Alloc"]; got != 1.5 {
		t.Fatalf("Alloc = %v, want %v", got, 1.5)
	}
	if got := c["PollCount"]; got != 15 {
		t.Fatalf("PollCount = %v, want %v", got, 15)
	}
}
