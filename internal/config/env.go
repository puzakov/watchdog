// Package config provides server configuration parsing from command-line flags
// and environment variables using the caarlos0/env library.
package config

import (
	"fmt"

	"github.com/caarlos0/env/v6"
)

// EnvConfig holds server configuration populated from environment variables and flags.
// Environment variables take precedence over flag defaults.
type EnvConfig struct {
	Addr             string `env:"ADDRESS"`
	StoreInterval    *int   `env:"STORE_INTERVAL"`
	StoreIntervalInt int
	FileStoragePath  string `env:"FILE_STORAGE_PATH"`
	Restore          bool   `env:"RESTORE"`
	DatabaseDsn      string `env:"DATABASE_DSN"`
	Key              string `env:"KEY"`
	CryptoKey        string `env:"CRYPTO_KEY"`
	AuditFile        string `env:"AUDIT_FILE"`
	AuditURL         string `env:"AUDIT_URL"`
}

// AppConfig merges flag-provided values with environment variables (env takes precedence)
// and returns a fully populated EnvConfig.
func AppConfig(addr string, storeInterval int, fileStoragePath string, restore bool, databaseDsn string, key string, auditFile string, auditURL string, cryptoKey string) *EnvConfig {
	var cfg EnvConfig
	err := env.Parse(&cfg)
	if err != nil {
		fmt.Println(err.Error())
	}

	if cfg.Addr != "" {
		addr = cfg.Addr
	}
	if cfg.StoreInterval != nil {
		storeInterval = cfg.StoreIntervalInt
	}
	if cfg.FileStoragePath != "" {
		fileStoragePath = cfg.FileStoragePath
	}
	if cfg.Restore {
		restore = true
	}
	if cfg.DatabaseDsn != "" {
		databaseDsn = cfg.DatabaseDsn
	}
	if cfg.Key != "" {
		key = cfg.Key
	}
	if cfg.AuditFile != "" {
		auditFile = cfg.AuditFile
	}
	if cfg.AuditURL != "" {
		auditURL = cfg.AuditURL
	}
	if cfg.CryptoKey != "" {
		cryptoKey = cfg.CryptoKey
	}

	return &EnvConfig{
		Addr:             addr,
		StoreIntervalInt: storeInterval,
		FileStoragePath:  fileStoragePath,
		Restore:          restore,
		DatabaseDsn:      databaseDsn,
		Key:              key,
		CryptoKey:        cryptoKey,
		AuditFile:        auditFile,
		AuditURL:         auditURL,
	}
}
