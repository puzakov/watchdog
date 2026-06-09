package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	models "github.com/puzakov/watchdog/internal/model"
	"github.com/puzakov/watchdog/internal/sign"
)

var ErrEndpointUnsupported = errors.New("endpoint unsupported")

var httpRetryDelays = []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

type SenderConfig struct {
	ServerAddress string
	Client        *http.Client
	Key           string
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
	return s.postJSON("/update", &models.Metrics{ID: name, MType: models.Gauge, Value: &value})
}

func (s *Sender) SendCounter(name string, delta int64) error {
	return s.postJSON("/update", &models.Metrics{ID: name, MType: models.Counter, Delta: &delta})
}

func (s *Sender) SendBatch(metrics []models.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}
	return s.postJSON("/updates", metrics)
}

func (s *Sender) postJSON(path string, payload any) error {
	u := fmt.Sprintf("%s%s", s.cfg.ServerAddress, path)

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	gzipped, err := gzipBytes(body)
	if err != nil {
		return err
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(gzipped))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		if s.cfg.Key != "" {
			req.Header.Set(sign.HeaderHashSHA256, sign.SumSHA256(body, s.cfg.Key))
		}

		resp, err := s.cfg.Client.Do(req)
		if err != nil {
			if isRetriableConnectError(err) && attempt < len(httpRetryDelays) {
				time.Sleep(httpRetryDelays[attempt])
				continue
			}
			return err
		}

		_, _ = io.Copy(io.Discard, resp.Body)
		closeErr := resp.Body.Close()
		if closeErr != nil {
			return closeErr
		}

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
			return fmt.Errorf("%w: %s", ErrEndpointUnsupported, resp.Status)
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status: %s", resp.Status)
		}
		return nil
	}
}

func gzipBytes(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	if _, err := gzw.Write(b); err != nil {
		_ = gzw.Close()
		return nil, err
	}
	if err := gzw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func isRetriableConnectError(err error) bool {
	// We're conservative here: retry only errors that likely happened before
	// the HTTP request was sent (connection could not be established).
	var uerr *url.Error
	if errors.As(err, &uerr) {
		err = uerr.Unwrap()
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Op == "dial" {
			return true
		}
	}

	// Common "can't connect" syscall errors.
	return errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ETIMEDOUT) ||
		errors.Is(err, syscall.ENETUNREACH) ||
		errors.Is(err, syscall.EHOSTUNREACH)
}
