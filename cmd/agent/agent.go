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
}

func main() {
	var (
		addr           string
		pollInterval   int
		reportInterval int
		key            string
	)

	const (
		defaultAddr           = "localhost:8080"
		defaultPollInterval   = 2
		defaultReportInterval = 10
	)

	flag.StringVar(&addr, "a", defaultAddr, "server address")
	flag.IntVar(&pollInterval, "p", defaultPollInterval, "poll interval in seconds")
	flag.IntVar(&reportInterval, "r", defaultReportInterval, "report interval in seconds")
	flag.StringVar(&key, "k", "", "SHA256 hash key")
	flag.Parse()

	var cfg EnvConfig
	err := env.Parse(&cfg)

	if err != nil {
		fmt.Println(err.Error())
	}
	if addr == defaultAddr && cfg.Addr != "" {
		addr = cfg.Addr
	}
	if pollInterval == defaultPollInterval && cfg.PollInterval != 0 {
		pollInterval = cfg.PollInterval
	}
	if reportInterval == defaultReportInterval && cfg.ReportInterval != 0 {
		reportInterval = cfg.ReportInterval
	}
	if key == "" && cfg.Key != "" {
		key = cfg.Key
	}

	a := agent.New(agent.Config{
		ServerAddress:  fmt.Sprintf("http://%s", addr),
		PollInterval:   time.Duration(pollInterval) * time.Second,
		ReportInterval: time.Duration(reportInterval) * time.Second,
		Timeout:        5 * time.Second, //http request timeout,
		Key:            key,
		Logger:         log.Default(),
	})
	a.Run()
}
