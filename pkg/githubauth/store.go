package githubauth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const fileName = "github.json"

// Credential is legacy workspace GitHub token (prefer host gh login).
type Credential struct {
	Token     string    `json:"token"`
	Login     string    `json:"login,omitempty"`
	Scopes    string    `json:"scopes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func path(wsRoot string) string {
	return filepath.Join(wsRoot, ".cogs", fileName)
}

// Load reads stored credentials. Missing file is not an error.
func Load(wsRoot string) (Credential, error) {
	b, err := os.ReadFile(path(wsRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return Credential{}, nil
		}
		return Credential{}, err
	}
	var c Credential
	if err := json.Unmarshal(b, &c); err != nil {
		return Credential{}, err
	}
	return c, nil
}

// Save writes credentials (0o600).
func Save(wsRoot string, c Credential) error {
	if err := os.MkdirAll(filepath.Join(wsRoot, ".cogs"), 0o750); err != nil {
		return err
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path(wsRoot), b, 0o600)
}

// Token returns a bearer token: host `gh auth token` first, then legacy .cogs/github.json.
func Token(wsRoot string) string {
	if t, err := tokenFromGHCLI(); err == nil && t != "" {
		return t
	}
	c, err := Load(wsRoot)
	if err != nil {
		return ""
	}
	return c.Token
}
