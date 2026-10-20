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
	out := serviceRenderOutputPath(kind, rel)
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return err
	}
	if kind == "rcd" {
		// Installer must be executable; rendered file in cwd is a template copy.
		fmt.Printf("Wrote %s\n", out)
		fmt.Println("Install: sudo install -m 555 cogs-app.rc /usr/local/etc/rc.d/cogs_app")
		fmt.Println("Enable:  echo 'cogs_app_enable=\"YES\"' | sudo tee -a /etc/rc.conf")
		return nil
	}
	fmt.Printf("Wrote %s\n", out)
	return nil
}

func serviceRenderOutputPath(kind, rel string) string {
	switch kind {
	case "systemd":
		return "cogs-app.service"
	case "rcd":
		return "cogs-app.rc"
	case "launchd":
		return "com.cogs.app.plist"
	default:
		return filepath.Base(rel)
	}
}
