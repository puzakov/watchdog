package main

import (
	"context"
	"crypto/rsa"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/puzakov/watchdog/internal/audit"
	"github.com/puzakov/watchdog/internal/build"
	"github.com/puzakov/watchdog/internal/config"
	"github.com/puzakov/watchdog/internal/db"
	"github.com/puzakov/watchdog/internal/db/migrations"
	grpcserver "github.com/puzakov/watchdog/internal/grpcserver"
	"github.com/puzakov/watchdog/internal/handler"
	"github.com/puzakov/watchdog/internal/logger"
	"github.com/puzakov/watchdog/internal/middleware"
	"github.com/puzakov/watchdog/internal/service"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/puzakov/watchdog/internal/crypto"
	proto "github.com/puzakov/watchdog/internal/proto"
)

func main() {
	build.PrintInfo()

	// Priority: flags > env vars > config file.
	// Load config file first (lowest priority) to use its values as flag defaults.
	var fileCfg *config.ServerConfigFile
	if cfgPath := config.ResolveConfigPath(); cfgPath != "" {
		var err error
		fileCfg, err = config.LoadServerConfigFile(cfgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		}
	}

	// Build flag defaults: config file value (if set) → hardcoded default.
	loadStr := func(fileVal, hardcoded string) string {
		if fileVal != "" {
			return fileVal
		}
		return hardcoded
	}
	loadInt := func(fileVal int, hardcoded int) int {
		if fileVal != 0 {
			return fileVal
		}
		return hardcoded
	}

	var (
		configFile      string
		addr            string
		grpcAddr        string
		storeInterval   int
		fileStoragePath string
		restore         bool
		databaseDsn     string
		key             string
		cryptoKey       string
		auditFile       string
		auditURL        string
		trustedSubnet   string
		pprofAddr       string
	)

	// Apply config file defaults.
	if fileCfg != nil {
		addr = fileCfg.Address
		grpcAddr = fileCfg.GRPCAddress
		fileStoragePath = fileCfg.StoreFile
		databaseDsn = fileCfg.DatabaseDSN
		key = fileCfg.Key
		cryptoKey = fileCfg.CryptoKey
		if fileCfg.Restore != nil {
			restore = *fileCfg.Restore
		}
		if si, err := config.ParseDurationSec(fileCfg.StoreInterval); err == nil {
			storeInterval = si
		}
	}

	flag.StringVar(&configFile, "c", "", "path to config file")
	flag.StringVar(&configFile, "config", "", "path to config file")
	flag.StringVar(&addr, "a", loadStr(addr, "localhost:8080"), "server address")
	flag.IntVar(&storeInterval, "i", loadInt(storeInterval, 300), "store interval in seconds")
	flag.StringVar(&fileStoragePath, "f", fileStoragePath, "file storage path")
	flag.BoolVar(&restore, "r", restore, "restore data from storage flag")
	flag.StringVar(&databaseDsn, "d", databaseDsn, "Database connection string")
	flag.StringVar(&key, "k", key, "SHA256 hash key")
	flag.StringVar(&cryptoKey, "crypto-key", cryptoKey, "path to RSA private key file")
	flag.StringVar(&auditFile, "audit-file", "", "audit log file path")
	flag.StringVar(&auditURL, "audit-url", "", "audit log URL")
	flag.StringVar(&grpcAddr, "g", "", "gRPC server address")
	flag.StringVar(&grpcAddr, "grpc", "", "gRPC server address")
	flag.StringVar(&trustedSubnet, "t", "", "trusted subnet CIDR")
	flag.StringVar(&pprofAddr, "pprof", "", "pprof listen address (e.g. localhost:6060)")
	flag.Parse()

	cfg := config.AppConfig(addr, grpcAddr, storeInterval, fileStoragePath, restore, databaseDsn, key, auditFile, auditURL, cryptoKey, trustedSubnet)
	_ = logger.Initialize("info")

	var privKey *rsa.PrivateKey
	if cfg.CryptoKey != "" {
		var err error
		privKey, err = crypto.LoadPrivateKey(cfg.CryptoKey)
		if err != nil {
			logger.Log.Fatal("failed to load private key: " + err.Error())
		}
		logger.Log.Info("RSA private key loaded from " + cfg.CryptoKey)
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
		auditor.Shutdown()
		_ = fileObserver.Close()
	}()

	h := handler.NewHandler(storage, conn, auditor)
	if cfg.Key != "" {
		h = middleware.HashSHA256(cfg.Key, h)
	}
	h = middleware.Gzip(h)
	if privKey != nil {
		h = middleware.DecryptRSA(privKey, h)
	}
	if cfg.TrustedSubnet != "" {
		h = middleware.CheckSubnet(cfg.TrustedSubnet, h)
	}
	h = middleware.LogRequest(h)

	var (
		grpcSrv   *grpc.Server
		grpcErrCh chan error
	)
	if cfg.GRPCAddr != "" {
		grpcLis, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			return fmt.Errorf("gRPC listen: %w", err)
		}

		grpcCreds, err := crypto.LoadOrGenerateServerCreds(cfg.GRPCTLSCert, cfg.GRPCTLSKey)
		if err != nil {
			return fmt.Errorf("gRPC TLS credentials: %w", err)
		}
		grpcSrv = grpc.NewServer(
			grpc.UnaryInterceptor(grpcserver.TrustedSubnetInterceptor(cfg.TrustedSubnet)),
			grpc.Creds(grpcCreds),
		)
		proto.RegisterMetricsServer(grpcSrv, grpcserver.NewMetricsServer(storage, auditor))
		reflection.Register(grpcSrv)

		grpcErrCh = make(chan error, 1)
		go func() {
			logger.Log.Info("gRPC server started on " + cfg.GRPCAddr)
			grpcErrCh <- grpcSrv.Serve(grpcLis)
		}()
	}

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
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		if grpcSrv != nil {
			grpcSrv.GracefulStop()
		}
		return nil
	case <-quit:
		if grpcErrCh != nil {
			// Avoid race: if the gRPC server errored before we got the signal,
			// use that error instead.
			select {
			case err := <-grpcErrCh:
				logger.Log.Error("gRPC server error", zap.Error(err))
			default:
			}
		}
		logger.Log.Info("shutting down server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown: %w", err)
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		// Gracefully stop the gRPC server.
		if grpcSrv != nil {
			grpcSrv.GracefulStop()
		}

		// Final save for periodic file persistence.
		if fs != nil && cfg.StoreIntervalInt > 0 {
			if err := fs.Save(storage.Snapshot(context.Background())); err != nil {
				logger.Log.Error("final save: " + err.Error())
			}
		}

		return nil
	}
}
