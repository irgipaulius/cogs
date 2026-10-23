package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/pkg/cockpit"
	"github.com/irgipaulius/cogs/pkg/dnsguard"
	"github.com/irgipaulius/cogs/pkg/release"
)

func importLegacyOnce(s *Store) error {
	marker := filepath.Join(s.wsRoot, ".cogs", "legacy-imported")
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	count, _ := s.countApps()
	if count > 0 {
		_ = os.WriteFile(marker, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644)
		return archiveLegacyFiles(s.wsRoot)
	}
	if err := importInstalledJSON(s); err != nil {
		return err
	}
	if err := importDeployments(s); err != nil {
		return err
	}
	if err := importDNS(s); err != nil {
		return err
	}
	_ = os.WriteFile(marker, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644)
	return archiveLegacyFiles(s.wsRoot)
}

func (s *Store) countApps() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM apps`).Scan(&n)
	return n, err
}

type legacyInstalledApp struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	GitRepo string `json:"git_repo,omitempty"`
	GitRoot string `json:"git_root,omitempty"`
}

func importInstalledJSON(s *Store) error {
	path := filepath.Join(s.wsRoot, ".cogs", "installed.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m struct {
		Apps []legacyInstalledApp `json:"apps"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	for _, ia := range m.Apps {
		d := AppDesired{
			ID:         ia.Name,
			Present:    true,
			Repo:       ia.GitRepo,
			Branch:     "main",
			AppPath:    ia.Path,
			GitRoot:    ia.GitRoot,
			DeployMode: "push",
		}
		if err := s.SetDesired(d); err != nil {
			return err
		}
	}
	return nil
}

func importDeployments(s *Store) error {
	df, err := cockpit.LoadDeployments(s.wsRoot)
	if err != nil {
		return err
	}
	for appID, meta := range df.Meta {
		if meta.GitCommit != "" {
			bin := release.BinaryPath(s.wsRoot, appID, meta.GitCommit, appID == "admin")
			id, err := s.UpsertRelease(appID, meta.GitCommit, bin, meta.LastStatus == "succeeded")
			if err != nil {
				return err
			}
			if meta.LastStatus == "succeeded" && id > 0 {
				_ = s.ActivateRelease(appID, id)
			}
		}
		if meta.LastStatus == "failed" {
			_ = s.SetConverged(appID, false, meta.LastError)
		}
	}
	return nil
}

func importDNS(s *Store) error {
	recs, err := dnsguard.Load(s.wsRoot)
	if err != nil {
		return err
	}
	for host, rec := range recs {
		kind := rec.Kind
		if kind == "" {
			kind = "page"
		}
		_ = s.ClaimHostname(rec.App, strings.ToLower(host), kind)
	}
	return nil
}

func archiveLegacyFiles(wsRoot string) error {
	legacy := filepath.Join(wsRoot, ".cogs", "legacy")
	_ = os.MkdirAll(legacy, 0o750)
	for _, name := range []string{"installed.json", "deployments.json", "dns-ownership.json"} {
		src := filepath.Join(wsRoot, ".cogs", name)
		if b, err := os.ReadFile(src); err == nil {
			_ = os.WriteFile(filepath.Join(legacy, name), b, 0o644)
			_ = os.Remove(src)
		}
	}
	inst := filepath.Join(wsRoot, ".cogs", "installs")
	if st, err := os.Stat(inst); err == nil && st.IsDir() {
		_ = os.Rename(inst, filepath.Join(legacy, "installs"))
	}
	return nil
}

// ExportAppsJSON mirrors ListApps for SSE payloads.
func (s *Store) ExportAppsJSON() ([]byte, error) {
	apps, err := s.ListApps()
	if err != nil {
		return nil, err
	}
	return json.Marshal(apps)
}
