package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSender_SendGauge_SendsExpectedRequest(t *testing.T) {
	var gotMethod, gotPath, gotCT string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := NewSender(SenderConfig{
		ServerAddress: srv.URL,
		Client:        srv.Client(),
	})

	if err := s.SendGauge("Alloc", 1.5); err != nil {
		t.Fatalf("SendGauge error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want %q", gotMethod, http.MethodPost)
	}
	if gotCT != "text/plain" {
		t.Fatalf("Content-Type = %q, want %q", gotCT, "text/plain")
	}
	if gotPath != "/update/gauge/Alloc/1.5" {
		t.Fatalf("path = %q, want %q", gotPath, "/update/gauge/Alloc/1.5")
	}
}

func TestSender_SendCounter_SendsExpectedRequest(t *testing.T) {
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := NewSender(SenderConfig{
		ServerAddress: srv.URL,
		Client:        srv.Client(),
	})

	if err := s.SendCounter("PollCount", 42); err != nil {
		t.Fatalf("SendCounter error: %v", err)
	}
	if gotPath != "/update/counter/PollCount/42" {
		t.Fatalf("path = %q, want %q", gotPath, "/update/counter/PollCount/42")
	}
}

func TestSender_Non200_IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	s := NewSender(SenderConfig{
		ServerAddress: srv.URL,
		Client:        srv.Client(),
	})

	if err := s.SendGauge("Alloc", 1); err == nil {
		t.Fatalf("expected error, got nil")
	}
}
