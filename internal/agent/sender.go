package agent

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
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
	val := strconv.FormatFloat(value, 'g', -1, 64)
	return s.post(models.Gauge, name, val)
}

func (s *Sender) SendCounter(name string, delta int64) error {
	val := strconv.FormatInt(delta, 10)
	return s.post(models.Counter, name, val)
}

func (s *Sender) post(mType, name, value string) error {
	escapedName := url.PathEscape(name)
	escapedValue := url.PathEscape(value)
	u := fmt.Sprintf("%s/update/%s/%s/%s", s.cfg.ServerAddress, mType, escapedName, escapedValue)

	req, err := http.NewRequest(http.MethodPost, u, http.NoBody)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")

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
