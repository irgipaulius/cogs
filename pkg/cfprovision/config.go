package cfprovision

import (
	"fmt"
	"io"

	"github.com/irgipaulius/cogs/pkg/cfclient"
)

// Config drives Cloudflare provisioning for one app.
type Config struct {
	Client *cfclient.Client
	Log    io.Writer
}

// NewConfig helper with log.
func NewConfig(wsRoot string, log io.Writer) Config {
	return Config{Client: &cfclient.Client{WSRoot: wsRoot}, Log: log}
}

func logf(log io.Writer, format string, args ...any) {
	if log == nil {
		return
	}
	fmt.Fprintf(log, format+"\n", args...)
}
