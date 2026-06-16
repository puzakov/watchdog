package audit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

// URLObserver sends audit events as HTTP POST requests to a configured URL.
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
		return
	}

	req, err := http.NewRequest(http.MethodPost, o.url, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}
