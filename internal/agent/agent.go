// Package agent implements the metrics collection agent that polls Go runtime
// and host metrics and sends them to a metrics server.
package agent

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	models "github.com/puzakov/watchdog/internal/model"
	"github.com/puzakov/watchdog/internal/service"
)

// Config configures the metrics collection agent.
// generate:reset
type Config struct {
	// ServerAddress is the base URL of the metrics server (e.g. http://localhost:8080).
	ServerAddress string
	// PollInterval is how often to collect runtime and host metrics.
	PollInterval time.Duration
	// ReportInterval is how often to send collected metrics to the server.
	ReportInterval time.Duration
	// Timeout is the HTTP client timeout for sending metrics.
	Timeout time.Duration
	// Key used for SHA256 request signing (empty = no signing).
	Key string
	// RateLimit is the maximum number of concurrent outgoing HTTP requests (worker pool size).
	RateLimit int
	// Logger for agent diagnostics. If nil, log.Default() is used.
	Logger *log.Logger
}

// Agent collects runtime and host metrics and periodically reports them to a server.
// generate:reset
type Agent struct {
	cfg    Config
	store  service.Storage
	sender *Sender

	lastReportedMu       sync.Mutex
	lastReportedCounters map[string]int64

	poolTasks chan func()
}

// New creates and configures an Agent with sensible defaults for zero-valued fields.
func New(cfg Config) *Agent {
	if cfg.ServerAddress == "" {
		cfg.ServerAddress = "http://localhost:8080"
	}
	cfg.ServerAddress = strings.TrimRight(cfg.ServerAddress, "/")
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 2 * time.Second
	}
	if cfg.ReportInterval == 0 {
		cfg.ReportInterval = 10 * time.Second
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	if cfg.RateLimit < 1 {
		cfg.RateLimit = 1
	}

	return &Agent{
		cfg:   cfg,
		store: service.NewMemStorage(),
		sender: NewSender(SenderConfig{
			ServerAddress: cfg.ServerAddress,
			Client: &http.Client{
				Timeout: cfg.Timeout,
			},
			Key:    cfg.Key,
			Logger: cfg.Logger,
		}),
		lastReportedCounters: make(map[string]int64),
	}
}

// Run starts runtime polling, host metrics polling, reporting, and an HTTP worker pool.
// It blocks forever (until the process is terminated).
func (a *Agent) Run() {
	workers := a.cfg.RateLimit
	if workers < 1 {
		workers = 1
	}
	a.poolTasks = make(chan func(), workers*64)
	for i := 0; i < workers; i++ {
		go a.poolWorker()
	}

	go a.pollRuntimeLoop()
	go a.pollHostLoop()
	go a.reportLoop()

	select {}
}

func (a *Agent) poolWorker() {
	for fn := range a.poolTasks {
		if fn != nil {
			fn()
		}
	}
}

func (a *Agent) runPooled(fn func() error) error {
	if a.poolTasks == nil {
		return fn()
	}
	errCh := make(chan error, 1)
	a.poolTasks <- func() {
		errCh <- fn()
	}
	return <-errCh
}

func (a *Agent) pollRuntimeLoop() {
	a.pollOnce()
	t := time.NewTicker(a.cfg.PollInterval)
	defer t.Stop()
	for range t.C {
		a.pollOnce()
	}
}

func (a *Agent) pollHostLoop() {
	ctx := context.Background()
	a.collectHostOnce(ctx)
	t := time.NewTicker(a.cfg.PollInterval)
	defer t.Stop()
	for range t.C {
		a.collectHostOnce(ctx)
	}
}

func (a *Agent) collectHostOnce(ctx context.Context) {
	for name, v := range CollectHostGauges(ctx) {
		_ = a.store.UpdateGauge(ctx, name, v)
	}
}

func (a *Agent) reportLoop() {
	a.reportOnce()
	t := time.NewTicker(a.cfg.ReportInterval)
	defer t.Stop()
	for range t.C {
		a.reportOnce()
	}
}

func (a *Agent) pollOnce() {
	ctx := context.Background()
	gauges := ReadRuntimeGauges()
	for name, v := range gauges {
		_ = a.store.UpdateGauge(ctx, name, v)
	}

	_ = a.store.UpdateGauge(ctx, "RandomValue", rand.Float64())
	_ = a.store.UpdateCounter(ctx, "PollCount", 1)
}

func (a *Agent) reportOnce() {
	ctx := context.Background()
	gauges, counters := a.store.Snapshot(ctx)

	batch := make([]models.Metrics, 0, len(gauges)+len(counters))
	for name, v := range gauges {
		vv := v
		batch = append(batch, models.Metrics{ID: name, MType: models.Gauge, Value: &vv})
	}

	a.lastReportedMu.Lock()
	includedCounters := make(map[string]int64)
	for name, current := range counters {
		prev := a.lastReportedCounters[name]
		delta := current - prev
		if delta == 0 {
			continue
		}
		dd := delta
		batch = append(batch, models.Metrics{ID: name, MType: models.Counter, Delta: &dd})
		includedCounters[name] = current
	}
	a.lastReportedMu.Unlock()

	if len(batch) == 0 {
		return
	}

	err := a.runPooled(func() error {
		return a.sender.SendBatch(batch)
	})
	if err == nil {
		a.lastReportedMu.Lock()
		for name, current := range includedCounters {
			a.lastReportedCounters[name] = current
		}
		a.lastReportedMu.Unlock()
		return
	}

	if !errors.Is(err, ErrEndpointUnsupported) {
		a.cfg.Logger.Printf("send batch: %v", err)
		return
	}

	// Backward-compatible fallback: parallel single-metric sends, bounded by worker pool.
	var wg sync.WaitGroup
	for _, m := range batch {
		m := m
		wg.Add(1)
		go func() {
			defer wg.Done()
			var sendErr error
			switch m.MType {
			case models.Gauge:
				if m.Value == nil {
					return
				}
				sendErr = a.runPooled(func() error {
					return a.sender.SendGauge(m.ID, *m.Value)
				})
			case models.Counter:
				if m.Delta == nil {
					return
				}
				sendErr = a.runPooled(func() error {
					return a.sender.SendCounter(m.ID, *m.Delta)
				})
			default:
				return
			}
			if sendErr != nil {
				a.cfg.Logger.Printf("send metric %s: %v", m.ID, sendErr)
				return
			}
			if m.MType == models.Counter {
				if current, ok := includedCounters[m.ID]; ok {
					a.lastReportedMu.Lock()
					a.lastReportedCounters[m.ID] = current
					a.lastReportedMu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
}
