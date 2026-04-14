package main

import (
	"context"
	"database/sql"
	"flag"
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
	var (
		addr            string
		storeInterval   int
		fileStoragePath string
		restore         bool
		databaseDsn     string
		key             string
	)

	flag.StringVar(&addr, "a", "localhost:8080", "server address")
	flag.IntVar(&storeInterval, "i", 300, "store interval in seconds")
	// Путь к файлу по-умолчанию пустой: файловое хранилище включается только
	// при явном задании флага -f или переменной окружения FILE_STORAGE_PATH.
	flag.StringVar(&fileStoragePath, "f", "", "file storage path")
	flag.BoolVar(&restore, "r", false, "restore data from storage flag")
	flag.StringVar(&databaseDsn, "d", "", "Database connection string")
	flag.StringVar(&key, "k", "", "SHA256 hash key")
	flag.Parse()

	cfg := config.AppConfig(addr, storeInterval, fileStoragePath, restore, databaseDsn, key)

	if err := run(cfg); err != nil {
		panic(err)
	}
}

func run(cfg *config.EnvConfig) error {
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
	h = middleware.HashSHA256(cfg.Key, h)
	h = middleware.Gzip(h)
	h = middleware.LogRequest(h)

	return http.ListenAndServe(cfg.Addr, h)
}
