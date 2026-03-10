package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/puzakov/watchdog/internal/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestLogRequest_DoesNotChangeResponseAndEmitsLog(t *testing.T) {
	prev := logger.Log
	t.Cleanup(func() { logger.Log = prev })

	core, observed := observer.New(zap.InfoLevel)
	logger.Log = zap.New(core)

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("hello"))
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)

	LogRequest(h).ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
	if got := rr.Body.String(); got != "hello" {
		t.Fatalf("body = %q, want %q", got, "hello")
	}

	entries := observed.All()
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want %d", len(entries), 1)
	}
	e := entries[0]
	if e.Message != "completed HTTP request" {
		t.Fatalf("log message = %q, want %q", e.Message, "completed HTTP request")
	}
	ctx := e.ContextMap()
	if ctx["uri"] != "/ping" {
		t.Fatalf("log uri = %#v, want %q", ctx["uri"], "/ping")
	}
	if ctx["method"] != "GET" {
		t.Fatalf("log method = %#v, want %q", ctx["method"], "GET")
	}
	if ctx["status"] != int64(http.StatusCreated) { // observer uses int64 for ints
		t.Fatalf("log status = %#v, want %d", ctx["status"], http.StatusCreated)
	}
	if ctx["size"] != int64(len("hello")) {
		t.Fatalf("log size = %#v, want %d", ctx["size"], len("hello"))
	}
	if _, ok := ctx["duration"]; !ok {
		t.Fatalf("expected duration field in log context")
	}
}

func TestLogRequest_LogsZeroSizeWhenNoBody(t *testing.T) {
	prev := logger.Log
	t.Cleanup(func() { logger.Log = prev })

	core, observed := observer.New(zap.InfoLevel)
	logger.Log = zap.New(core)

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nobody", nil)

	LogRequest(h).ServeHTTP(rr, req)

	entries := observed.All()
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want %d", len(entries), 1)
	}
	ctx := entries[0].ContextMap()
	if ctx["status"] != int64(http.StatusNoContent) {
		t.Fatalf("log status = %#v, want %d", ctx["status"], http.StatusNoContent)
	}
	if ctx["size"] != int64(0) {
		t.Fatalf("log size = %#v, want %d", ctx["size"], 0)
	}
}
