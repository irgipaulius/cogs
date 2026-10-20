// Package config loads and validates COGS_* environment variables.
package config

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// App holds common CoGS application settings from the environment.
type App struct {
	ListenAddr        string
	AllowPublicBind   bool
	DBPath            string
	SessionSecret     string
	TurnstileSecret   string
	TurnstileSiteKey  string
	AccessTeamDomain  string
	AccessAudience    string
	PrivateToken      string
	APIKey            string
	PublicPageOrigin  string
	PublicAPIURL      string
	MaxUploadBytes    int64
	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string
	D1AccountID       string
	D1DatabaseID      string
	D1APIToken        string
	JobsMode          string
	ObsMode           string
	ObsLogPath        string
	LinkPerf          bool
	LinkPerfMaxBytes  int64
}

// Load reads env from os.Environ, optionally overlaying a dotenv file.
func Load(envFile string) (App, error) {
	vars := map[string]string{}
	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if ok {
			vars[k] = v
		}
	}
	if envFile != "" {
		if err := overlayFile(envFile, vars); err != nil {
			return App{}, err
		}
	}
	return parse(vars)
}

func overlayFile(path string, vars map[string]string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		vars[k] = v
	}
	return sc.Err()
}

func parse(vars map[string]string) (App, error) {
	get := func(key string) string { return vars[key] }

	c := App{
		ListenAddr:        firstNonEmpty(get("COGS_LISTEN"), "127.0.0.1:18787"),
		AllowPublicBind:   truthy(get("COGS_ALLOW_PUBLIC_BIND")),
		DBPath:            firstNonEmpty(get("COGS_DB_PATH"), "data.db"),
		SessionSecret:     get("COGS_SESSION_SECRET"),
		TurnstileSecret:   get("COGS_TURNSTILE_SECRET"),
		TurnstileSiteKey:  get("COGS_TURNSTILE_SITE_KEY"),
		AccessTeamDomain:  get("COGS_ACCESS_TEAM_DOMAIN"),
		AccessAudience:    get("COGS_ACCESS_AUDIENCE"),
		PrivateToken:      get("COGS_PRIVATE_TOKEN"),
		APIKey:            get("COGS_API_KEY"),
		PublicPageOrigin:  get("COGS_PUBLIC_PAGE_ORIGIN"),
		PublicAPIURL:      get("COGS_PUBLIC_API_URL"),
		MaxUploadBytes:    int64Env(get, "COGS_MAX_UPLOAD_BYTES", 12<<20),
		R2AccountID:       get("COGS_R2_ACCOUNT_ID"),
		R2AccessKeyID:     get("COGS_R2_ACCESS_KEY_ID"),
		R2SecretAccessKey: get("COGS_R2_SECRET_ACCESS_KEY"),
		R2Bucket:          get("COGS_R2_BUCKET"),
		D1AccountID:       get("COGS_D1_ACCOUNT_ID"),
		D1DatabaseID:      get("COGS_D1_DATABASE_ID"),
		D1APIToken:        get("COGS_D1_API_TOKEN"),
		JobsMode:          firstNonEmpty(get("COGS_JOBS_MODE"), "os"),
		ObsMode:           firstNonEmpty(get("COGS_OBS"), "file"),
		ObsLogPath:        get("COGS_OBS_LOG"),
		LinkPerf:          !truthy(get("COGS_LINKPERF_OFF")),
		LinkPerfMaxBytes:  int64Env(get, "COGS_LINKPERF_MAX_BYTES", 512<<10),
	}

	var missing []string
	if c.SessionSecret == "" {
		missing = append(missing, "COGS_SESSION_SECRET")
	}
	if len(missing) > 0 {
		return App{}, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}

	host, _, err := net.SplitHostPort(c.ListenAddr)
	if err != nil {
		return App{}, fmt.Errorf("COGS_LISTEN: %w", err)
	}
	if !c.AllowPublicBind && host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return App{}, fmt.Errorf("refusing public bind %q; set COGS_ALLOW_PUBLIC_BIND=1 to override", c.ListenAddr)
	}
	return c, nil
}

// LoadRelaxed is like Load but only requires DB path for CLI tools.
func LoadRelaxed(envFile string) (App, error) {
	vars := map[string]string{}
	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if ok {
			vars[k] = v
		}
	}
	if envFile != "" {
		if err := overlayFile(envFile, vars); err != nil {
			return App{}, err
		}
	}
	get := func(key string) string { return vars[key] }
	return App{
		ListenAddr:        firstNonEmpty(get("COGS_LISTEN"), "127.0.0.1:18787"),
		AllowPublicBind:   truthy(get("COGS_ALLOW_PUBLIC_BIND")),
		DBPath:            firstNonEmpty(get("COGS_DB_PATH"), "data.db"),
		SessionSecret:     get("COGS_SESSION_SECRET"),
		R2AccountID:       get("COGS_R2_ACCOUNT_ID"),
		R2AccessKeyID:     get("COGS_R2_ACCESS_KEY_ID"),
		R2SecretAccessKey: get("COGS_R2_SECRET_ACCESS_KEY"),
		R2Bucket:          get("COGS_R2_BUCKET"),
		D1AccountID:       get("COGS_D1_ACCOUNT_ID"),
		D1DatabaseID:      get("COGS_D1_DATABASE_ID"),
		D1APIToken:        get("COGS_D1_API_TOKEN"),
	}, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func truthy(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// IntEnv parses an integer env value with default.
func int64Env(get func(string) string, key string, def int64) int64 {
	v := get(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

func IntEnv(vars map[string]string, key string, def int) int {
	v := vars[key]
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
