package agent

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	models "github.com/puzakov/watchdog/internal/model"
	"github.com/puzakov/watchdog/internal/service"
)

type Config struct {
	ServerAddress  string
	PollInterval   time.Duration
	ReportInterval time.Duration
	Timeout        time.Duration
	Key            string
	Logger         *log.Logger
}

type Agent struct {
	cfg    Config
	store  service.Storage
	sender *Sender

	lastReportedCounters map[string]int64
}

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

func (a *Agent) Run() {
	pollTicker := time.NewTicker(a.cfg.PollInterval)
	reportTicker := time.NewTicker(a.cfg.ReportInterval)
	defer pollTicker.Stop()
	defer reportTicker.Stop()

	// Сохраняем прежнее поведение: сразу собираем и сразу отправляем.
	a.pollOnce()
	a.reportOnce()

	for {
		select {
		case <-pollTicker.C:
			a.pollOnce()
		case <-reportTicker.C:
			a.reportOnce()
		}
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

	if len(batch) == 0 {
		return
	}

	err := a.sender.SendBatch(batch)
	if err == nil {
		for name, current := range includedCounters {
			a.lastReportedCounters[name] = current
		}
		return
	}

	if errors.Is(err, ErrEndpointUnsupported) {
		// Backward-compatible fallback for older servers.
		for _, m := range batch {
			switch m.MType {
			case models.Gauge:
				if m.Value == nil {
					continue
				}
				if err := a.sender.SendGauge(m.ID, *m.Value); err != nil {
					a.cfg.Logger.Printf("send gauge %s: %v", m.ID, err)
				}
			case models.Counter:
				if m.Delta == nil {
					continue
				}
				if err := a.sender.SendCounter(m.ID, *m.Delta); err != nil {
					a.cfg.Logger.Printf("send counter %s: %v", m.ID, err)
					continue
				}
				if current, ok := includedCounters[m.ID]; ok {
					a.lastReportedCounters[m.ID] = current
				}
			}
		}
	}
}
