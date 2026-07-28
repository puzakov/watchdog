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
	GRPCAddr         string `env:"GRPC_ADDRESS"`
	GRPCTLSCert      string `env:"GRPC_TLS_CERT"`
	GRPCTLSKey       string `env:"GRPC_TLS_KEY"`
	StoreInterval    *int   `env:"STORE_INTERVAL"`
	StoreIntervalInt int
	FileStoragePath  string `env:"FILE_STORAGE_PATH"`
	Restore          bool   `env:"RESTORE"`
	DatabaseDsn      string `env:"DATABASE_DSN"`
	Key              string `env:"KEY"`
	CryptoKey        string `env:"CRYPTO_KEY"`
	AuditFile        string `env:"AUDIT_FILE"`
	AuditURL         string `env:"AUDIT_URL"`
	TrustedSubnet    string `env:"TRUSTED_SUBNET"`
}

// AppConfig merges flag-provided values with environment variables (env takes precedence)
// and returns a fully populated EnvConfig.
func AppConfig(addr string, grpcAddr string, storeInterval int, fileStoragePath string, restore bool, databaseDsn string, key string, auditFile string, auditURL string, cryptoKey string, trustedSubnet string) *EnvConfig {
	var cfg EnvConfig
	err := env.Parse(&cfg)
	if err != nil {
		fmt.Println(err.Error())
	}

	if cfg.Addr != "" {
		addr = cfg.Addr
	}
	if cfg.GRPCAddr != "" {
		grpcAddr = cfg.GRPCAddr
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
	if cfg.TrustedSubnet != "" {
		trustedSubnet = cfg.TrustedSubnet
	}

	return &EnvConfig{
		Addr:             addr,
		GRPCAddr:         grpcAddr,
		GRPCTLSCert:      cfg.GRPCTLSCert,
		GRPCTLSKey:       cfg.GRPCTLSKey,
		StoreIntervalInt: storeInterval,
		FileStoragePath:  fileStoragePath,
		Restore:          restore,
		DatabaseDsn:      databaseDsn,
		Key:              key,
		CryptoKey:        cryptoKey,
		AuditFile:        auditFile,
		AuditURL:         auditURL,
		TrustedSubnet:    trustedSubnet,
	}
}
