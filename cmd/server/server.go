package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/puzakov/watchdog/internal/config"
	"github.com/puzakov/watchdog/internal/db"
	"github.com/puzakov/watchdog/internal/handler"
	"github.com/puzakov/watchdog/internal/middleware"
	"github.com/puzakov/watchdog/internal/service"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	cfg := config.AppConfig()
	storage := service.NewMemStorage()
	fs := service.NewFileStore(cfg.FileStoragePath)

	conn, err := db.NewDatabaseConnection(context.Background(), cfg.DatabaseDsn)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		defer conn.Close(context.Background())
	}

	if cfg.Restore {
		if err := fs.Restore(storage); err != nil {
			fmt.Println(err.Error())
		}
	}

	if cfg.StoreIntervalInt == 0 {
		storage = service.NewPersistingStorage(storage, fs)
	} else if cfg.StoreIntervalInt > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(cfg.StoreIntervalInt) * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				_ = fs.Save(storage.Snapshot())
			}
		}()
	}

	h := handler.NewHandler(storage, conn)
	h = middleware.Gzip(h)
	h = middleware.LogRequest(h)

	return http.ListenAndServe(cfg.Addr, h)
}
