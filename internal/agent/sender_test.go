package agent

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	models "github.com/puzakov/watchdog/internal/model"
)

func TestSender_SendGauge_SendsExpectedRequest(t *testing.T) {
	var gotMethod, gotPath, gotCT string
	var got models.Metrics

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotCT = r.Header.Get("Content-Type")
		defer r.Body.Close()

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
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
	if gotCT != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", gotCT, "application/json")
	}
	if gotPath != "/update" {
		t.Fatalf("path = %q, want %q", gotPath, "/update")
	}
	if got.ID != "Alloc" || got.MType != models.Gauge || got.Value == nil || *got.Value != 1.5 || got.Delta != nil {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestSender_SendCounter_SendsExpectedRequest(t *testing.T) {
	var gotPath string
	var gotCT string
	var got models.Metrics

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotCT = r.Header.Get("Content-Type")
		defer r.Body.Close()

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
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
	if gotCT != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", gotCT, "application/json")
	}
	if gotPath != "/update" {
		t.Fatalf("path = %q, want %q", gotPath, "/update")
	}
	if got.ID != "PollCount" || got.MType != models.Counter || got.Delta == nil || *got.Delta != 42 || got.Value != nil {
		t.Fatalf("unexpected payload: %+v", got)
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
