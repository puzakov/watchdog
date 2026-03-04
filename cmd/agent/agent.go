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
	addr           string `env:"ADDRESS"`
	pollInterval   int    `env:"REPORT_INTERVAL"`
	reportInterval int    `env:"POLL_INTERVAL"`
}

func main() {
	var (
		addr           string
		pollInterval   int
		reportInterval int
	)

	flag.StringVar(&addr, "a", "localhost:8080", "server address")
	flag.IntVar(&pollInterval, "p", 2, "poll interval in seconds")
	flag.IntVar(&reportInterval, "r", 10, "report interval in seconds")
	flag.Parse()

	var cfg EnvConfig
	err := env.Parse(&cfg)

	if err != nil {
		fmt.Println(err.Error())
	}

	switch {
	case cfg.addr != "":
		addr = cfg.addr
	case cfg.pollInterval != 0:
		pollInterval = cfg.pollInterval
	case cfg.reportInterval != 0:
		reportInterval = cfg.reportInterval
	}

	a := agent.New(agent.Config{
		ServerAddress:  fmt.Sprintf("http://%s", addr),
		PollInterval:   time.Duration(pollInterval) * time.Second,
		ReportInterval: time.Duration(reportInterval) * time.Second,
		Timeout:        5 * time.Second, //http request timeout,
		Logger:         log.Default(),
	})
	a.Run()
}
