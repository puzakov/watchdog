package agent

import (
	"bytes"
	"compress/gzip"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/puzakov/watchdog/internal/crypto"
	models "github.com/puzakov/watchdog/internal/model"
	"github.com/puzakov/watchdog/internal/sign"
)

// ErrEndpointUnsupported is returned when the server does not support the
// target endpoint (404 or 405). Triggers a fallback to legacy single-metric sends.
var ErrEndpointUnsupported = errors.New("endpoint unsupported")

var httpRetryDelays = []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

var gzipBufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

var gzipWriterPool = sync.Pool{
	New: func() any {
		return gzip.NewWriter(io.Discard)
	},
}

// SenderConfig configures the HTTP metrics sender.
//
// generate:reset
type SenderConfig struct {
	// ServerAddress is the base URL of the metrics server.
	ServerAddress string
	// Client is the HTTP client used for requests. If nil, http.DefaultClient is used.
	Client *http.Client
	// Key for SHA256 request signing (empty = no signing).
	Key string
	// CryptoKey is the RSA public key used to encrypt request bodies.
	// When set, the gzip-compressed payload is RSA-OAEP encrypted before sending.
	// If nil, no encryption is applied.
	CryptoKey *rsa.PublicKey
	// Logger for diagnostics. If nil, log.Default() is used.
	Logger *log.Logger
}

// Sender sends metrics to the server over HTTP with gzip compression and optional SHA256 signing.
//
// generate:reset
type Sender struct {
	cfg SenderConfig
}

// NewSender creates a Sender with the given configuration.
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

// SendGauge sends a single gauge metric to the /update endpoint.
func (s *Sender) SendGauge(name string, value float64) error {
	return s.postJSON("/update", &models.Metrics{ID: name, MType: models.Gauge, Value: &value})
}

// SendCounter sends a single counter metric to the /update endpoint.
func (s *Sender) SendCounter(name string, delta int64) error {
	return s.postJSON("/update", &models.Metrics{ID: name, MType: models.Counter, Delta: &delta})
}

// SendBatch sends a batch of metrics to the /updates endpoint.
// If the endpoint is unsupported, returns ErrEndpointUnsupported
// (the caller should fall back to single-metric sends).
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

	wireBody, err := gzipBytes(body)
	if err != nil {
		return err
	}

	// When RSA encryption is configured, encrypt the gzip-compressed payload.
	// The gzip layer is inside the encryption envelope; the wire Content-Encoding
	// header is removed because the body on the wire is no longer plain gzip.
	if s.cfg.CryptoKey != nil {
		wireBody, err = crypto.EncryptOAEP(wireBody, s.cfg.CryptoKey)
		if err != nil {
			return err
		}
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(wireBody))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		if s.cfg.CryptoKey == nil {
			req.Header.Set("Content-Encoding", "gzip")
		}
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
	buf := gzipBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer gzipBufPool.Put(buf)

	gzw := gzipWriterPool.Get().(*gzip.Writer)
	gzw.Reset(buf)
	if _, err := gzw.Write(b); err != nil {
		gzipWriterPool.Put(gzw)
		return nil, err
	}
	if err := gzw.Close(); err != nil {
		gzipWriterPool.Put(gzw)
		return nil, err
	}
	gzipWriterPool.Put(gzw)

	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out, nil
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
