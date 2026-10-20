package cli

import (
	"path/filepath"
	"strconv"

	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/portalloc"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func resolveListen(projectDir string, m *launch.Manifest) (bind string, port int) {
	bind = m.BindHost
	if bind == "" {
		bind = "127.0.0.1"
	}
	port = m.Port
	if root, ws, err := workspace.FindRoot(projectDir); err == nil {
		_ = ws.AssignPorts(root)
		reloaded, err := launch.Load(projectDir)
		if err == nil {
			*m = reloaded
		}
		appRef := filepath.Base(projectDir)
		p, h := ws.PortForApp(root, appRef, *m)
		if m.Port == 0 {
			m.Port = p
		}
		port = m.Port
		if m.BindHost == "" || m.BindHost == "127.0.0.1" {
			bind = h
			m.BindHost = h
		}
		return bind, port
	}
	if port == 0 {
		port = portalloc.DefaultConfig().BasePort
		m.Port = port
	}
	return bind, port
}

func listenAddr(bind string, port int) string {
	return bind + ":" + strconv.Itoa(port)
}
