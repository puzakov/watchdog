package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/caarlos0/env/v6"
	"github.com/puzakov/watchdog/internal/handler"
	"github.com/puzakov/watchdog/internal/logger"
	"github.com/puzakov/watchdog/internal/middleware"
	"github.com/puzakov/watchdog/internal/service"
)

type EnvConfig struct {
	Addr string `env:"ADDRESS"`
}

func main() {
	var addr string

	flag.StringVar(&addr, "a", "localhost:8080", "server address")
	flag.Parse()

	var cfg EnvConfig
	err := env.Parse(&cfg)
	if err != nil {
		fmt.Println(err.Error())
	}
	if cfg.Addr != "" {
		addr = cfg.Addr
	}

	if err = logger.Initialize("info"); err != nil {
		fmt.Println(err.Error())
	}

	storage := service.NewMemStorage()
	h := handler.NewHandler(storage)
	h = middleware.Gzip(h)
	h = middleware.LogRequest(h)

	err = http.ListenAndServe(addr, h)
	if err != nil {
		panic(err)
	}
}
