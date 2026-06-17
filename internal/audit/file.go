package audit

import (
	"encoding/json"
	"os"
	"sync"

	"go.uber.org/zap"

	"github.com/puzakov/watchdog/internal/logger"
)

// FileObserver writes audit events as newline-delimited JSON to a file.
// The file is opened once and kept open for the lifetime of the observer.
type FileObserver struct {
	file *os.File
	mu   sync.Mutex
}

// NewFileObserver creates a FileObserver that appends events to the given path.
// Returns an observer that silently discards events if the file cannot be opened.
func NewFileObserver(path string) *FileObserver {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		logger.Log.Error("audit file: open failed", zap.String("path", path), zap.Error(err))
		return &FileObserver{}
	}
	return &FileObserver{file: f}
}

func (o *FileObserver) Notify(event Event) {
	if o.file == nil {
		return
	}
	data, err := json.Marshal(event)
	if err != nil {
		logger.Log.Error("audit file: marshal failed", zap.Error(err))
		return
	}
	data = append(data, '\n')

	o.mu.Lock()
	defer o.mu.Unlock()

	if _, err := o.file.Write(data); err != nil {
		logger.Log.Error("audit file: write failed", zap.Error(err))
	}
}

// Close flushes and closes the underlying file.
// Safe to call on a nil or already-closed observer.
func (o *FileObserver) Close() error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.file == nil {
		return nil
	}
	return o.file.Close()
}
