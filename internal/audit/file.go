package audit

import (
	"encoding/json"
	"os"
	"sync"
)

// FileObserver writes audit events as newline-delimited JSON to a file.
type FileObserver struct {
	path string
	mu   sync.Mutex
}

// NewFileObserver creates a FileObserver that appends events to the given path.
func NewFileObserver(path string) *FileObserver {
	return &FileObserver{path: path}
}

func (o *FileObserver) Notify(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	data = append(data, '\n')

	o.mu.Lock()
	defer o.mu.Unlock()

	f, err := os.OpenFile(o.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	_, _ = f.Write(data)
}
