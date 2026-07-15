package build

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn and returns everything written to stdout.
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestPrintInfo_AllValuesSet(t *testing.T) {
	Version = "1.2.3"
	Date = "2026-07-08"
	Commit = "abc123"

	out := captureStdout(PrintInfo)

	if !strings.Contains(out, "Build version: 1.2.3") {
		t.Errorf("output missing version, got: %s", out)
	}
	if !strings.Contains(out, "Build date: 2026-07-08") {
		t.Errorf("output missing date, got: %s", out)
	}
	if !strings.Contains(out, "Build commit: abc123") {
		t.Errorf("output missing commit, got: %s", out)
	}
}

func TestPrintInfo_AllEmpty(t *testing.T) {
	Version = ""
	Date = ""
	Commit = ""

	out := captureStdout(PrintInfo)

	if !strings.Contains(out, "Build version: N/A") {
		t.Errorf("expected N/A for version, got: %s", out)
	}
	if !strings.Contains(out, "Build date: N/A") {
		t.Errorf("expected N/A for date, got: %s", out)
	}
	if !strings.Contains(out, "Build commit: N/A") {
		t.Errorf("expected N/A for commit, got: %s", out)
	}
}

func TestPrintInfo_PartialValues(t *testing.T) {
	Version = "0.0.1"
	Date = ""
	Commit = "def456"

	out := captureStdout(PrintInfo)

	if !strings.Contains(out, "Build version: 0.0.1") {
		t.Errorf("output missing version, got: %s", out)
	}
	if !strings.Contains(out, "Build date: N/A") {
		t.Errorf("expected N/A for date, got: %s", out)
	}
	if !strings.Contains(out, "Build commit: def456") {
		t.Errorf("output missing commit, got: %s", out)
	}
}

func TestPrintInfo_OutputFormat(t *testing.T) {
	Version = "v2"
	Date = "today"
	Commit = "abcd"

	out := captureStdout(PrintInfo)

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), out)
	}

	expected := []string{
		"Build version: v2",
		"Build date: today",
		"Build commit: abcd",
	}
	for i, exp := range expected {
		if lines[i] != exp {
			t.Errorf("line %d: got %q, want %q", i, lines[i], exp)
		}
	}
}

func TestPrintInfo_Newlines(t *testing.T) {
	Version = "x"
	Date = "y"
	Commit = "z"

	out := captureStdout(PrintInfo)

	// Each line should end with \n (including the last).
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("expected trailing newline, got: %q", out)
	}

	// Count newlines.
	n := strings.Count(out, "\n")
	if n != 3 {
		t.Errorf("expected 3 newlines, got %d", n)
	}
}
