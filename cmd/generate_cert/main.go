// Command generate_cert generates a self-signed TLS certificate and key
// for gRPC communication between the agent and server.
//
// Usage:
//
//	go run ./cmd/generate_cert/ -cert server.crt -key server.key
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/puzakov/watchdog/internal/crypto"
	"github.com/puzakov/watchdog/internal/logger"
	"go.uber.org/zap"
)

func main() {
	err := logger.Initialize("debug")
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}

	certFile := flag.String("cert", "grpc.crt", "output certificate file path")
	keyFile := flag.String("key", "grpc.key", "output private key file path")
	flag.Parse()

	certPEM, keyPEM, err := crypto.GenerateCertificate()
	if err != nil {
		logger.Log.Error("error generate certificate", zap.Error(err))
		os.Exit(1)
	}

	if err := os.WriteFile(*certFile, certPEM, 0644); err != nil {
		logger.Log.Error("error writing cert", zap.Error(err))
		os.Exit(1)
	}

	if err := os.WriteFile(*keyFile, keyPEM, 0600); err != nil {
		logger.Log.Error("error writing key", zap.Error(err))
		os.Exit(1)
	}
	logger.Log.Info("keys written success", zap.String("cert", *certFile), zap.String("key", *keyFile))
}
