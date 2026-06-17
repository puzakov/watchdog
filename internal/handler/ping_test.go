package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlePing_NilConnection(t *testing.T) {
	w := httptest.NewRecorder()
	HandlePing(nil, w)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
