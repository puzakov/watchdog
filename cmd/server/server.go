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
	Addr            string `env:"ADDRESS"`
	StoreInterval   int    `env:"STORE_INTERVAL"`
	FileStoragePath string `env:"FILE_STORAGE_PATH"`
	Restore         bool   `env:"RESTORE"`
}

func main() {
	var (
		addr            string
		storeInterval   int
		fileStoragePath string
		restore         bool
	)

	flag.StringVar(&addr, "a", "localhost:8080", "server address")
	flag.IntVar(&storeInterval, "i", 300, "store interval in seconds")
	flag.StringVar(&fileStoragePath, "f", "storage.json", "file storage path")
	flag.BoolVar(&restore, "r", false, "restore data from storage flag")
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
