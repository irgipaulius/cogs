package cli

import (
	"sync"
	"time"
)

type upAppGate struct {
	mu       sync.Mutex
	failures map[string]int
	nextTry  map[string]time.Time
	lastLog  map[string]string
}

var upGate upAppGate

func init() {
	upGate.failures = map[string]int{}
	upGate.nextTry = map[string]time.Time{}
	upGate.lastLog = map[string]string{}
}

func (g *upAppGate) recordFailure(appName, msg string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.failures[appName]++
	n := g.failures[appName]
	delay := time.Duration(n) * 15 * time.Second
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	g.nextTry[appName] = time.Now().Add(delay)
	if g.lastLog[appName] != msg {
		g.lastLog[appName] = msg
	}
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
