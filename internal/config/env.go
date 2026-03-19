package config

import (
	"flag"
	"fmt"
	"strconv"

	"github.com/caarlos0/env/v6"
)

type EnvConfig struct {
	Addr             string `env:"ADDRESS"`
	StoreInterval    string `env:"STORE_INTERVAL"` // тип string потому что int по-умолчанию 0, это влияет на логику если не задано значение
	StoreIntervalInt int
	FileStoragePath  string `env:"FILE_STORAGE_PATH"`
	Restore          bool   `env:"RESTORE"`
	DatabaseDsn      string `env:"DATABASE_DSN"`
}

func AppConfig() *EnvConfig {
	var (
		addr            string
		storeInterval   int
		fileStoragePath string
		restore         bool
		databaseDsn     string
	)

	flag.StringVar(&addr, "a", "localhost:8080", "server address")
	flag.IntVar(&storeInterval, "i", 300, "store interval in seconds")
	flag.StringVar(&fileStoragePath, "f", "storage.json", "file storage path")
	flag.BoolVar(&restore, "r", false, "restore data from storage flag")
	flag.StringVar(&databaseDsn, "d", "postgresql://watchdog_db_user:secret@localhost:5434/watchdog_db_app", "Database connection string")
	flag.Parse()

	var cfg EnvConfig
	err := env.Parse(&cfg)
	if err != nil {
		fmt.Println(err.Error())
	}

	if cfg.Addr != "" {
		addr = cfg.Addr
	}
	if cfg.StoreInterval != "" {
		if v, err := strconv.Atoi(cfg.StoreInterval); err != nil {
			fmt.Println(err.Error())
		} else {
			storeInterval = v
		}
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

	return &EnvConfig{Addr: addr, StoreIntervalInt: storeInterval, FileStoragePath: fileStoragePath, Restore: restore, DatabaseDsn: databaseDsn}
}
