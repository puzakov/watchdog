package config

import (
	"os"
	"testing"
)

func TestAppConfigDefaults(t *testing.T) {
	cfg := AppConfig("", 0, "", false, "", "", "", "", "", "")
	if cfg.Addr != "" {
		t.Errorf("Addr = %q, want empty", cfg.Addr)
	}
	if cfg.FileStoragePath != "" {
		t.Errorf("FileStoragePath = %q, want empty", cfg.FileStoragePath)
	}
	if cfg.Restore {
		t.Error("Restore = true, want false")
	}
}

func TestAppConfigWithFlags(t *testing.T) {
	cfg := AppConfig("localhost:9090", 60, "/tmp/metrics.json", true, "postgres://localhost/mydb", "secret-key", "/tmp/audit.log", "http://audit.local", "/path/to/key.pem", "")
	if cfg.Addr != "localhost:9090" {
		t.Errorf("Addr = %q, want %q", cfg.Addr, "localhost:9090")
	}
	if cfg.StoreIntervalInt != 60 {
		t.Errorf("StoreIntervalInt = %d, want 60", cfg.StoreIntervalInt)
	}
	if cfg.FileStoragePath != "/tmp/metrics.json" {
		t.Errorf("FileStoragePath = %q, want /tmp/metrics.json", cfg.FileStoragePath)
	}
	if !cfg.Restore {
		t.Error("Restore = false, want true")
	}
	if cfg.DatabaseDsn != "postgres://localhost/mydb" {
		t.Errorf("DatabaseDsn = %q, want postgres://localhost/mydb", cfg.DatabaseDsn)
	}
	if cfg.Key != "secret-key" {
		t.Errorf("Key = %q, want secret-key", cfg.Key)
	}
	if cfg.AuditFile != "/tmp/audit.log" {
		t.Errorf("AuditFile = %q, want /tmp/audit.log", cfg.AuditFile)
	}
	if cfg.AuditURL != "http://audit.local" {
		t.Errorf("AuditURL = %q, want http://audit.local", cfg.AuditURL)
	}
	if cfg.CryptoKey != "/path/to/key.pem" {
		t.Errorf("CryptoKey = %q, want /path/to/key.pem", cfg.CryptoKey)
	}
}

func TestAppConfigEnvOverride(t *testing.T) {
	os.Setenv("ADDRESS", "localhost:7070")
	os.Setenv("STORE_INTERVAL", "30")
	os.Setenv("FILE_STORAGE_PATH", "/env/path/metrics.json")
	os.Setenv("RESTORE", "true")
	os.Setenv("DATABASE_DSN", "postgres://envhost/db")
	os.Setenv("KEY", "env-key")
	os.Setenv("AUDIT_FILE", "/env/audit.log")
	os.Setenv("AUDIT_URL", "http://env.audit.local")
	os.Setenv("CRYPTO_KEY", "/env/key.pem")
	defer func() {
		os.Unsetenv("ADDRESS")
		os.Unsetenv("STORE_INTERVAL")
		os.Unsetenv("FILE_STORAGE_PATH")
		os.Unsetenv("RESTORE")
		os.Unsetenv("DATABASE_DSN")
		os.Unsetenv("KEY")
		os.Unsetenv("AUDIT_FILE")
		os.Unsetenv("AUDIT_URL")
		os.Unsetenv("CRYPTO_KEY")
	}()

	cfg := AppConfig("", 0, "", false, "", "", "", "", "", "")
	if cfg.Addr != "localhost:7070" {
		t.Errorf("Addr = %q, want localhost:7070", cfg.Addr)
	}
	if cfg.FileStoragePath != "/env/path/metrics.json" {
		t.Errorf("FileStoragePath = %q, want /env/path/metrics.json", cfg.FileStoragePath)
	}
	if !cfg.Restore {
		t.Error("Restore = false, want true")
	}
	if cfg.DatabaseDsn != "postgres://envhost/db" {
		t.Errorf("DatabaseDsn = %q, want postgres://envhost/db", cfg.DatabaseDsn)
	}
	if cfg.Key != "env-key" {
		t.Errorf("Key = %q, want env-key", cfg.Key)
	}
	if cfg.AuditFile != "/env/audit.log" {
		t.Errorf("AuditFile = %q, want /env/audit.log", cfg.AuditFile)
	}
	if cfg.AuditURL != "http://env.audit.local" {
		t.Errorf("AuditURL = %q, want http://env.audit.local", cfg.AuditURL)
	}
	if cfg.CryptoKey != "/env/key.pem" {
		t.Errorf("CryptoKey = %q, want /env/key.pem", cfg.CryptoKey)
	}
}

func TestAppConfigFlagDefaults(t *testing.T) {
	// When no env var is set, flag value is used
	cfg := AppConfig("localhost:3333", 0, "", false, "", "", "", "", "/flag/key.pem", "")
	if cfg.Addr != "localhost:3333" {
		t.Errorf("Addr = %q, want localhost:3333", cfg.Addr)
	}
	if cfg.CryptoKey != "/flag/key.pem" {
		t.Errorf("CryptoKey = %q, want /flag/key.pem", cfg.CryptoKey)
	}
}
