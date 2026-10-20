package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/irgipaulius/cogs/cogstemplate"
)

func cmdService(args []string) error {
	if len(args) < 2 || args[0] != "render" {
		return fmt.Errorf("usage: cogs service render <systemd|rcd|launchd>")
	}
	kind := args[1]
	data := map[string]string{
		"User":     "cogs",
		"WorkDir":  "/var/lib/cogs",
		"EnvFile":  "/etc/cogs/app.env",
		"Binary":   "/usr/local/bin/cogs-app",
	}
	var rel string
	switch kind {
	case "systemd":
		rel = "template/deploy/systemd/cogs-app.service"
	case "rcd":
		rel = "template/deploy/rc.d/cogs"
	case "launchd":
		rel = "template/deploy/launchd/com.cogs.app.plist"
	default:
		return fmt.Errorf("unknown service kind %q", kind)
	}
	raw, err := fs.ReadFile(cogstemplate.FS, rel)
	if err != nil {
		return err
	}
	tmpl, err := template.New(kind).Parse(string(raw))
	if err != nil {
		return err
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		return err
	}
	out := filepath.Base(rel)
	if kind == "systemd" {
		out = "cogs-app.service"
	}
	return os.WriteFile(out, []byte(b.String()), 0o644)
}
