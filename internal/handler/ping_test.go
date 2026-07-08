package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/puzakov/watchdog/internal/db"
)

func TestHandlePing_NilConnection(t *testing.T) {
	w := httptest.NewRecorder()
	HandlePing(nil, w)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandlePing_NilPool(t *testing.T) {
	// DatabaseConnection with nil pool — Ping returns error, so 500.
	conn := &db.DatabaseConnection{Pool: nil}
	w := httptest.NewRecorder()
	HandlePing(conn, w)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
