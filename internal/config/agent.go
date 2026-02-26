package config

import (
	"log"
	"time"

	"github.com/puzakov/watchdog/internal/agent"
)

const (
	host           = "http://localhost:8080"
	pollSeconds    = 2               //poll interval in seconds
	reportSeconds  = 10              //report interval in seconds
	requestTimeout = 5 * time.Second //http request timeout
)

var AgentConfig = agent.Config{
	ServerAddress:  host,
	PollInterval:   time.Duration(pollSeconds) * time.Second,
	ReportInterval: time.Duration(reportSeconds) * time.Second,
	Timeout:        requestTimeout,
	Logger:         log.Default(),
}
