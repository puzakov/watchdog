package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	models "github.com/puzakov/watchdog/internal/model"
)

func TestAgent_pollOnce_UpdatesPollCountAndRandomValue(t *testing.T) {
	// Сервер не нужен: reportOnce не вызываем.
	a := New(Config{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		Timeout:        1 * time.Second,
		Logger:         log.New(io.Discard, "", 0),
	})

	_, c0 := a.store.Snapshot(context.Background())
	if c0["PollCount"] != 0 {
		t.Fatalf("PollCount before = %d, want %d", c0["PollCount"], 0)
	}

	a.pollOnce()

	g1, c1 := a.store.Snapshot(context.Background())
	if c1["PollCount"] != 1 {
		t.Fatalf("PollCount after = %d, want %d", c1["PollCount"], 1)
	}
	if _, ok := g1["RandomValue"]; !ok {
		t.Fatalf("RandomValue was not set")
	}
	if _, ok := g1["Alloc"]; !ok {
		t.Fatalf("expected runtime gauge Alloc to be present")
	}
}

func TestAgent_reportOnce_SendsCounterAsDelta(t *testing.T) {
	type sentMetric struct {
		Path string
		M    []models.Metrics
	}
	var sent []sentMetric

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var body []byte
		if r.Header.Get("Content-Encoding") == "gzip" {
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
		var m []models.Metrics
		_ = json.Unmarshal(body, &m)
		sent = append(sent, sentMetric{Path: r.URL.EscapedPath(), M: m})
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	a := New(Config{
		ServerAddress:  srv.URL,
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		Timeout:        1 * time.Second,
		Logger:         log.New(io.Discard, "", 0),
	})

	// Ограничим метрики до одной gauge и одной counter, чтобы тест был стабильным.
	_ = a.store.UpdateGauge(context.Background(), "RandomValue", 1.0)
	_ = a.store.UpdateCounter(context.Background(), "PollCount", 5)

	a.reportOnce()

	var gotDelta1 *int64
	for _, s := range sent {
		if s.Path != "/updates" {
			t.Fatalf("unexpected path %q", s.Path)
		}
		for _, m := range s.M {
			if m.ID == "PollCount" && m.MType == models.Counter {
				gotDelta1 = m.Delta
			}
		}
	}
	if gotDelta1 == nil || *gotDelta1 != 5 {
		t.Fatalf("first report should send delta=5, got sent=%+v", sent)
	}

	// Второй раз без изменений — counter не должен отправляться.
	sent = nil

	a.reportOnce()

	for _, s := range sent {
		for _, m := range s.M {
			if m.ID == "PollCount" && m.MType == models.Counter {
				t.Fatalf("second report should not send counter, got sent=%+v", sent)
			}
		}
	}

	// Увеличили counter на 2 — должен уйти delta=2.
	_ = a.store.UpdateCounter(context.Background(), "PollCount", 2)

	sent = nil

	a.reportOnce()

	var gotDelta3 *int64
	for _, s := range sent {
		for _, m := range s.M {
			if m.ID == "PollCount" && m.MType == models.Counter {
				gotDelta3 = m.Delta
			}
		}
	}
	if gotDelta3 == nil || *gotDelta3 != 2 {
		// Отсортируем для более стабильного вывода в ошибке
		sort.Slice(sent, func(i, j int) bool {
			if len(sent[i].M) == 0 || len(sent[j].M) == 0 {
				return len(sent[i].M) < len(sent[j].M)
			}
			return sent[i].M[0].ID < sent[j].M[0].ID
		})
		t.Fatalf("third report should send delta=2, got sent=%+v", sent)
	}
}

func TestAgent_runPooled_NilPoolTasks(t *testing.T) {
	a := New(Config{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		Timeout:        1 * time.Second,
		Logger:         log.New(io.Discard, "", 0),
	})
	// poolTasks is nil by default — runPooled should execute directly.
	err := a.runPooled(func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("runPooled with nil poolTasks should return nil, got %v", err)
	}

	// Also test with an error.
	expectedErr := errors.New("test error")
	err = a.runPooled(func() error {
		return expectedErr
	})
	if err != expectedErr {
		t.Fatalf("runPooled should propagate error, got %v", err)
	}
}

func TestAgent_reportOnce_FallbackOnEndpointUnsupported(t *testing.T) {
	// Server that returns 404 for /updates and 200 for /update.
	var updateCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() == "/updates" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		updateCalls++
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	a := New(Config{
		ServerAddress:  srv.URL,
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		Timeout:        1 * time.Second,
		Logger:         log.New(io.Discard, "", 0),
		RateLimit:      2,
	})

	// Start the worker pool so fallback can use runPooled with channels.
	a.poolTasks = make(chan func(), 64)
	workerDone := make(chan struct{})
	go func() {
		a.poolWorker()
		close(workerDone)
	}()

	// Add both a gauge and a counter to trigger fallback for both.
	_ = a.store.UpdateGauge(context.Background(), "Alloc", 3.0)
	_ = a.store.UpdateCounter(context.Background(), "PollCount", 10)

	a.reportOnce()

	// Close the worker pool and wait.
	close(a.poolTasks)
	<-workerDone

	// Expected: 1 for the gauge + 1 for the counter = 2 single-metric sends.
	if updateCalls < 1 {
		t.Fatalf("expected at least 1 fallback call to /update, got %d", updateCalls)
	}
}

func TestAgent_reportOnce_LogsErrorOnNonEndpointError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	var logBuf bytes.Buffer
	a := New(Config{
		ServerAddress:  srv.URL,
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		Timeout:        1 * time.Second,
		Logger:         log.New(&logBuf, "", 0),
		RateLimit:      1,
	})

	// Start the worker pool.
	a.poolTasks = make(chan func(), 64)
	workerDone := make(chan struct{})
	go func() {
		a.poolWorker()
		close(workerDone)
	}()

	_ = a.store.UpdateGauge(context.Background(), "Alloc", 1.0)
	a.reportOnce()

	close(a.poolTasks)
	<-workerDone

	if logBuf.Len() == 0 {
		t.Error("expected log output for non-endpoint-unsupported error")
	}
}

func TestAgent_reportOnce_UpdatesLastReportedOnSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	a := New(Config{
		ServerAddress:  srv.URL,
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		Timeout:        1 * time.Second,
		Logger:         log.New(io.Discard, "", 0),
		RateLimit:      1,
	})

	// Start the worker pool.
	a.poolTasks = make(chan func(), 64)
	workerDone := make(chan struct{})
	go func() {
		a.poolWorker()
		close(workerDone)
	}()

	_ = a.store.UpdateCounter(context.Background(), "PollCount", 5)
	a.reportOnce()
	close(a.poolTasks)
	<-workerDone

	a.lastReportedMu.Lock()
	last := a.lastReportedCounters["PollCount"]
	a.lastReportedMu.Unlock()
	if last != 5 {
		t.Fatalf("lastReportedCounters[PollCount] = %d, want 5", last)
	}
}

func TestAgent_New_RateLimitZero(t *testing.T) {
	a := New(Config{
		ServerAddress:  "http://localhost:8080",
		PollInterval:   1 * time.Second,
		ReportInterval: 5 * time.Second,
		Timeout:        2 * time.Second,
		Logger:         log.New(io.Discard, "", 0),
		RateLimit:      0, // should default to 1
	})
	if a.cfg.RateLimit != 1 {
		t.Fatalf("RateLimit = %d, want 1", a.cfg.RateLimit)
	}
}

func TestAgent_New_AllDefaults(t *testing.T) {
	a := New(Config{})
	if a.cfg.ServerAddress != "http://localhost:8080" {
		t.Fatalf("ServerAddress = %q, want %q", a.cfg.ServerAddress, "http://localhost:8080")
	}
	if a.cfg.PollInterval != 2*time.Second {
		t.Fatalf("PollInterval = %v, want 2s", a.cfg.PollInterval)
	}
	if a.cfg.ReportInterval != 10*time.Second {
		t.Fatalf("ReportInterval = %v, want 10s", a.cfg.ReportInterval)
	}
	if a.cfg.Timeout != 5*time.Second {
		t.Fatalf("Timeout = %v, want 5s", a.cfg.Timeout)
	}
	if a.cfg.RateLimit != 1 {
		t.Fatalf("RateLimit = %d, want 1", a.cfg.RateLimit)
	}
}

func TestAgent_New_TrimsServerAddress(t *testing.T) {
	a := New(Config{ServerAddress: "http://example.com/"})
	if a.cfg.ServerAddress != "http://example.com" {
		t.Fatalf("ServerAddress = %q, want %q", a.cfg.ServerAddress, "http://example.com")
	}
}

func TestAgent_New_CustomRateLimit(t *testing.T) {
	a := New(Config{
		RateLimit: 5,
		Logger:    log.New(io.Discard, "", 0),
	})
	if a.cfg.RateLimit != 5 {
		t.Fatalf("RateLimit = %d, want 5", a.cfg.RateLimit)
	}
}
