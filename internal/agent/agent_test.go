package agent

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
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

	_, c0 := a.store.Snapshot()
	if c0["PollCount"] != 0 {
		t.Fatalf("PollCount before = %d, want %d", c0["PollCount"], 0)
	}

	a.pollOnce()

	g1, c1 := a.store.Snapshot()
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
	var (
		mu    sync.Mutex
		paths []string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.EscapedPath())
		mu.Unlock()
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
	a.store.UpdateGauge("RandomValue", 1.0)
	a.store.UpdateCounter("PollCount", 5)

	a.reportOnce()

	mu.Lock()
	got1 := strings.Join(paths, "\n")
	mu.Unlock()
	if !strings.Contains(got1, "/update/counter/PollCount/5") {
		t.Fatalf("first report should send delta=5, got:\n%s", got1)
	}

	// Второй раз без изменений — counter не должен отправляться.
	mu.Lock()
	paths = nil
	mu.Unlock()

	a.reportOnce()

	mu.Lock()
	got2 := strings.Join(paths, "\n")
	mu.Unlock()
	if strings.Contains(got2, "/update/counter/PollCount/") {
		t.Fatalf("second report should not send counter, got:\n%s", got2)
	}

	// Увеличили counter на 2 — должен уйти delta=2.
	a.store.UpdateCounter("PollCount", 2)

	mu.Lock()
	paths = nil
	mu.Unlock()

	a.reportOnce()

	mu.Lock()
	got3 := strings.Join(paths, "\n")
	mu.Unlock()
	if !strings.Contains(got3, "/update/counter/PollCount/2") {
		t.Fatalf("third report should send delta=2, got:\n%s", got3)
	}
}
