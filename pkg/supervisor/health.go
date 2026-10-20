package supervisor

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/pkg/launch"
)

const healthTimeout = 2 * time.Second

var healthClient = &http.Client{Timeout: healthTimeout}

// healthURLForApp resolves an HTTP health probe for apps not managed via local pidfiles (e.g. Docker).
func healthURLForApp(appName, appDir string) string {
	key := "COGS_APP_HEALTH_" + strings.ToUpper(strings.ReplaceAll(appName, "-", "_"))
	if u := strings.TrimSpace(os.Getenv(key)); u != "" {
		return u
	}
	m, err := launch.Load(appDir)
	if err != nil || m.Port <= 0 {
		return ""
	}
	host := m.BindHost
	if host == "" {
		host = "127.0.0.1"
	}
	return "http://" + host + ":" + strconv.Itoa(m.Port) + "/health"
}

func probeHealth(url string) bool {
	if url == "" {
		return false
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := healthClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
