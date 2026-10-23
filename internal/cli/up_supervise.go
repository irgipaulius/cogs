package cli

import (
	"sync"
	"time"

	"github.com/irgipaulius/cogs/pkg/provision"
)

type upAppGate struct {
	mu        sync.Mutex
	failures  map[string]int
	nextTry   map[string]time.Time
	lastLog   map[string]string
}

var upGate upAppGate

func init() {
	upGate.failures = map[string]int{}
	upGate.nextTry = map[string]time.Time{}
	upGate.lastLog = map[string]string{}
}

func (g *upAppGate) allow(appName string, st provision.State) (ok bool, reason string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if st.Ready {
		delete(g.failures, appName)
		delete(g.nextTry, appName)
		delete(g.lastLog, appName)
		return true, ""
	}
	return false, st.Status
}

func (g *upAppGate) mayLog(appName string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if t, ok := g.nextTry[appName]; ok && time.Now().Before(t) {
		return false
	}
	g.nextTry[appName] = time.Now().Add(2 * time.Minute)
	return true
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
	if g.lastLog[appName] == msg {
		return false
	}
	g.lastLog[appName] = msg
	return true
}
