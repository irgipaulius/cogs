package cli

import (
	"sync"
	"time"
)

const appHealthGracePeriod = 90 * time.Second

var (
	appHealthStartMu sync.Mutex
	appHealthStart   = map[string]time.Time{}
)

func markAppHealthGrace(projectDir string) {
	appHealthStartMu.Lock()
	appHealthStart[projectDir] = time.Now()
	appHealthStartMu.Unlock()
}

func appInHealthGrace(projectDir string) bool {
	appHealthStartMu.Lock()
	start, ok := appHealthStart[projectDir]
	appHealthStartMu.Unlock()
	if !ok {
		return false
	}
	return time.Since(start) < appHealthGracePeriod
}
