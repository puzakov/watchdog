package main

import (
	"github.com/puzakov/watchdog/internal/agent"
	"github.com/puzakov/watchdog/internal/config"
)

func main() {
	a := agent.New(config.AgentConfig)
	a.Run()
}
