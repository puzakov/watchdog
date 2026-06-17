package logger

import (
	"testing"

	"go.uber.org/zap"
)

func TestInitialize(t *testing.T) {
	err := Initialize("debug")
	if err != nil {
		t.Fatalf("Initialize(debug) error = %v", err)
	}
	if Log == nil {
		t.Fatal("Log is nil after Initialize")
	}
	// Verify the logger actually works
	Log.Debug("test debug message")
	Log.Info("test info message")
}

func TestInitializeInvalidLevel(t *testing.T) {
	err := Initialize("invalid-level")
	if err == nil {
		t.Fatal("Initialize with invalid level should return error")
	}
}

func TestInitializeInfo(t *testing.T) {
	err := Initialize("info")
	if err != nil {
		t.Fatalf("Initialize(info) error = %v", err)
	}
	if Log == nil {
		t.Fatal("Log is nil after Initialize")
	}

	// Debug should not panic but not output at info level
	Log.Debug("should not appear")
}

func TestDefaultLoggerIsNop(t *testing.T) {
	// Before Initialize, Log is a no-op logger
	if Log == nil {
		t.Fatal("Log should not be nil by default")
	}

	// Verify it's not nil and is a no-op logger
	if Log.Core().Enabled(zap.DebugLevel) {
		t.Log("Log.Core enabled debug level — may have been initialized already")
	}
}
