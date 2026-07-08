package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeJSON(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// --- Server config file ---

func TestLoadServerConfigFile_Success(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.json")
	writeJSON(t, path, `{
		"address": "localhost:9090",
		"restore": true,
		"store_interval": "60s",
		"store_file": "/tmp/metrics.json",
		"database_dsn": "postgres://localhost/db",
		"crypto_key": "/tmp/key.pem",
		"key": "secret"
	}`)

	cfg, err := LoadServerConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Address != "localhost:9090" {
		t.Errorf("Address = %q, want %q", cfg.Address, "localhost:9090")
	}
	if cfg.Restore == nil || !*cfg.Restore {
		t.Error("Restore = false, want true")
	}
	if cfg.StoreInterval != "60s" {
		t.Errorf("StoreInterval = %q, want %q", cfg.StoreInterval, "60s")
	}
	if cfg.StoreFile != "/tmp/metrics.json" {
		t.Errorf("StoreFile = %q, want %q", cfg.StoreFile, "/tmp/metrics.json")
	}
	if cfg.DatabaseDSN != "postgres://localhost/db" {
		t.Errorf("DatabaseDSN = %q, want %q", cfg.DatabaseDSN, "postgres://localhost/db")
	}
	if cfg.CryptoKey != "/tmp/key.pem" {
		t.Errorf("CryptoKey = %q, want %q", cfg.CryptoKey, "/tmp/key.pem")
	}
	if cfg.Key != "secret" {
		t.Errorf("Key = %q, want %q", cfg.Key, "secret")
	}
}

func TestLoadServerConfigFile_Minimal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "minimal.json")
	writeJSON(t, path, `{"address": "localhost:8080"}`)

	cfg, err := LoadServerConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Address != "localhost:8080" {
		t.Errorf("Address = %q, want %q", cfg.Address, "localhost:8080")
	}
	if cfg.Restore != nil {
		t.Error("Restore should be nil when not set")
	}
}

func TestLoadServerConfigFile_EmptyRestore(t *testing.T) {
	// restore: false should not override a true default (it's omitted when false
	// due to omitempty, but explicit false is still valid).
	path := filepath.Join(t.TempDir(), "restore.json")
	writeJSON(t, path, `{"address": "x", "restore": false}`)

	cfg, err := LoadServerConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Restore == nil {
		t.Fatal("Restore should not be nil when explicitly set")
	}
	if *cfg.Restore {
		t.Error("Restore = true, want false")
	}
}

func TestLoadServerConfigFile_FileNotFound(t *testing.T) {
	_, err := LoadServerConfigFile("/nonexistent/config.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadServerConfigFile_EmptyPath(t *testing.T) {
	_, err := LoadServerConfigFile("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestLoadServerConfigFile_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	writeJSON(t, path, `{invalid}`)

	_, err := LoadServerConfigFile(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- Agent config file ---

func TestLoadAgentConfigFile_Success(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.json")
	writeJSON(t, path, `{
		"address": "localhost:9090",
		"report_interval": "30s",
		"poll_interval": "5s",
		"crypto_key": "/tmp/pub.pem",
		"key": "mykey"
	}`)

	cfg, err := LoadAgentConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Address != "localhost:9090" {
		t.Errorf("Address = %q, want %q", cfg.Address, "localhost:9090")
	}
	if cfg.ReportInterval != "30s" {
		t.Errorf("ReportInterval = %q, want %q", cfg.ReportInterval, "30s")
	}
	if cfg.PollInterval != "5s" {
		t.Errorf("PollInterval = %q, want %q", cfg.PollInterval, "5s")
	}
	if cfg.CryptoKey != "/tmp/pub.pem" {
		t.Errorf("CryptoKey = %q, want %q", cfg.CryptoKey, "/tmp/pub.pem")
	}
	if cfg.Key != "mykey" {
		t.Errorf("Key = %q, want %q", cfg.Key, "mykey")
	}
}

func TestLoadAgentConfigFile_Minimal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "minimal.json")
	writeJSON(t, path, `{}`)

	cfg, err := LoadAgentConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != "" {
		t.Errorf("Address = %q, want empty", cfg.Address)
	}
}

func TestLoadAgentConfigFile_FileNotFound(t *testing.T) {
	_, err := LoadAgentConfigFile("/nonexistent/agent.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadAgentConfigFile_EmptyPath(t *testing.T) {
	_, err := LoadAgentConfigFile("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

// --- ParseDurationSec ---

func TestParseDurationSec(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"0s", 0},
		{"1s", 1},
		{"30s", 30},
		{"60s", 60},
		{"300s", 300},
		{"1m", 60},
		{"5m", 300},
		{"1h", 3600},
		{"1m30s", 90},
	}

	for _, tt := range tests {
		got, err := ParseDurationSec(tt.input)
		if err != nil {
			t.Errorf("ParseDurationSec(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseDurationSec(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseDurationSec_Invalid(t *testing.T) {
	_, err := ParseDurationSec("not-a-duration")
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestParseDurationSec_Negative(t *testing.T) {
	_, err := ParseDurationSec("-5s")
	if err == nil {
		t.Fatal("expected error or zero for negative duration")
	}
}

// --- ResolveConfigPath (env var) ---

func TestResolveConfigPath_FromEnv(t *testing.T) {
	os.Setenv("CONFIG", "/env/config.json")
	defer os.Unsetenv("CONFIG")

	path := ResolveConfigPath()
	if path != "/env/config.json" {
		t.Errorf("ResolveConfigPath() = %q, want %q", path, "/env/config.json")
	}
}

func TestResolveConfigPath_FromArgsShort(t *testing.T) {
	// Simulate -c flag.
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"app", "-c", "/args/config.json"}
	path := ResolveConfigPath()
	if path != "/args/config.json" {
		t.Errorf("ResolveConfigPath() = %q, want %q", path, "/args/config.json")
	}
}

func TestResolveConfigPath_FromArgsEquals(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"app", "-c=/args/config.json"}
	path := ResolveConfigPath()
	if path != "/args/config.json" {
		t.Errorf("ResolveConfigPath() = %q, want %q", path, "/args/config.json")
	}
}

func TestResolveConfigPath_FromArgsLong(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"app", "--config", "/args/config.json"}
	path := ResolveConfigPath()
	if path != "/args/config.json" {
		t.Errorf("ResolveConfigPath() = %q, want %q", path, "/args/config.json")
	}
}

func TestResolveConfigPath_FromArgsLongEquals(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"app", "--config=/args/config.json"}
	path := ResolveConfigPath()
	if path != "/args/config.json" {
		t.Errorf("ResolveConfigPath() = %q, want %q", path, "/args/config.json")
	}
}

func TestResolveConfigPath_NotSet(t *testing.T) {
	origArgs := os.Args
	origEnv := os.Getenv("CONFIG")
	defer func() {
		os.Args = origArgs
		if origEnv != "" {
			os.Setenv("CONFIG", origEnv)
		} else {
			os.Unsetenv("CONFIG")
		}
	}()

	os.Unsetenv("CONFIG")
	os.Args = []string{"app", "-a", "localhost:8080"}
	path := ResolveConfigPath()
	if path != "" {
		t.Errorf("ResolveConfigPath() = %q, want empty", path)
	}
}

func TestResolveConfigPath_ArgsVsEnv(t *testing.T) {
	// Both env var and flag are set. The env var is returned first since
	// it is checked before scanning os.Args.
	origArgs := os.Args
	origEnv := os.Getenv("CONFIG")
	defer func() {
		os.Args = origArgs
		if origEnv != "" {
			os.Setenv("CONFIG", origEnv)
		} else {
			os.Unsetenv("CONFIG")
		}
	}()

	os.Setenv("CONFIG", "/env/config.json")
	os.Args = []string{"app", "-c", "/args/config.json"}

	path := ResolveConfigPath()
	// CONFIG env is checked before os.Args, so it wins.
	if path != "/env/config.json" {
		t.Errorf("ResolveConfigPath() = %q, want %q", path, "/env/config.json")
	}
}

// --- End-to-end config priority via integration helpers ---

func TestServerConfigFile_RoundTrip(t *testing.T) {
	// Write a server config, load it, and verify all fields.
	path := filepath.Join(t.TempDir(), "server.json")
	writeJSON(t, path, `{
		"address": "0.0.0.0:3000",
		"restore": false,
		"store_interval": "120s",
		"store_file": "/data/db.json",
		"database_dsn": "",
		"crypto_key": "/keys/private.pem",
		"key": ""
	}`)

	cfg, err := LoadServerConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Address != "0.0.0.0:3000" {
		t.Errorf("Address = %q", cfg.Address)
	}
	if cfg.Restore == nil || *cfg.Restore {
		t.Error("Restore expected false")
	}
	if cfg.StoreInterval != "120s" {
		t.Errorf("StoreInterval = %q", cfg.StoreInterval)
	}
	if cfg.StoreFile != "/data/db.json" {
		t.Errorf("StoreFile = %q", cfg.StoreFile)
	}
	if cfg.CryptoKey != "/keys/private.pem" {
		t.Errorf("CryptoKey = %q", cfg.CryptoKey)
	}
}

func TestAgentConfigFile_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.json")
	writeJSON(t, path, `{
		"address": "agent.example.com:8080",
		"report_interval": "15s",
		"poll_interval": "3s",
		"crypto_key": "/keys/public.pem"
	}`)

	cfg, err := LoadAgentConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Address != "agent.example.com:8080" {
		t.Errorf("Address = %q", cfg.Address)
	}
	if cfg.ReportInterval != "15s" {
		t.Errorf("ReportInterval = %q", cfg.ReportInterval)
	}
	if cfg.PollInterval != "3s" {
		t.Errorf("PollInterval = %q", cfg.PollInterval)
	}
	if cfg.CryptoKey != "/keys/public.pem" {
		t.Errorf("CryptoKey = %q", cfg.CryptoKey)
	}
}

func TestParseDurationSec_StoreIntervalExamples(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"1s", 1},
		{"300s", 300},
		{"0s", 0},
	}

	for _, tt := range tests {
		got, err := ParseDurationSec(tt.input)
		if err != nil {
			t.Errorf("ParseDurationSec(%q): %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseDurationSec(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
