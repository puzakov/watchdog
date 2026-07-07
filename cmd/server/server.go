package main

import (
	"context"
	"crypto/rsa"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/puzakov/watchdog/internal/audit"
	"github.com/puzakov/watchdog/internal/config"
	"github.com/puzakov/watchdog/internal/crypto"
	"github.com/puzakov/watchdog/internal/db"
	"github.com/puzakov/watchdog/internal/db/migrations"
	"github.com/puzakov/watchdog/internal/handler"
	"github.com/puzakov/watchdog/internal/logger"
	"github.com/puzakov/watchdog/internal/middleware"
	"github.com/puzakov/watchdog/internal/service"
)

// Build info — set via -ldflags at build time:
//
//	go build -ldflags "-X main.buildVersion=1.0.0 -X main.buildDate=$(date +%Y-%m-%d) -X main.buildCommit=$(git rev-parse --short HEAD)"
var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func printBuildInfo() {
	valOrNA := func(s string) string {
		if s == "" {
			return "N/A"
		}
		return s
	}

	fmt.Printf("Build version: %s\n", valOrNA(buildVersion))
	fmt.Printf("Build date: %s\n", valOrNA(buildDate))
	fmt.Printf("Build commit: %s\n", valOrNA(buildCommit))
}

func main() {
	printBuildInfo()

	var (
		addr            string
		storeInterval   int
		fileStoragePath string
		restore         bool
		databaseDsn     string
		key             string
		cryptoKey       string
		auditFile       string
		auditURL        string
		pprofAddr       string
	)

	flag.StringVar(&addr, "a", "localhost:8080", "server address")
	flag.IntVar(&storeInterval, "i", 300, "store interval in seconds")
	// Путь к файлу по-умолчанию пустой: файловое хранилище включается только
	// при явном задании флага -f или переменной окружения FILE_STORAGE_PATH.
	flag.StringVar(&fileStoragePath, "f", "", "file storage path")
	flag.BoolVar(&restore, "r", false, "restore data from storage flag")
	flag.StringVar(&databaseDsn, "d", "", "Database connection string")
	flag.StringVar(&key, "k", "", "SHA256 hash key")
	flag.StringVar(&cryptoKey, "crypto-key", "", "path to RSA private key file")
	flag.StringVar(&auditFile, "audit-file", "", "audit log file path")
	flag.StringVar(&auditURL, "audit-url", "", "audit log URL")
	flag.StringVar(&pprofAddr, "pprof", "", "pprof listen address (e.g. localhost:6060)")
	flag.Parse()

	cfg := config.AppConfig(addr, storeInterval, fileStoragePath, restore, databaseDsn, key, auditFile, auditURL, cryptoKey)
	_ = logger.Initialize("info")

	var privKey *rsa.PrivateKey
	if cfg.CryptoKey != "" {
		var err error
		privKey, err = crypto.LoadPrivateKey(cfg.CryptoKey)
		if err != nil {
			logger.Log.Error("failed to load private key: " + err.Error())
		} else {
			logger.Log.Info("RSA private key loaded from " + cfg.CryptoKey)
		}
	}

	if err := run(cfg, privKey, pprofAddr); err != nil {
		logger.Log.Error(err.Error())
		return
	}
}

func run(cfg *config.EnvConfig, privKey *rsa.PrivateKey, pprofAddr string) error {
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
			logger.Log.Error(err.Error())
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

	var auditor *audit.Subject
	var fileObserver *audit.FileObserver
	if cfg.AuditFile != "" || cfg.AuditURL != "" {
		auditor = audit.NewSubject()
		if cfg.AuditFile != "" {
			fileObserver = audit.NewFileObserver(cfg.AuditFile)
			auditor.Subscribe(fileObserver)
		}
		if cfg.AuditURL != "" {
			auditor.Subscribe(audit.NewURLObserver(cfg.AuditURL))
		}
	}
	defer func() {
		_ = fileObserver.Close()
		auditor.Shutdown()
	}()

	h := handler.NewHandler(storage, conn, auditor)
	h = middleware.HashSHA256(cfg.Key, h)
	h = middleware.Gzip(h)
	h = middleware.DecryptRSA(privKey, h)
	h = middleware.LogRequest(h)

	srv := &http.Server{Addr: cfg.Addr, Handler: h}

	if pprofAddr != "" {
		go func() {
			logger.Log.Info("pprof server started on " + pprofAddr)
			if err := http.ListenAndServe(pprofAddr, nil); err != nil {
				logger.Log.Error("pprof server: " + err.Error())
			}
		}()
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-quit:
		logger.Log.Info("shutting down server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown: %w", err)
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
