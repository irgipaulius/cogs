package cli

import (
	"strings"

	"github.com/irgipaulius/cogs/pkg/state"
)

// recordAppBackgroundJobErrors sets last_error when app.log since start shows a failing background worker.
func recordAppBackgroundJobErrors(st *state.Store, appID, projectDir string) {
	if st == nil || appID == "" {
		return
	}
	b := appLogBytesSinceStart(projectDir)
	if len(b) == 0 {
		return
	}
	s := string(b)
	if !strings.Contains(s, "scraper:") {
		return
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "scraper:") {
			continue
		}
		if strings.Contains(line, "can't open") || strings.Contains(line, "exit status") || strings.Contains(line, "no such file") {
			_ = st.SetConverged(appID, false, "background job: "+line)
			return
		}
	}
}
