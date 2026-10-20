package ingress

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const DefaultFilename = "cogs.ingress.json"

// Config controls tunnel-first vs direct access per route.
type Config struct {
	// DefaultPolicy is "tunnel_only" (zero value behavior when file loaded).
	DefaultPolicy string `json:"default_policy"`
	// TrustLoopback allows 127.0.0.1 / ::1 without Cloudflare headers (local dev, admin).
	TrustLoopback bool `json:"trust_loopback"`
	// DirectSecret if set requires X-Cogs-Direct-Key header on direct (non-CF) requests.
	DirectSecret string `json:"direct_secret_env"`
	Routes       []Route `json:"routes"`
}

// Route declares one HTTP pattern and whether the public internet may hit it without tunnel headers.
type Route struct {
	// Pattern is "METHOD /path" or "/path" (any method). Path may use * suffix wildcard.
	Pattern     string `json:"pattern"`
	AllowDirect bool   `json:"allow_direct"`
}

// Load reads cogs.ingress.json.
func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, err
	}
	if c.DefaultPolicy == "" {
		c.DefaultPolicy = "tunnel_only"
	}
	// Default true so local curl / cloudflared→127.0.0.1 work without CF headers on loopback TCP.
	if !c.trustLoopbackSet(b) {
		c.TrustLoopback = true
	}
	return c, nil
}

func (c Config) trustLoopbackSet(raw []byte) bool {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	_, ok := m["trust_loopback"]
	return ok
}

// LoadOptional returns nil when no config file exists (ingress middleware off).
func LoadOptional(path string) (*Config, error) {
	if path == "" {
		return nil, nil
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	c, err := Load(path)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ResolvePath picks COGS_INGRESS_CONFIG or cogs.ingress.json beside the env file.
func ResolvePath(envFile string) string {
	if p := os.Getenv("COGS_INGRESS_CONFIG"); p != "" {
		return p
	}
	if envFile == "" {
		return DefaultFilename
	}
	return filepath.Join(filepath.Dir(envFile), DefaultFilename)
}
