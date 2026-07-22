package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// ServerConfigFile represents the JSON configuration file for the server.
// Field names use snake_case to match the documented JSON format.
type ServerConfigFile struct {
	Address       string `json:"address"`
	Restore       *bool  `json:"restore,omitempty"`
	StoreInterval string `json:"store_interval"` // duration string, e.g. "300s"
	StoreFile     string `json:"store_file"`
	DatabaseDSN   string `json:"database_dsn"`
	CryptoKey     string `json:"crypto_key"`
	Key           string `json:"key"`
	TrustedSubnet string `json:"trusted_subnet"`
}

// AgentConfigFile represents the JSON configuration file for the agent.
type AgentConfigFile struct {
	Address        string `json:"address"`
	ReportInterval string `json:"report_interval"` // duration string, e.g. "10s"
	PollInterval   string `json:"poll_interval"`   // duration string, e.g. "2s"
	CryptoKey      string `json:"crypto_key"`
	Key            string `json:"key"`
}

func loadConfigFile[T ServerConfigFile | AgentConfigFile](path string) (*T, error) {
	if path == "" {
		return nil, errors.New("config file path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	var cfg T
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	return &cfg, nil
}

// LoadServerConfigFile reads and parses a JSON server configuration file.
func LoadServerConfigFile(path string) (*ServerConfigFile, error) {
	return loadConfigFile[ServerConfigFile](path)
}

// LoadAgentConfigFile reads and parses a JSON agent configuration file.
func LoadAgentConfigFile(path string) (*AgentConfigFile, error) {
	return loadConfigFile[AgentConfigFile](path)
}

// ParseDurationSec parses a duration string (e.g. "300s", "1m", "1h") and
// returns the equivalent integer number of seconds. Returns 0 if the string is
// empty, or an error if parsing fails or the duration is negative.
func ParseDurationSec(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("negative duration %q", s)
	}
	return int(d.Seconds()), nil
}

// ResolveConfigPath returns the config file path from the CONFIG environment
// variable. This is called before flag.Parse, so command-line -c is handled
// by scanning os.Args manually.
func ResolveConfigPath() string {
	// Check CONFIG env var first.
	if p := os.Getenv("CONFIG"); p != "" {
		return p
	}
	// Scan os.Args for -c or -config.
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case arg == "-c" || arg == "--config":
			if i+1 < len(os.Args) {
				return os.Args[i+1]
			}
		case strings.HasPrefix(arg, "-c=") || strings.HasPrefix(arg, "--config="):
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				return parts[1]
			}
		}
	}
	return ""
}
