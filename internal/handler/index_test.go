package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/puzakov/watchdog/internal/service"
)

func TestHandleIndex_ReturnsHTMLAndContainsMetrics(t *testing.T) {
	store := service.NewMemStorage()
	_ = store.UpdateGauge("Alloc", 1.5)
	_ = store.UpdateCounter("PollCount", 2)

	w := httptest.NewRecorder()
	HandleIndex(store, w)

	resp := w.Result()
	err := resp.Body.Close()
	if err != nil {
		t.Errorf("%v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want prefix %q", ct, "text/html")
	}

	body := w.Body.String()
	if !strings.Contains(body, "<!doctype html>") {
		t.Fatalf("body does not look like HTML document, body:\n%s", body)
	}
	if !strings.Contains(body, "<h2>Gauge</h2>") || !strings.Contains(body, "<h2>Counter</h2>") {
		t.Fatalf("body does not contain expected sections, body:\n%s", body)
	}
	if !strings.Contains(body, "Alloc") || !strings.Contains(body, "1.5") {
		t.Fatalf("body does not contain gauge metric, body:\n%s", body)
	}
	if !strings.Contains(body, "PollCount") || !strings.Contains(body, "2") {
		t.Fatalf("body does not contain counter metric, body:\n%s", body)
	}
}
