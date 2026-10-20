package workspace

import (
	"path/filepath"

	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/portalloc"
)

// Deploy holds workspace-wide deployment defaults.
type Deploy struct {
	BindHost      string `json:"bind_host"`
	BasePort      int    `json:"base_port"`
	PortStep      int    `json:"port_step"`
	ReservedPorts []int `json:"reserved_ports"`
}

// ApplyDeployDefaults fills empty deploy fields.
func (r *Root) ApplyDeployDefaults() {
	if r.Deploy.BindHost == "" {
		r.Deploy.BindHost = "127.0.0.1"
	}
	if r.Deploy.BasePort == 0 {
		r.Deploy.BasePort = portalloc.DefaultConfig().BasePort
	}
	if r.Deploy.PortStep == 0 {
		r.Deploy.PortStep = 1
	}
}

// PortConfig builds portalloc.Config from workspace deploy settings.
func (r *Root) PortConfig() portalloc.Config {
	r.ApplyDeployDefaults()
	def := portalloc.DefaultConfig()
	res := portalloc.MergeReserved(def.Reserved, r.Deploy.ReservedPorts)
	return portalloc.Config{
		BasePort: r.Deploy.BasePort,
		PortStep: r.Deploy.PortStep,
		Reserved: res,
	}
}

// AssignPorts sets manifest ports for apps with port 0 in cogs.launch.json.
func (r *Root) AssignPorts(wsRoot string) error {
	r.ApplyDeployDefaults()
	cfg := r.PortConfig()
	used := map[int]struct{}{}
	var manifestPorts []int
	for _, a := range r.Apps {
		dir := filepath.Join(wsRoot, a.Path)
		m, err := launch.Load(dir)
		if err != nil {
			continue
		}
		manifestPorts = append(manifestPorts, m.Port)
	}
	used = portalloc.CollectUsed(manifestPorts)

	for i, a := range r.Apps {
		dir := filepath.Join(wsRoot, a.Path)
		m, err := launch.Load(dir)
		if err != nil {
			return err
		}
		if m.Port > 0 {
			used[m.Port] = struct{}{}
			continue
		}
		p := cfg.ForIndex(i, used)
		used[p] = struct{}{}
		m.Port = p
		if err := launch.Save(dir, m); err != nil {
			return err
		}
	}
	return nil
}

// PortForApp resolves listen port for an app (manifest or workspace index).
func (r *Root) PortForApp(wsRoot, appName string, manifest launch.Manifest) (int, string) {
	r.ApplyDeployDefaults()
	if manifest.Port > 0 {
		return manifest.Port, r.Deploy.BindHost
	}
	cfg := r.PortConfig()
	used := map[int]struct{}{}
	idx := 0
	found := false
	for i, a := range r.Apps {
		dir := filepath.Join(wsRoot, a.Path)
		m, err := launch.Load(dir)
		if err == nil && m.Port > 0 {
			used[m.Port] = struct{}{}
		}
		if a.Name == appName || a.Path == appName {
			idx = i
			found = true
		}
	}
	if !found {
		for i, a := range r.Apps {
			if filepath.Join(wsRoot, a.Path) == appName {
				idx = i
				found = true
				break
			}
		}
	}
	p := cfg.ForIndex(idx, used)
	return p, r.Deploy.BindHost
}
