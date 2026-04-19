package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/caarlos0/env/v6"
	"github.com/puzakov/watchdog/internal/agent"
)

type EnvConfig struct {
	Addr           string `env:"ADDRESS"`
	PollInterval   int    `env:"POLL_INTERVAL"`
	ReportInterval int    `env:"REPORT_INTERVAL"`
	Key            string `env:"KEY"`
	RateLimit      int    `env:"RATE_LIMIT"`
}

func main() {
	var (
		addr           string
		pollInterval   int
		reportInterval int
		key            string
		rateLimit      int
	)

	flag.StringVar(&addr, "a", "localhost:8080", "server address")
	flag.IntVar(&pollInterval, "p", 2, "poll interval in seconds")
	flag.IntVar(&reportInterval, "r", 10, "report interval in seconds")
	flag.StringVar(&key, "k", "", "SHA256 hash key")
	flag.IntVar(&rateLimit, "l", 4, "max concurrent outgoing HTTP requests (worker pool size)")
	flag.Parse()

	var cfg EnvConfig
	err := env.Parse(&cfg)

	if err != nil {
		fmt.Println(err.Error())
	}
	if cfg.Addr != "" {
		addr = cfg.Addr
	}
	if cfg.PollInterval != 0 {
		pollInterval = cfg.PollInterval
	}
	if cfg.ReportInterval != 0 {
		reportInterval = cfg.ReportInterval
	}
	if cfg.Key != "" {
		key = cfg.Key
	}
	if cfg.RateLimit > 0 {
		rateLimit = cfg.RateLimit
	}

	a := agent.New(agent.Config{
		ServerAddress:  fmt.Sprintf("http://%s", addr),
		PollInterval:   time.Duration(pollInterval) * time.Second,
		ReportInterval: time.Duration(reportInterval) * time.Second,
		Timeout:        5 * time.Second, //http request timeout,
		Key:            key,
		RateLimit:      rateLimit,
		Logger:         log.Default(),
	})
	a.Run()
}
