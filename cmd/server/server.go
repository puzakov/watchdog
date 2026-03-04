package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/caarlos0/env/v6"
	"github.com/puzakov/watchdog/internal/handler"
	"github.com/puzakov/watchdog/internal/service"
)

type EnvConfig struct {
	addr string `env:"ADDRESS"`
}

func main() {
	var addr string

	flag.StringVar(&addr, "a", "localhost:8080", "server address")
	flag.Parse()

	var cfg EnvConfig
	err := env.Parse(&cfg)

	if err != nil {
		fmt.Printf("%w\n", err)
	}

	if cfg.addr != "" {
		addr = cfg.addr
	}

	storage := service.NewMemStorage()
	h := handler.NewHandler(storage)

	err = http.ListenAndServe(addr, h)
	if err != nil {
		panic(err)
	}
}
