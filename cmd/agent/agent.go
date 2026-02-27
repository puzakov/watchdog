package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/puzakov/watchdog/internal/agent"
)

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

	a := agent.New(agent.Config{
		ServerAddress:  fmt.Sprintf("http://%s", addr),
		PollInterval:   time.Duration(pollInterval) * time.Second,
		ReportInterval: time.Duration(reportInterval) * time.Second,
		Timeout:        5 * time.Second, //http request timeout,
		Logger:         log.Default(),
	})
	a.Run()
}
