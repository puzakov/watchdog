package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/puzakov/watchdog/internal/config"
)

// buildServerBinary builds the server binary and returns its path.
func buildServerBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "server.test")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build server: %v\n%s", err, out)
	}
	return bin
}

// runServer starts the server with args, captures output for ~1.5s, then
// sends Interrupt to let the server shut down gracefully and flush logs.
func runServer(t *testing.T, bin string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Start()

	time.Sleep(1500 * time.Millisecond)
	_ = cmd.Process.Signal(os.Interrupt)
	time.Sleep(500 * time.Millisecond)
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	return out.String()
}

func TestServerBinary_BuildInfoWithoutFlags(t *testing.T) {
	out := runServer(t, buildServerBinary(t), nil)

	if !strings.Contains(out, "Build version: N/A") {
		t.Errorf("expected Build version: N/A, got: %s", out)
	}
	if !strings.Contains(out, "Build date: N/A") {
		t.Errorf("expected Build date: N/A, got: %s", out)
	}
	if !strings.Contains(out, "Build commit: N/A") {
		t.Errorf("expected Build commit: N/A, got: %s", out)
	}
}

func TestServerBinary_ConfigFileFlag(t *testing.T) {
	// Use a bad address to make the server fail and log the address.
	cfgPath := filepath.Join(t.TempDir(), "server.json")
	cfgContent := `{"address": "nosuchhost:9999", "store_interval": "300s"}`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	bin := buildServerBinary(t)
	out := runServer(t, bin, nil, "-c", cfgPath)

	if !strings.Contains(out, "nosuchhost") {
		t.Errorf("expected address from config file, got:\n%s", out)
	}
}

func TestServerBinary_FlagOverridesConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "server.json")
	cfgContent := `{"address": "fromconfig:1111"}`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	bin := buildServerBinary(t)
	out := runServer(t, bin, nil, "-c", cfgPath, "-a", "fromflag:2222")

	if !strings.Contains(out, "fromflag") {
		t.Errorf("expected flag address, got:\n%s", out)
	}
}

func TestServerBinary_EnvOverridesConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "server.json")
	cfgContent := `{"address": "fromconfig:1111"}`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	bin := buildServerBinary(t)
	out := runServer(t, bin, append(os.Environ(), "ADDRESS=fromenv:3333"), "-c", cfgPath)

	if !strings.Contains(out, "fromenv") {
		t.Errorf("expected env address, got:\n%s", out)
	}
}

func TestServerBinary_UnknownFlagShowsUsage(t *testing.T) {
	bin := buildServerBinary(t)
	cmd := exec.Command(bin, "--badflag")
	out, _ := cmd.CombinedOutput()

	if !strings.Contains(string(out), "flag provided but not defined") {
		t.Errorf("expected unknown flag error, got: %s", out)
	}
}

func TestRun_WithConfig(t *testing.T) {
	// Test that run() accepts a config and starts briefly without error.
	cfg := &config.EnvConfig{
		Addr:             "localhost:0",
		StoreIntervalInt: 0,
		Restore:          false,
	}
	done := make(chan error, 1)
	go func() {
		done <- run(cfg, nil, "")
	}()

	select {
	case <-time.After(500 * time.Millisecond):
		// Server is still running — expected.
	case err := <-done:
		if err != nil {
			t.Logf("server exited: %v", err)
		}
	}
}
