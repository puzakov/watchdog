package agent

import (
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/puzakov/watchdog/internal/service"
)

type Config struct {
	ServerAddress  string
	PollInterval   time.Duration
	ReportInterval time.Duration
	Timeout        time.Duration
	Logger         *log.Logger
}

type Agent struct {
	cfg    Config
	store  *service.MemStorage
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
			Logger: cfg.Logger,
		}),
		lastReportedCounters: make(map[string]int64),
	}
}

// Run запускает бесконечные циклы:
// - poll: обновление runtime-метрик + PollCount + RandomValue
// - report: отправка всех накопленных метрик на сервер
func (a *Agent) Run() {
	go a.pollLoop()
	a.reportLoop()
}

func (a *Agent) pollLoop() {
	for {
		gauges := ReadRuntimeGauges()
		for name, v := range gauges {
			a.store.UpdateGauge(name, v)
		}

		a.store.UpdateGauge("RandomValue", rand.Float64())
		a.store.UpdateCounter("PollCount", 1)

		time.Sleep(a.cfg.PollInterval)
	}
}

func (a *Agent) reportLoop() {
	for {
		gauges, counters := a.store.Snapshot()

		for name, v := range gauges {
			if err := a.sender.SendGauge(name, v); err != nil {
				a.cfg.Logger.Printf("send gauge %s: %v", name, err)
			}
		}

		for name, current := range counters {
			prev := a.lastReportedCounters[name]
			delta := current - prev
			if delta == 0 {
				continue
			}
			if err := a.sender.SendCounter(name, delta); err != nil {
				a.cfg.Logger.Printf("send counter %s: %v", name, err)
				continue
			}
			a.lastReportedCounters[name] = current
		}

		time.Sleep(a.cfg.ReportInterval)
	}
}
