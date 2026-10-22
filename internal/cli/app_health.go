package cli

import (
	"net/http"
	"time"

	"github.com/irgipaulius/cogs/pkg/appcfg"
)

func appHealthURL(projectDir string) string {
	app, err := appcfg.Load(projectDir)
	if err != nil || app.Port <= 0 {
		return ""
	}
	host := app.BindHost
	if host == "" {
		host = "127.0.0.1"
	}
	return "http://" + listenAddr(host, app.Port) + "/health"
}

func probeAppHealth(url string) bool {
	if url == "" {
		return false
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
