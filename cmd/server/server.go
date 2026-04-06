package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/puzakov/watchdog/internal/config"
	"github.com/puzakov/watchdog/internal/db"
	"github.com/puzakov/watchdog/internal/db/migrations"
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
	ctx := context.Background()

	var (
		fs   *service.FileStore
		conn *db.DatabaseConnection
	)

	// Storage selection priority:
	// 1) PostgreSQL (DATABASE_DSN / -d)
	// 2) File (FILE_STORAGE_PATH / -f)
	// 3) In-memory
	if cfg.DatabaseDsn != "" {
		c, err := db.NewDatabaseConnection(ctx, cfg.DatabaseDsn)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			conn = c
			defer conn.Close()

			sqlDB := stdlib.OpenDB(*conn.Pool.Config().ConnConfig)
			defer func(db *sql.DB) { _ = db.Close() }(sqlDB)
			if err := migrations.Up(sqlDB); err != nil {
				return err
			}

			storage = service.NewPostgresStorage(conn.Pool)
		}
	}

	if cfg.DatabaseDsn == "" && cfg.FileStoragePath != "" {
		fs = service.NewFileStore(cfg.FileStoragePath)

		if cfg.Restore {
			if err := fs.Restore(ctx, storage); err != nil {
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
					_ = fs.Save(storage.Snapshot(ctx))
				}
			}()
		}
	}

	h := handler.NewHandler(storage, conn)
	h = middleware.Gzip(h)
	h = middleware.LogRequest(h)

	return http.ListenAndServe(cfg.Addr, h)
}
