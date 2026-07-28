package main

import (
	"crypto/rsa"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/caarlos0/env/v6"
	"github.com/puzakov/watchdog/internal/agent"
	"github.com/puzakov/watchdog/internal/build"
	"github.com/puzakov/watchdog/internal/config"
	"github.com/puzakov/watchdog/internal/crypto"
)

type EnvConfig struct {
	Addr           string `env:"ADDRESS"`
	GRPCAddress    string `env:"GRPC_ADDRESS"`
	GRPCTLSCA      string `env:"GRPC_TLS_CA"`
	PollInterval   int    `env:"POLL_INTERVAL"`
	ReportInterval int    `env:"REPORT_INTERVAL"`
	Key            string `env:"KEY"`
	CryptoKey      string `env:"CRYPTO_KEY"`
	RateLimit      int    `env:"RATE_LIMIT"`
}

func main() {
	build.PrintInfo()

	// Priority: flags > env vars > config file.
	// Load config file first (lowest priority) to use its values as flag defaults.
	var fileCfg *config.AgentConfigFile
	if cfgPath := config.ResolveConfigPath(); cfgPath != "" {
		var err error
		fileCfg, err = config.LoadAgentConfigFile(cfgPath)
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
	loadInt := func(fileVal, hardcoded int) int {
		if fileVal != 0 {
			return fileVal
		}
		return hardcoded
	}

	var (
		configFile     string
		addr           string
		grpcAddr       string
		grpcTLSca      string
		pollInterval   int
		reportInterval int
		key            string
		cryptoKey      string
		rateLimit      int
	)

	// Apply config file defaults.
	if fileCfg != nil {
		addr = fileCfg.Address
		grpcAddr = fileCfg.GRPCAddress
		key = fileCfg.Key
		cryptoKey = fileCfg.CryptoKey
		if pi, err := config.ParseDurationSec(fileCfg.PollInterval); err == nil {
			pollInterval = pi
		}
		if ri, err := config.ParseDurationSec(fileCfg.ReportInterval); err == nil {
			reportInterval = ri
		}
	}

	flag.StringVar(&configFile, "c", "", "path to config file")
	flag.StringVar(&configFile, "config", "", "path to config file")
	flag.StringVar(&addr, "a", loadStr(addr, "localhost:8080"), "server address")
	flag.IntVar(&pollInterval, "p", loadInt(pollInterval, 2), "poll interval in seconds")
	flag.IntVar(&reportInterval, "r", loadInt(reportInterval, 10), "report interval in seconds")
	flag.StringVar(&key, "k", key, "SHA256 hash key")
	flag.StringVar(&cryptoKey, "crypto-key", cryptoKey, "path to RSA public key file")
	flag.StringVar(&grpcAddr, "g", "", "gRPC server address")
	flag.StringVar(&grpcAddr, "grpc", "", "gRPC server address")
	flag.IntVar(&rateLimit, "l", 4, "max concurrent outgoing HTTP requests (worker pool size)")
	flag.Parse()

	var cfg EnvConfig
	err := env.Parse(&cfg)

	if err != nil {
		fmt.Println(err.Error())
	}
	if cfg.Addr != "" {
		addr = cfg.Addr
	}
	if cfg.PollInterval != 0 {
		pollInterval = cfg.PollInterval
	}
	if cfg.ReportInterval != 0 {
		reportInterval = cfg.ReportInterval
	}
	if cfg.Key != "" {
		key = cfg.Key
	}
	if cfg.RateLimit > 0 {
		rateLimit = cfg.RateLimit
	}
	if cfg.GRPCAddress != "" {
		grpcAddr = cfg.GRPCAddress
	}
	if cfg.GRPCTLSCA != "" {
		grpcTLSca = cfg.GRPCTLSCA
	}
	if cfg.CryptoKey != "" {
		cryptoKey = cfg.CryptoKey
	}

	var pubKey *rsa.PublicKey
	if cryptoKey != "" {
		pubKey, err = crypto.LoadPublicKey(cryptoKey)
		if err != nil {
			log.Fatalf("failed to load public key: %v", err)
		}
	}

	a := agent.New(agent.Config{
		ServerAddress:  fmt.Sprintf("http://%s", addr),
		GRPCAddress:    grpcAddr,
		GRPCTLSCA:      grpcTLSca,
		PollInterval:   time.Duration(pollInterval) * time.Second,
		ReportInterval: time.Duration(reportInterval) * time.Second,
		Timeout:        5 * time.Second, //http request timeout,
		Key:            key,
		CryptoKey:      pubKey,
		RateLimit:      rateLimit,
		Logger:         log.Default(),
	})
	a.Run()
}
