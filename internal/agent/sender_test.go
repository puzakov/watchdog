package agent

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"syscall"
	"testing"

	models "github.com/puzakov/watchdog/internal/model"
)

func TestSender_SendGauge_SendsExpectedRequest(t *testing.T) {
	var gotMethod, gotPath, gotCT, gotCE, gotHash string
	var got models.Metrics

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotCT = r.Header.Get("Content-Type")
		gotCE = r.Header.Get("Content-Encoding")
		gotHash = r.Header.Get("HashSHA256")
		defer r.Body.Close()

		var body []byte
		if gotCE == "gzip" {
			gzr, err := gzip.NewReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			body, _ = io.ReadAll(gzr)
			_ = gzr.Close()
		} else {
			body, _ = io.ReadAll(r.Body)
		}
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := NewSender(SenderConfig{
		ServerAddress: srv.URL,
		Client:        srv.Client(),
		Key:           "secret",
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
	if gotCE != "gzip" {
		t.Fatalf("Content-Encoding = %q, want %q", gotCE, "gzip")
	}
	if gotPath != "/update" {
		t.Fatalf("path = %q, want %q", gotPath, "/update")
	}
	if gotHash == "" {
		t.Fatalf("HashSHA256 header is empty, want non-empty")
	}
	if got.ID != "Alloc" || got.MType != models.Gauge || got.Value == nil || *got.Value != 1.5 || got.Delta != nil {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestSender_SendCounter_SendsExpectedRequest(t *testing.T) {
	var gotPath string
	var gotCT string
	var gotCE string
	var got models.Metrics

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotCT = r.Header.Get("Content-Type")
		gotCE = r.Header.Get("Content-Encoding")
		defer r.Body.Close()

		var body []byte
		if gotCE == "gzip" {
			gzr, err := gzip.NewReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			body, _ = io.ReadAll(gzr)
			_ = gzr.Close()
		} else {
			body, _ = io.ReadAll(r.Body)
		}
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
	if gotCE != "gzip" {
		t.Fatalf("Content-Encoding = %q, want %q", gotCE, "gzip")
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

func TestSender_SendBatch_SendsExpectedRequest(t *testing.T) {
	var gotPath, gotCT, gotCE string
	var got []models.Metrics

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotCT = r.Header.Get("Content-Type")
		gotCE = r.Header.Get("Content-Encoding")
		defer r.Body.Close()

		var body []byte
		if gotCE == "gzip" {
			gzr, err := gzip.NewReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			body, _ = io.ReadAll(gzr)
			_ = gzr.Close()
		} else {
			body, _ = io.ReadAll(r.Body)
		}
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := NewSender(SenderConfig{
		ServerAddress: srv.URL,
		Client:        srv.Client(),
	})

	v := 1.5
	d := int64(42)
	batch := []models.Metrics{
		{ID: "Alloc", MType: models.Gauge, Value: &v},
		{ID: "PollCount", MType: models.Counter, Delta: &d},
	}

	if err := s.SendBatch(batch); err != nil {
		t.Fatalf("SendBatch error: %v", err)
	}
	if gotCT != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", gotCT, "application/json")
	}
	if gotCE != "gzip" {
		t.Fatalf("Content-Encoding = %q, want %q", gotCE, "gzip")
	}
	if gotPath != "/updates" {
		t.Fatalf("path = %q, want %q", gotPath, "/updates")
	}
	if len(got) != 2 {
		t.Fatalf("payload len = %d, want %d", len(got), 2)
	}
	if got[0].ID != "Alloc" || got[0].MType != models.Gauge || got[0].Value == nil || *got[0].Value != 1.5 || got[0].Delta != nil {
		t.Fatalf("unexpected payload[0]: %+v", got[0])
	}
	if got[1].ID != "PollCount" || got[1].MType != models.Counter || got[1].Delta == nil || *got[1].Delta != 42 || got[1].Value != nil {
		t.Fatalf("unexpected payload[1]: %+v", got[1])
	}
}

func TestSender_SendBatch_EmptySlice(t *testing.T) {
	s := NewSender(SenderConfig{
		ServerAddress: "http://localhost:8080",
	})
	if err := s.SendBatch(nil); err != nil {
		t.Fatalf("SendBatch with nil slice should return nil, got %v", err)
	}
	if err := s.SendBatch([]models.Metrics{}); err != nil {
		t.Fatalf("SendBatch with empty slice should return nil, got %v", err)
	}
}

func TestSender_SendBatch_EndpointUnsupported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	s := NewSender(SenderConfig{
		ServerAddress: srv.URL,
		Client:        srv.Client(),
	})

	v := 1.0
	err := s.SendBatch([]models.Metrics{
		{ID: "test", MType: models.Gauge, Value: &v},
	})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !errors.Is(err, ErrEndpointUnsupported) {
		t.Fatalf("expected ErrEndpointUnsupported, got %v", err)
	}
}

func TestSender_SendBatch_MethodNotAllowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	t.Cleanup(srv.Close)

	s := NewSender(SenderConfig{
		ServerAddress: srv.URL,
		Client:        srv.Client(),
	})

	v := 1.0
	err := s.SendBatch([]models.Metrics{
		{ID: "test", MType: models.Gauge, Value: &v},
	})
	if err == nil {
		t.Fatal("expected error for 405 response")
	}
	if !errors.Is(err, ErrEndpointUnsupported) {
		t.Fatalf("expected ErrEndpointUnsupported, got %v", err)
	}
}

func TestIsRetriableConnectError(t *testing.T) {
	// Plain error — not retriable.
	if isRetriableConnectError(errors.New("plain error")) {
		t.Error("plain error should not be retriable")
	}

	// url.Error wrapping a dial error — retriable.
	dialErr := &url.Error{Op: "Post", URL: "http://localhost:8080", Err: &net.OpError{Op: "dial", Err: errors.New("connection refused")}}
	if !isRetriableConnectError(dialErr) {
		t.Error("dial error should be retriable")
	}

	// url.Error wrapping a non-dial net.OpError — not retriable.
	readErr := &url.Error{Op: "Post", URL: "http://localhost:8080", Err: &net.OpError{Op: "read", Err: errors.New("connection reset")}}
	if isRetriableConnectError(readErr) {
		t.Error("read error should not be retriable by op check")
	}

	// Direct syscall errors.
	if !isRetriableConnectError(syscall.ECONNREFUSED) {
		t.Error("ECONNREFUSED should be retriable")
	}
	if !isRetriableConnectError(syscall.ECONNRESET) {
		t.Error("ECONNRESET should be retriable")
	}
	if !isRetriableConnectError(syscall.EPIPE) {
		t.Error("EPIPE should be retriable")
	}
	if !isRetriableConnectError(syscall.ETIMEDOUT) {
		t.Error("ETIMEDOUT should be retriable")
	}
	if !isRetriableConnectError(syscall.ENETUNREACH) {
		t.Error("ENETUNREACH should be retriable")
	}
	if !isRetriableConnectError(syscall.EHOSTUNREACH) {
		t.Error("EHOSTUNREACH should be retriable")
	}

	// Non-retriable syscall error.
	if isRetriableConnectError(syscall.ENOENT) {
		t.Error("ENOENT should not be retriable")
	}
}

func TestSender_NewSender_Defaults(t *testing.T) {
	s := NewSender(SenderConfig{})
	if s.cfg.ServerAddress != "http://localhost:8080" {
		t.Fatalf("default ServerAddress = %q, want %q", s.cfg.ServerAddress, "http://localhost:8080")
	}
	if s.cfg.Client == nil {
		t.Fatal("default Client should not be nil")
	}
	if s.cfg.Logger == nil {
		t.Fatal("default Logger should not be nil")
	}
}

func TestSender_NewSender_TrimsSlash(t *testing.T) {
	s := NewSender(SenderConfig{ServerAddress: "http://example.com/"})
	if s.cfg.ServerAddress != "http://example.com" {
		t.Fatalf("ServerAddress = %q, want %q", s.cfg.ServerAddress, "http://example.com")
	}
}
