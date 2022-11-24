package daemon

import (
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/logrusorgru/aurora/v3"
	"golang.org/x/mod/modfile"

	"encr.dev/cli/daemon/run"
	"encr.dev/cli/internal/onboarding"
	"encr.dev/parser"
	"encr.dev/pkg/appfile"
	"encr.dev/pkg/errlist"
	"encr.dev/pkg/experiments"
	"encr.dev/pkg/vcs"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

// OnStart implements run.EventListener.
func (s *Server) OnStart(r *run.Run) {}

// OnReload implements run.EventListener.
func (s *Server) OnReload(r *run.Run) {}

// OnStop implements run.EventListener.
func (s *Server) OnStop(r *run.Run) {}

// OnStdout implements run.EventListener.
func (s *Server) OnStdout(r *run.Run, line []byte) {
	s.mu.Lock()
	slog, ok := s.streams[r.ID]
	s.mu.Unlock()

	if ok {
		slog.Stdout(true).Write(line)
	}
}

// OnStderr implements run.EventListener.
func (s *Server) OnStderr(r *run.Run, line []byte) {
	s.mu.Lock()
	slog, ok := s.streams[r.ID]
	s.mu.Unlock()

	if ok {
		slog.Stderr(true).Write(line)
	}
}

func (s *Server) OnError(r *run.Run, err *errlist.List) {
	s.mu.Lock()
	slog, ok := s.streams[r.ID]
	s.mu.Unlock()

	if ok {
		slog.Error(err)
	}
}

// parseApp parses the app.
func (s *Server) parseApp(appRoot, workingDir string, parseTests bool) (*parser.Result, error) {
	modPath := filepath.Join(appRoot, "go.mod")
	modData, err := ioutil.ReadFile(modPath)
	if err != nil {
		return nil, err
	}
	mod, err := modfile.Parse(modPath, modData, nil)
	if err != nil {
		return nil, err
	}

	exp, err := appfile.Experiments(appRoot)
	if err != nil {
		return nil, err
	}
	expSet, err := experiments.NewSet(exp, nil)
	if err != nil {
		return nil, err
	}

	vcsRevision := vcs.GetRevision(appRoot)

	cfg := &parser.Config{
		AppRoot:                  appRoot,
		Experiments:              expSet,
		AppRevision:              vcsRevision.Revision,
		AppHasUncommittedChanges: vcsRevision.Uncommitted,
		ModulePath:               mod.Module.Mod.Path,
		WorkingDir:               workingDir,
		ParseTests:               parseTests,
	}
	return parser.Parse(cfg)
}

func showFirstRunExperience(run *run.Run, md *meta.Data, stdout io.Writer) {
	if state, err := onboarding.Load(); err == nil {
		if !state.FirstRun.IsSet() {
			// Is there a suitable endpoint to call?
			var rpc *meta.RPC
			var command string
			for _, svc := range md.Svcs {
				for _, r := range svc.Rpcs {
					if cmd := genCurlCommand(run, md, r); rpc == nil || len(command) < len(cmd) {
						rpc = r
						command = cmd
					}
				}
			}
			if rpc != nil {
				state.FirstRun.Set()
				if err := state.Write(); err == nil {
					stdout.Write([]byte(aurora.Sprintf("\nHint: make an API call by running: %s\n", aurora.Cyan(command))))
				}
			}
		}
	}
}

// findAvailableAddr attempts to find an available host:port that's near
// the given startAddr.
func findAvailableAddr(startAddr string) (host string, port int, ok bool) {
	host, portStr, err := net.SplitHostPort(startAddr)
	if err != nil {
		host = "localhost"
		portStr = "4000"
	}
	startPort, err := strconv.Atoi(portStr)
	if err != nil {
		startPort = 4000
	}

	for p := startPort + 1; p <= startPort+10 && p <= 65535; p++ {
		addr := host + ":" + strconv.Itoa(p)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			ln.Close()
			return host, p, true
		}
	}
	return "", 0, false
}

func genCurlCommand(run *run.Run, md *meta.Data, rpc *meta.RPC) string {
	var payload []byte
	method := rpc.HttpMethods[0]
	switch method {
	case "GET", "HEAD", "DELETE":
		// doesn't use HTTP body payloads
	default:
		payload = genSchema(md, rpc.RequestSchema)
	}

	var segments []string
	for _, seg := range rpc.Path.Segments {
		var v string
		switch seg.Type {
		default:
			v = "foo"
		case meta.PathSegment_LITERAL:
			v = seg.Value
		case meta.PathSegment_WILDCARD:
			v = "foo"
		case meta.PathSegment_PARAM:
			switch seg.ValueType {
			case meta.PathSegment_STRING:
				v = "foo"
			case meta.PathSegment_BOOL:
				v = "true"
			case meta.PathSegment_INT8, meta.PathSegment_INT16, meta.PathSegment_INT32, meta.PathSegment_INT64,
				meta.PathSegment_UINT8, meta.PathSegment_UINT16, meta.PathSegment_UINT32, meta.PathSegment_UINT64:
				v = "1"
			case meta.PathSegment_UUID:
				v = "be23a21f-d12c-432c-91ec-fb8a52e23967" // some random UUID
			default:
				v = "foo"
			}
		}
		segments = append(segments, v)
	}

	parts := []string{"curl"}
	if (payload != nil && method != "POST") || (payload == nil && method != "GET") {
		parts = append(parts, " -X ", method)
	}
	path := "/" + strings.Join(segments, "/")
	parts = append(parts, " http://", run.ListenAddr, path)
	if payload != nil {
		parts = append(parts, " -d '", string(payload), "'")
	}
	return strings.Join(parts, "")
}

// errIsAddrInUse reports whether the error is due to the address already being in use.
func errIsAddrInUse(err error) bool {
	if opErr, ok := err.(*net.OpError); ok {
		if syscallErr, ok := opErr.Err.(*os.SyscallError); ok {
			if errno, ok := syscallErr.Err.(syscall.Errno); ok {
				const WSAEADDRINUSE = 10048
				switch {
				case errno == syscall.EADDRINUSE:
					return true
				case runtime.GOOS == "windows" && errno == WSAEADDRINUSE:
					return true
				}
			}
		}
	}
	return false
}
