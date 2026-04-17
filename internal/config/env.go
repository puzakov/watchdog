package config

import (
	"fmt"

	"github.com/caarlos0/env/v6"
)

type EnvConfig struct {
	Addr             string `env:"ADDRESS"`
	StoreInterval    *int   `env:"STORE_INTERVAL"`
	StoreIntervalInt int
	FileStoragePath  string `env:"FILE_STORAGE_PATH"`
	Restore          bool   `env:"RESTORE"`
	DatabaseDsn      string `env:"DATABASE_DSN"`
	Key              string `env:"KEY"`
}

func AppConfig(addr string, storeInterval int, fileStoragePath string, restore bool, databaseDsn string, key string) *EnvConfig {
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
	if key == "" && cfg.Key != "" {
		key = cfg.Key
	}

	return &EnvConfig{Addr: addr, StoreIntervalInt: storeInterval, FileStoragePath: fileStoragePath, Restore: restore, DatabaseDsn: databaseDsn, Key: key}
}
