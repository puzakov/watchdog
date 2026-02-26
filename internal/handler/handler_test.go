package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/puzakov/watchdog/internal/service"
)

func TestNewHandler_RoutesToUpdate(t *testing.T) {
	store := service.NewMemStorage()
	h := NewHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/123", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	g, _ := store.Snapshot()
	if got := g["Alloc"]; got != 123 {
		t.Fatalf("Alloc = %v, want %v", got, 123)
	}
}
