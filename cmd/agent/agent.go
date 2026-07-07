package main

import (
	"crypto/rsa"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/caarlos0/env/v6"
	"github.com/puzakov/watchdog/internal/agent"
	"github.com/puzakov/watchdog/internal/crypto"
)

// Build info — set via -ldflags at build time:
//
//	go build -ldflags "-X main.buildVersion=1.0.0 -X main.buildDate=$(date +%Y-%m-%d) -X main.buildCommit=$(git rev-parse --short HEAD)"
var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

type EnvConfig struct {
	Addr           string `env:"ADDRESS"`
	PollInterval   int    `env:"POLL_INTERVAL"`
	ReportInterval int    `env:"REPORT_INTERVAL"`
	Key            string `env:"KEY"`
	CryptoKey      string `env:"CRYPTO_KEY"`
	RateLimit      int    `env:"RATE_LIMIT"`
}

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
		addr           string
		pollInterval   int
		reportInterval int
		key            string
		cryptoKey      string
		rateLimit      int
	)

	flag.StringVar(&addr, "a", "localhost:8080", "server address")
	flag.IntVar(&pollInterval, "p", 2, "poll interval in seconds")
	flag.IntVar(&reportInterval, "r", 10, "report interval in seconds")
	flag.StringVar(&key, "k", "", "SHA256 hash key")
	flag.StringVar(&cryptoKey, "crypto-key", "", "path to RSA public key file")
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
	if cfg.CryptoKey != "" {
		cryptoKey = cfg.CryptoKey
	}

	var pubKey *rsa.PublicKey
	if cryptoKey != "" {
		pubKey, err = crypto.LoadPublicKey(cryptoKey)
		if err != nil {
			fmt.Println(err.Error())
		}
	}

	a := agent.New(agent.Config{
		ServerAddress:  fmt.Sprintf("http://%s", addr),
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
