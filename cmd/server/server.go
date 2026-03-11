package main

import (
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/caarlos0/env/v6"
	"github.com/puzakov/watchdog/internal/handler"
	"github.com/puzakov/watchdog/internal/middleware"
	"github.com/puzakov/watchdog/internal/service"
)

type EnvConfig struct {
	Addr            string `env:"ADDRESS"`
	StoreInterval   string `env:"STORE_INTERVAL"` // тип string потому что int по-умолчанию 0, это влияет на логику если не задано значение
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

	switch {
	case cfg.Addr != "":
		addr = cfg.Addr
	case cfg.StoreInterval != "":
		if v, err := strconv.Atoi(cfg.StoreInterval); err != nil {
			fmt.Println(err.Error())
		} else {
			storeInterval = v
		}
	case cfg.FileStoragePath != "":
		fileStoragePath = cfg.FileStoragePath
	case cfg.Restore == true:
		restore = true
	}

	storage := service.NewMemStorage()
	fs := service.NewFileStore(fileStoragePath)
	if restore {
		if err := fs.Restore(storage); err != nil {
			fmt.Println(err.Error())
		}
	}

	if storeInterval == 0 {
		storage = service.NewPersistingStorage(storage, fs)
	} else if storeInterval > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(storeInterval) * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					_ = fs.Save(storage.Snapshot())
				}
			}
		}()
	}

	h := handler.NewHandler(storage)
	h = middleware.Gzip(h)
	h = middleware.LogRequest(h)

	err = http.ListenAndServe(addr, h)
	if err != nil {
		panic(err)
	}
}
