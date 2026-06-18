package audit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/puzakov/watchdog/internal/logger"
)

// URLObserver sends audit events as HTTP POST requests to a configured URL.
// generate:reset
type URLObserver struct {
	url    string
	client *http.Client
}

// NewURLObserver creates a URLObserver that POSTs events to the given URL.
func NewURLObserver(url string) *URLObserver {
	return &URLObserver{
		url: url,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (o *URLObserver) Notify(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		logger.Log.Error("audit url: marshal failed", zap.Error(err))
		return
	}

	req, err := http.NewRequest(http.MethodPost, o.url, bytes.NewReader(data))
	if err != nil {
		logger.Log.Error("audit url: create request failed",
			zap.String("url", o.url), zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		logger.Log.Error("audit url: request failed",
			zap.String("url", o.url), zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		logger.Log.Error("audit url: bad response",
			zap.String("url", o.url), zap.Int("status", resp.StatusCode))
	}
}
