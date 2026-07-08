package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/puzakov/watchdog/internal/config"
	"github.com/puzakov/watchdog/internal/logger"
)

func init() {
	_ = logger.Initialize("info")
}

func TestRun_BasicInMemory(t *testing.T) {
	cfg := &config.EnvConfig{
		Addr:             "localhost:0",
		StoreIntervalInt: 0,
		Restore:          false,
	}

	done := make(chan error, 1)
	go func() {
		done <- run(cfg, "")
	}()

	// Wait a bit for server to start, then kill it.
	time.Sleep(100 * time.Millisecond)

	// Send SIGINT to trigger graceful shutdown.
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGINT)

	select {
	case err := <-done:
		if err != nil {
			t.Logf("server exited with: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}

func TestRun_WithFileStorage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.json")

	cfg := &config.EnvConfig{
		Addr:             "localhost:0",
		StoreIntervalInt: 1, // periodic save
		FileStoragePath:  path,
		Restore:          false,
	}

	done := make(chan error, 1)
	go func() {
		done <- run(cfg, "")
	}()

	time.Sleep(100 * time.Millisecond)

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGINT)

	select {
	case err := <-done:
		if err != nil {
			t.Logf("server exited with: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}

func TestRun_WithFileStoragePersisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.json")

	cfg := &config.EnvConfig{
		Addr:             "localhost:0",
		StoreIntervalInt: 0, // persisting mode
		FileStoragePath:  path,
		Restore:          false,
	}

	done := make(chan error, 1)
	go func() {
		done <- run(cfg, "")
	}()

	time.Sleep(100 * time.Millisecond)

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGINT)

	select {
	case err := <-done:
		if err != nil {
			t.Logf("server exited with: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}

func TestRun_WithAuditFile(t *testing.T) {
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "audit.json")

	cfg := &config.EnvConfig{
		Addr:             "localhost:0",
		StoreIntervalInt: 0,
		AuditFile:        auditPath,
	}

	done := make(chan error, 1)
	go func() {
		done <- run(cfg, "")
	}()

	time.Sleep(100 * time.Millisecond)

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGINT)

	select {
	case err := <-done:
		if err != nil {
			t.Logf("server exited with: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}

func TestRun_InvalidAddress(t *testing.T) {
	cfg := &config.EnvConfig{
		Addr:             "nosuchhost.invalid:99999",
		StoreIntervalInt: 0,
	}

	done := make(chan error, 1)
	go func() {
		done <- run(cfg, "")
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error for invalid address")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server should have failed to start")
	}
}
