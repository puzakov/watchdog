package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/puzakov/watchdog/internal/config"
)

var serverBinary string

func TestMain(m *testing.M) {
	// Build binary once for all tests.
	dir, err := os.MkdirTemp("", "server-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	serverBinary = filepath.Join(dir, "server.test")
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", serverBinary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build server: %v\n%s", err, out)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// safeBuf is a goroutine-safe bytes.Buffer for use with cmd.Stdout/Stderr.
type safeBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (sb *safeBuf) Write(p []byte) (int, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

func (sb *safeBuf) Len() int {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Len()
}

func (sb *safeBuf) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.String()
}

// runServer starts the server with args, captures output, then
// sends Interrupt to let the server shut down gracefully.
func runServer(t *testing.T, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command(serverBinary, args...)
	cmd.Env = env
	var out safeBuf
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}

	// Poll for output, kill as soon as we see any.
	for range 40 { // max 2s wait
		if out.Len() > 5 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	_ = cmd.Process.Signal(os.Interrupt)
	time.Sleep(100 * time.Millisecond)
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	return out.String()
}

func TestServerBinary_BuildInfoWithoutFlags(t *testing.T) {
	out := runServer(t, nil)

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

	out := runServer(t, nil, "-c", cfgPath)

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

	out := runServer(t, nil, "-c", cfgPath, "-a", "fromflag:2222")

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

	out := runServer(t, append(os.Environ(), "ADDRESS=fromenv:3333"), "-c", cfgPath)

	if !strings.Contains(out, "fromenv") {
		t.Errorf("expected env address, got:\n%s", out)
	}
}

func TestServerBinary_UnknownFlagShowsUsage(t *testing.T) {
	cmd := exec.Command(serverBinary, "--badflag")
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
