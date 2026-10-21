package supervisor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

// Server is the unix-socket control plane.
type Server struct {
	SocketPath string
	AdminDB    string
	Store      *adminstore.Store
	Metrics    *MetricsRing
	Registry   Registry
	mu         sync.Mutex
}

// DefaultSocketPath under workspace .cogs.
func DefaultSocketPath(wsRoot string) string {
	return filepath.Join(wsRoot, ".cogs", "supervisor.sock")
}

// Run listens until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(s.SocketPath), 0o750); err != nil {
		return err
	}
	ln, err := listenUnix(s.SocketPath)
	if err != nil {
		return err
	}
	defer ln.Close()
	defer os.Remove(s.SocketPath)

	stop := make(chan struct{})
	go s.Metrics.Start(stop)
	defer close(stop)

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return err
			}
		}
		go s.handleConn(conn)
	}
}

// listenUnix removes a leftover socket file from a crashed supervisor, then binds.
func listenUnix(path string) (net.Listener, error) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	ln, err := net.Listen("unix", path)
	if err == nil {
		return ln, nil
	}
	_ = os.Remove(path)
	return net.Listen("unix", path)
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	if !peerAllowed(conn) {
		writeResp(conn, Response{OK: false, Error: "forbidden peer"})
		return
	}
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	if !sc.Scan() {
		return
	}
	var req Request
	if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
		writeResp(conn, Response{OK: false, Error: "bad json"})
		return
	}
	resp := s.dispatch(context.Background(), req)
	writeResp(conn, resp)
}

func writeResp(conn net.Conn, resp Response) {
	b, _ := json.Marshal(resp)
	b = append(b, '\n')
	_, _ = conn.Write(b)
}

func (s *Server) dispatch(ctx context.Context, req Request) Response {
	user, err := s.Store.ResolveSession(ctx, req.Session)
	if err != nil {
		return Response{OK: false, Error: err.Error()}
	}
	switch req.Op {
	case "metrics", "cockpit":
		if !user.HasPermission("*", adminstore.PermMetricsRead) {
			return Response{OK: false, Error: "denied"}
		}
		return Response{OK: true, Data: encodeResponse(s.Metrics.Board(s.Registry))}
	case "metrics_stream_hint":
		if !user.HasPermission("*", adminstore.PermMetricsRead) {
			return Response{OK: false, Error: "denied"}
		}
		return Response{OK: true, Data: encodeResponse(s.Metrics.LatestForStream())}
	case "telemetry":
		if !user.HasPermission("*", adminstore.PermMetricsRead) && !user.HasPermission(req.App, adminstore.PermMetricsRead) {
			return Response{OK: false, Error: "denied"}
		}
		dir, ok := s.Metrics.appPaths[req.App]
		if !ok {
			dir = filepath.Join(s.Registry.WorkspaceRoot, "apps", req.App)
		}
		return Response{OK: true, Data: encodeResponse(s.Metrics.loadCockpit(req.App, dir))}
	case "logs":
		if !user.HasPermission(req.App, adminstore.PermLogsRead) {
			return Response{OK: false, Error: "denied"}
		}
		dir, ok := s.Metrics.appPaths[req.App]
		if !ok {
			dir = filepath.Join(s.Registry.WorkspaceRoot, "apps", req.App)
		}
		kind := req.LogKind
		if kind == "" {
			kind = "process"
		}
		lines, err := TailLog(dir, kind, 400)
		if err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true, Data: encodeResponse(lines)}
	case "registry":
		if !user.IsRoot && !user.HasPermission("*", adminstore.PermMetricsRead) {
			return Response{OK: false, Error: "denied"}
		}
		pub := s.Registry
		pub.Apps = append([]AppDB(nil), pub.Apps...)
		for i := range pub.Apps {
			pub.Apps[i].Path = ""
		}
		return Response{OK: true, Data: encodeResponse(pub)}
	case "sql_tables":
		if !user.HasPermission(req.App, adminstore.PermSQLRead) {
			return Response{OK: false, Error: "denied"}
		}
		tables, err := ListTables(ctx, s.Registry, req.DBID)
		if err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true, Data: encodeResponse(tables)}
	case "sql":
		perm := adminstore.PermSQLRead
		if req.Write {
			perm = adminstore.PermSQLWrite
		}
		if !user.HasPermission(req.App, perm) {
			return Response{OK: false, Error: "denied"}
		}
		res, err := RunSQL(ctx, s.Registry, req.DBID, req.SQL, req.Write)
		if err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		_ = s.Store.WriteAudit(ctx, user, "sql", req.DBID+": "+truncate(req.SQL, 200))
		return Response{OK: true, Data: encodeResponse(res)}
	case "redeploy":
		if !user.HasPermission(req.App, adminstore.PermRedeploy) {
			return Response{OK: false, Error: "denied"}
		}
		detail, err := Redeploy(ctx, s.Registry.WorkspaceRoot, req.App)
		if err != nil {
			_ = s.Store.WriteAudit(ctx, user, "redeploy_fail", req.App+": "+detail)
			return Response{OK: false, Error: err.Error() + ": " + detail}
		}
		_ = s.Store.WriteAudit(ctx, user, "redeploy", req.App+": "+truncate(detail, 500))
		return Response{OK: true, Data: encodeResponse(map[string]string{"detail": detail})}
	case "rollback":
		if !user.HasPermission(req.App, adminstore.PermRedeploy) {
			return Response{OK: false, Error: "denied"}
		}
		detail, err := rollbackApp(ctx, s.Registry.WorkspaceRoot, req.App)
		if err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		_ = s.Store.WriteAudit(ctx, user, "rollback", req.App+": "+truncate(detail, 200))
		return Response{OK: true, Data: encodeResponse(map[string]string{"detail": detail})}
	case "deploy_status":
		data, err := deployStatus(s.Registry.WorkspaceRoot)
		if err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true, Data: data}
	case "github_status":
		return Response{OK: true, Data: githubStatus(s.Registry.WorkspaceRoot)}
	case "github_login_start":
		if !user.IsRoot {
			return Response{OK: false, Error: "denied"}
		}
		data, err := githubDeviceStart(s.Registry.WorkspaceRoot)
		if err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true, Data: data}
	case "github_login_poll":
		if !user.IsRoot {
			return Response{OK: false, Error: "denied"}
		}
		var body struct {
			DeviceCode string `json:"device_code"`
		}
		_ = json.Unmarshal(req.Raw, &body)
		data, err := githubDevicePoll(s.Registry.WorkspaceRoot, body.DeviceCode)
		if err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true, Data: data}
	case "github_repos":
		if !user.IsRoot {
			return Response{OK: false, Error: "denied"}
		}
		data, err := githubRepos(s.Registry.WorkspaceRoot)
		if err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true, Data: data}
	case "add_app":
		if !user.IsRoot {
			return Response{OK: false, Error: "denied"}
		}
		detail, err := addAppFromRequest(ctx, s.Registry.WorkspaceRoot, req.Raw)
		if err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		_ = s.Store.WriteAudit(ctx, user, "add_app", detail)
		return Response{OK: true, Data: encodeResponse(map[string]string{"detail": detail})}
	case "ping":
		return Response{OK: true, Data: encodeResponse(map[string]string{"status": "ok"})}
	default:
		return Response{OK: false, Error: fmt.Sprintf("unknown op %q", req.Op)}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// NewServerFromWorkspace builds a supervisor for wsRoot.
func NewServerFromWorkspace(wsRoot, adminDBPath string) (*Server, error) {
	reg, err := LoadRegistry(wsRoot)
	if err != nil {
		return nil, err
	}
	_, ws, err := workspace.FindRoot(wsRoot)
	if err != nil {
		return nil, err
	}
	store, err := adminstore.Open(adminDBPath)
	if err != nil {
		return nil, err
	}
	sock := os.Getenv("COGS_SUPERVISOR_SOCKET")
	if sock == "" {
		sock = DefaultSocketPath(wsRoot)
	}
	return &Server{
		SocketPath: sock,
		AdminDB:    adminDBPath,
		Store:      store,
		Metrics:    NewMetricsRing(wsRoot, ws),
		Registry:   reg,
	}, nil
}
