package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type upAppGate struct {
	mu          sync.Mutex
	failures    map[string]int
	nextTry     map[string]time.Time
	lastLog     map[string]string
	authMissing map[string]string // key -> last message
}

var upGate upAppGate

func init() {
	upGate.failures = map[string]int{}
	upGate.nextTry = map[string]time.Time{}
	upGate.lastLog = map[string]string{}
	upGate.authMissing = map[string]string{}
}

const (
	tunnelBackoffBase = 5 * time.Second
	tunnelBackoffCap  = 5 * time.Minute
	authRetryInterval = 5 * time.Minute
)

func (g *upAppGate) recordFailure(appName, msg string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.failures[appName]++
	n := g.failures[appName]
	delay := time.Duration(n) * 15 * time.Second
	if delay > tunnelBackoffCap {
		delay = tunnelBackoffCap
	}
	g.nextTry[appName] = time.Now().Add(delay)
	if g.lastLog[appName] != msg {
		g.lastLog[appName] = msg
	}
}

func (g *upAppGate) recordTunnelFailure(key, msg string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.failures[key]++
	n := g.failures[key]
	delay := tunnelBackoffBase
	for i := 1; i < n; i++ {
		delay *= 2
		if delay >= tunnelBackoffCap {
			delay = tunnelBackoffCap
			break
		}
	}
	g.nextTry[key] = time.Now().Add(delay)
	if g.lastLog[key] != msg {
		g.lastLog[key] = msg
	}
}

func (g *upAppGate) mayRetry(appName string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if msg, blocked := g.authMissing[appName]; blocked && msg != "" {
		if t, ok := g.nextTry[appName]; ok && time.Now().Before(t) {
			return false
		}
	}
	if t, ok := g.nextTry[appName]; ok && time.Now().Before(t) {
		return false
	}
	return true
}

func (g *upAppGate) recordAuthFailure(key, msg string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.authMissing[key] = msg
	g.nextTry[key] = time.Now().Add(authRetryInterval)
	g.lastLog[key] = msg
}

func (g *upAppGate) clearAuthFailure(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.authMissing, key)
	delete(g.failures, key)
	delete(g.nextTry, key)
}

func (g *upAppGate) shouldLog(appName, msg string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if t, ok := g.nextTry[appName]; ok && time.Now().Before(t) {
		return false
	}
	if g.lastLog[appName] == msg {
		return false
	}
	g.lastLog[appName] = msg
	return true
}

func (g *upAppGate) writeAlerts(root string) {
	g.mu.Lock()
	alerts := map[string]any{}
	if len(g.authMissing) > 0 {
		m := map[string]string{}
		for k, v := range g.authMissing {
			m[k] = v
		}
		alerts["cloudflare_auth_missing"] = m
	}
	g.mu.Unlock()
	if root == "" {
		return
	}
	dir := filepath.Join(root, ".cogs", "run")
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "supervisor-alerts.json")
	if len(alerts) == 0 {
		_ = os.Remove(path)
		return
	}
	b, _ := json.Marshal(alerts)
	_ = os.WriteFile(path, b, 0o644)
}
