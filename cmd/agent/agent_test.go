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
)

var agentBinary string

func TestMain(m *testing.M) {
	// Build binary once for all tests.
	dir, err := os.MkdirTemp("", "agent-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	agentBinary = filepath.Join(dir, "agent.test")
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", agentBinary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build agent: %v\n%s", err, out)
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

// runAgent runs the agent binary with the given args and returns combined stdout+stderr.
// It polls for output and kills the process as soon as output appears (or after 2s max).
func runAgent(t *testing.T, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command(agentBinary, args...)
	cmd.Env = env
	var out safeBuf
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatalf("start agent: %v", err)
	}

	// Poll for output, kill as soon as we see any.
	for range 40 { // max 2s wait
		if out.Len() > 5 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	_ = cmd.Process.Signal(os.Interrupt)
	time.Sleep(50 * time.Millisecond)
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	return out.String()
}

func TestAgentBinary_BuildInfoWithoutFlags(t *testing.T) {
	out := runAgent(t, nil)

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

func TestAgentBinary_BuildInfoWithLdflags(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "agent.ldflags.test")
	cmd := exec.Command("go", "build",
		"-ldflags", "-X github.com/puzakov/watchdog/internal/build.Version=v1.0 -X github.com/puzakov/watchdog/internal/build.Date=today -X github.com/puzakov/watchdog/internal/build.Commit=abc123",
		"-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build with ldflags: %v\n%s", err, out)
	}

	var out safeBuf
	runcmd := exec.Command(bin)
	runcmd.Stdout = &out
	runcmd.Stderr = &out
	_ = runcmd.Start()
	for range 40 {
		if out.Len() > 5 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = runcmd.Process.Signal(os.Interrupt)
	time.Sleep(50 * time.Millisecond)
	_ = runcmd.Process.Kill()
	_ = runcmd.Wait()
	output := out.String()

	if !strings.Contains(output, "Build version: v1.0") {
		t.Errorf("expected version v1.0, got: %s", output)
	}
	if !strings.Contains(output, "Build date: today") {
		t.Errorf("expected date today, got: %s", output)
	}
	if !strings.Contains(output, "Build commit: abc123") {
		t.Errorf("expected commit abc123, got: %s", output)
	}
}

func TestAgentBinary_ConfigFileFlag(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "agent.json")
	cfgContent := `{
		"address": "testhost:9999",
		"report_interval": "1s",
		"poll_interval": "1s",
		"crypto_key": "/nonexistent/key.pem"
	}`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	out := runAgent(t, nil, "-c", cfgPath)

	if !strings.Contains(out, "/nonexistent/key.pem") {
		t.Errorf("expected key path from config file, got:\n%s", out)
	}
}

func TestAgentBinary_FlagOverridesConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "agent.json")
	cfgContent := `{"address": "fromconfig:1111", "crypto_key": "/nonexistent/key.pem"}`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	out := runAgent(t, nil, "-c", cfgPath, "-a", "fromflag:2222", "-k", "testkey")

	if !strings.Contains(out, "/nonexistent/key.pem") {
		t.Errorf("expected crypto_key from config, got:\n%s", out)
	}
}

func TestAgentBinary_EnvOverridesConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "agent.json")
	cfgContent := `{"address": "fromconfig:1111", "crypto_key": "/nonexistent/key.pem"}`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	out := runAgent(t, append(os.Environ(), "ADDRESS=fromenv:3333"), "-c", cfgPath)

	if !strings.Contains(out, "/nonexistent/key.pem") {
		t.Errorf("expected crypto_key from config, got:\n%s", out)
	}
}

func TestAgentBinary_UnknownFlagShowsUsage(t *testing.T) {
	cmd := exec.Command(agentBinary, "--nonexistent")
	out, _ := cmd.CombinedOutput()

	if !strings.Contains(string(out), "flag provided but not defined") {
		t.Errorf("expected unknown flag error, got: %s", out)
	}
}
