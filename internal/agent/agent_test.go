package agent

import (
	"encoding/json"
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
	type sentMetric struct {
		Path string
		M    models.Metrics
	}
	var sent []sentMetric

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		var m models.Metrics
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
	a.store.UpdateGauge("RandomValue", 1.0)
	a.store.UpdateCounter("PollCount", 5)

	a.reportOnce()

	var gotDelta1 *int64
	for _, s := range sent {
		if s.Path != "/update" {
			t.Fatalf("unexpected path %q", s.Path)
		}
		if s.M.ID == "PollCount" && s.M.MType == models.Counter {
			gotDelta1 = s.M.Delta
		}
	}
	if gotDelta1 == nil || *gotDelta1 != 5 {
		t.Fatalf("first report should send delta=5, got sent=%+v", sent)
	}

	// Второй раз без изменений — counter не должен отправляться.
	sent = nil

	a.reportOnce()

	for _, s := range sent {
		if s.M.ID == "PollCount" && s.M.MType == models.Counter {
			t.Fatalf("second report should not send counter, got sent=%+v", sent)
		}
	}

	// Увеличили counter на 2 — должен уйти delta=2.
	a.store.UpdateCounter("PollCount", 2)

	sent = nil

	a.reportOnce()

	var gotDelta3 *int64
	for _, s := range sent {
		if s.M.ID == "PollCount" && s.M.MType == models.Counter {
			gotDelta3 = s.M.Delta
		}
	}
	if gotDelta3 == nil || *gotDelta3 != 2 {
		// Отсортируем для более стабильного вывода в ошибке
		sort.Slice(sent, func(i, j int) bool { return sent[i].M.ID < sent[j].M.ID })
		t.Fatalf("third report should send delta=2, got sent=%+v", sent)
	}
}
