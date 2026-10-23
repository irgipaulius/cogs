package cli

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appbuild"
	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/gitrepo"
	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

type doctorFinding struct {
	App     string
	OK      bool
	Problem string
	Fix     string
}

func doctorWorkspace(root string, ws workspace.Root) []doctorFinding {
	var out []doctorFinding
	if paths, err := gitrepo.RootOwnedGitFiles(root); err == nil {
		if msg := gitrepo.FormatOwnershipWarning(root, paths); msg != "" {
			out = append(out, doctorFinding{
				OK: false, Problem: msg,
				Fix: fmt.Sprintf("sudo chown -R $(whoami) %s/.git", root),
			})
		}
	}
	goWork := ""
	if b, err := os.ReadFile(filepath.Join(root, "go.work")); err == nil {
		goWork = string(b)
	} else {
		out = append(out, doctorFinding{
			OK: false, Problem: "go.work missing at workspace root",
			Fix: "create go.work and add each app module with: go work use ./apps/<app>",
		})
	}

	tunnels := map[string]string{}
	ports := map[int]string{}
	for _, a := range ws.Apps {
		dir := filepath.Join(root, a.Path)
		out = append(out, doctorOne(dir, a, goWork)...)
		m, err := launch.Load(dir)
		if err != nil {
			continue
		}
		if m.TunnelName != "" {
			if prev, ok := tunnels[m.TunnelName]; ok {
				out = append(out, doctorFinding{
					App: a.Name, OK: false,
					Problem: fmt.Sprintf("tunnel %q is also used by %s", m.TunnelName, prev),
					Fix:     "give each app its own tunnel_name in cogs.app.toml, then cogs launch <app>",
				})
			}
			tunnels[m.TunnelName] = a.Name
		}
		if m.Port != 0 {
			if prev, ok := ports[m.Port]; ok {
				out = append(out, doctorFinding{
					App: a.Name, OK: false,
					Problem: fmt.Sprintf("port %d is also used by %s", m.Port, prev),
					Fix:     "set a unique port in cogs.app.toml",
				})
			}
			ports[m.Port] = a.Name
		}
	}
	out = append(out, doctorSocket(root))
	return out
}

func doctorOne(dir string, a workspace.App, goWork string) []doctorFinding {
	var out []doctorFinding
	app := a.Name
	if _, err := os.Stat(filepath.Join(dir, appcfg.Filename)); err != nil {
		out = append(out, doctorFinding{
			App: app, OK: false,
			Problem: "no cogs.app.toml (launch.json can drift from .env, go.work, and config.js)",
			Fix:     "add " + filepath.Join(a.Path, appcfg.Filename) + " and treat it as the source of truth",
		})
	}
	rel := "./" + filepath.ToSlash(a.Path)
	if goWork != "" && !strings.Contains(goWork, rel) && !strings.Contains(goWork, a.Path) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			out = append(out, doctorFinding{
				App: app, OK: false,
				Problem: "go.work does not include " + rel,
				Fix:     "go work use " + rel,
			})
		}
	}
	m, err := launch.Load(dir)
	if err != nil {
		out = append(out, doctorFinding{App: app, OK: false, Problem: "cannot load app config: " + err.Error(), Fix: "fix cogs.app.toml or cogs.launch.json"})
		return out
	}
	if m.Zone == "" {
		out = append(out, doctorFinding{
			App: app, OK: false,
			Problem: "zone is unset, so cogs launch will prompt",
			Fix:     `set zone = "<apex>" in ` + filepath.Join(a.Path, appcfg.Filename),
		})
	}
	if m.Zone != "" {
		wantPage := m.PageSubdomain + "." + m.Zone
		wantAPI := m.APISubdomain + "." + m.Zone
		if m.IsCombined() {
			wantAPI = wantPage
		}
		envPath := filepath.Join(dir, ".env")
		if listen, ok := listenFromEnv(envPath); ok {
			host, _, err := net.SplitHostPort(listen)
			if err == nil && host != "127.0.0.1" && host != "localhost" && host != "::1" {
				out = append(out, doctorFinding{
					App: app, OK: false,
					Problem: fmt.Sprintf("COGS_LISTEN=%s binds beyond loopback", listen),
					Fix:     "use bind_host in cogs.app.toml and COGS_LISTEN=127.0.0.1:<port>, or set COGS_ALLOW_PUBLIC_BIND=1 if intentional",
				})
			}
		}
		if page, api, ok := hostsFromEnv(envPath); ok {
			if page != wantPage || api != wantAPI {
				out = append(out, doctorFinding{
					App: app, OK: false,
					Problem: fmt.Sprintf(".env hosts are %s and %s, config wants %s and %s", page, api, wantPage, wantAPI),
					Fix:     "set COGS_PUBLIC_PAGE_ORIGIN=https://" + wantPage + " and COGS_PUBLIC_API_URL=https://" + wantAPI + " in " + filepath.Join(a.Path, ".env"),
				})
			}
		}
		distCfg := filepath.Join(appbuild.PagesDistDir(dir), "config.js")
		b, err := os.ReadFile(distCfg)
		if err != nil && !m.IsCombined() {
			out = append(out, doctorFinding{
				App: app, OK: false,
				Problem: ".cogs/pages-dist missing (Worker UI not built for production)",
				Fix:     "wait for reconcile worker stage or run: cogs launch " + app + " -y",
			})
		} else if err == nil && !m.IsCombined() {
			if !configDeclares(string(b), wantPage, wantAPI) {
				out = append(out, doctorFinding{
					App: app, OK: false,
					Problem: "pages-dist config.js does not point at https://" + wantAPI,
					Fix:     "re-run worker stage after Cloudflare auth is fixed",
				})
			}
		}
		out = append(out, doctorDNS(app, wantPage, wantAPI, m.IsCombined())...)
		out = append(out, doctorToken(dir, app, m.Zone))
	}
	return out
}

func configDeclares(body, pageHost, apiHost string) bool {
	return strings.Contains(body, `pageOrigin: "https://`+pageHost+`"`) &&
		strings.Contains(body, `apiBase: "https://`+apiHost+`"`)
}

func doctorDNS(app, pageHost, apiHost string, combined bool) []doctorFinding {
	host := apiHost
	want := "cfargotunnel.com"
	if combined {
		host = pageHost
	}
	cname, ok := publicCNAME(host)
	if !ok {
		return nil
	}
	if strings.Contains(cname, want) || strings.Contains(cname, "pages.dev") {
		return nil
	}
	return []doctorFinding{{
		App: app, OK: false,
		Problem: host + " CNAME is " + cname,
		Fix:     "cogs launch " + app + " -y  (API CNAME → tunnel; UI → Worker custom domain, not pages.dev)",
	}}
}

// publicCNAME returns a CNAME that differs from the hostname.
// Proxied Cloudflare records often resolve as the name itself, which is not a mismatch.
func publicCNAME(host string) (string, bool) {
	cname, err := net.LookupCNAME(host)
	if err != nil {
		return "", false
	}
	cname = strings.TrimSuffix(strings.ToLower(cname), ".")
	if cname == "" || cname == strings.ToLower(host) {
		return "", false
	}
	return cname, true
}

func doctorToken(projectDir, app, zone string) doctorFinding {
	token, err := cfTokenDNS(projectDir, zone)
	if err != nil || token == "" {
		return doctorFinding{
			App: app, OK: false,
			Problem: "no Cloudflare DNS token for zone " + zone,
			Fix:     "cogs launch " + app + "  (saves one workspace token under .cogs/cloudflare/)",
		}
	}
	if !tokenCanEditDNS(token, zone) {
		return doctorFinding{
			App: app, OK: false,
			Problem: "DNS token cannot edit zone " + zone,
			Fix:     "create a Zone DNS Edit token for " + zone + " and paste it when cogs launch asks",
		}
	}
	return doctorFinding{App: app, OK: true, Problem: "DNS token can edit " + zone}
}

func doctorSocket(root string) doctorFinding {
	sock := filepath.Join(root, ".cogs", "supervisor.sock")
	if _, err := os.Stat(sock); err != nil {
		return doctorFinding{OK: true, Problem: "supervisor socket not present"}
	}
	if supervisorSocketLive(sock) {
		return doctorFinding{OK: true, Problem: "supervisor socket is accepting connections"}
	}
	return doctorFinding{
		OK: false,
		Problem: "stale supervisor socket " + sock,
		Fix:     "rm " + sock + " && cogs up",
	}
}

func printDoctor(findings []doctorFinding) int {
	fails := 0
	for _, f := range findings {
		mark := "ok  "
		if !f.OK {
			mark = "FAIL"
			fails++
		}
		name := f.App
		if name == "" {
			name = "workspace"
		}
		fmt.Printf("%s  %s  %s\n", mark, name, f.Problem)
		if f.Fix != "" && !f.OK {
			fmt.Printf("      fix: %s\n", f.Fix)
		}
	}
	if fails == 0 {
		fmt.Println("doctor: all checks passed")
		return 0
	}
	fmt.Printf("doctor: %d problem(s)\n", fails)
	return 1
}
