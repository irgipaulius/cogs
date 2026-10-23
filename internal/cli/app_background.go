package cli

import (
	"regexp"
	"strings"

	"github.com/irgipaulius/cogs/pkg/state"
)

var bgJobFailRE = regexp.MustCompile(`(?i)(exit status [1-9][0-9]*|can't open|no such file|failed|error)`)

// recordAppBackgroundJobErrors sets last_error when app.log since start shows a failing background worker.
func recordAppBackgroundJobErrors(st *state.Store, appID, projectDir string) {
	if st == nil || appID == "" {
		return
	}
	b := appLogBytesSinceStart(projectDir)
	if len(b) == 0 {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !backgroundJobFailedLine(line) {
			continue
		}
		_ = st.SetConverged(appID, false, "background job: "+line)
		return
	}
}

func backgroundJobFailedLine(line string) bool {
	if line == "" {
		return false
	}
	// CoGS apps log background workers with a "scraper:" prefix today.
	if !strings.Contains(line, "scraper:") {
		return false
	}
	return bgJobFailRE.MatchString(line)
}
