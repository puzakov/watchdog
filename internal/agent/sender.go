package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	models "github.com/puzakov/watchdog/internal/model"
)

type SenderConfig struct {
	ServerAddress string
	Client        *http.Client
	Logger        *log.Logger
}

type Sender struct {
	cfg SenderConfig
}

func NewSender(cfg SenderConfig) *Sender {
	if cfg.ServerAddress == "" {
		cfg.ServerAddress = "http://localhost:8080"
	}
	cfg.ServerAddress = strings.TrimRight(cfg.ServerAddress, "/")
	if cfg.Client == nil {
		cfg.Client = http.DefaultClient
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	return &Sender{cfg: cfg}
}

func (s *Sender) SendGauge(name string, value float64) error {
	return s.post(&models.Metrics{ID: name, MType: models.Gauge, Value: &value})
}

func (s *Sender) SendCounter(name string, delta int64) error {
	return s.post(&models.Metrics{ID: name, MType: models.Counter, Delta: &delta})
}

func (s *Sender) post(args *models.Metrics) error {
	u := fmt.Sprintf("%s/update", s.cfg.ServerAddress)

	body, err := json.Marshal(&args)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	if _, err := gzw.Write(body); err != nil {
		_ = gzw.Close()
		return err
	}
	if err := gzw.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, u, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := s.cfg.Client.Do(req)
	if err != nil {
		return err
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	err = resp.Body.Close()
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}
	return nil
}
