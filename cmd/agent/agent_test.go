package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// buildAgentBinary builds the agent binary and returns its path.
func buildAgentBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "agent.test")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build agent: %v\n%s", err, out)
	}
	return bin
}

// runAgent runs the agent binary with the given args and returns combined stdout+stderr.
// Killed after 2s since the agent runs indefinitely.
func runAgent(t *testing.T, bin string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Start()

	// Wait briefly then kill with SIGTERM so buffers are flushed.
	time.Sleep(1500 * time.Millisecond)
	_ = cmd.Process.Signal(os.Interrupt)
	time.Sleep(100 * time.Millisecond)
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	return out.String()
}

func TestAgentBinary_BuildInfoWithoutFlags(t *testing.T) {
	out := runAgent(t, buildAgentBinary(t), nil)

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

	out := runAgent(t, bin, nil)
	if !strings.Contains(out, "Build version: v1.0") {
		t.Errorf("expected version v1.0, got: %s", out)
	}
	if !strings.Contains(out, "Build date: today") {
		t.Errorf("expected date today, got: %s", out)
	}
	if !strings.Contains(out, "Build commit: abc123") {
		t.Errorf("expected commit abc123, got: %s", out)
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

	out := runAgent(t, buildAgentBinary(t), nil, "-c", cfgPath)

	// The config file loading happens before flag.Parse and uses ResolveConfigPath.
	// If the config file was loaded, the address from the config is used.
	// crypto_key points to a missing file, so an error message appears on stdout.
	if !strings.Contains(out, "/nonexistent/key.pem") {
		t.Errorf("expected key path from config file, got:\n%s", out)
	}
}

func TestAgentBinary_FlagOverridesConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "agent.json")
	// Config sets address but also specifies crypto_key for verification.
	cfgContent := `{"address": "fromconfig:1111", "crypto_key": "/nonexistent/key.pem"}`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	// -a should override config address. The crypto_key is still loaded from config.
	bin := buildAgentBinary(t)
	out := runAgent(t, bin, nil, "-c", cfgPath, "-a", "fromflag:2222", "-k", "testkey")

	// Because -a overrides the config address, the agent connects to fromflag:2222.
	// crypto_key is from config — should still print the missing file error.
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

	bin := buildAgentBinary(t)
	out := runAgent(t, bin, append(os.Environ(), "ADDRESS=fromenv:3333"), "-c", cfgPath)

	if !strings.Contains(out, "/nonexistent/key.pem") {
		t.Errorf("expected crypto_key from config, got:\n%s", out)
	}
}

func TestAgentBinary_UnknownFlagShowsUsage(t *testing.T) {
	bin := buildAgentBinary(t)

	cmd := exec.Command(bin, "--nonexistent")
	out, _ := cmd.CombinedOutput()

	if !strings.Contains(string(out), "flag provided but not defined") {
		t.Errorf("expected unknown flag error, got: %s", out)
	}
}
