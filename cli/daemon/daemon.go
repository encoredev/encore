// Package daemon implements the Encore daemon gRPC server.
package daemon

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"encr.dev/cli/daemon/internal/appfile"
	"encr.dev/cli/daemon/run"
	"encr.dev/cli/daemon/secret"
	"encr.dev/cli/daemon/sqldb"
	"encr.dev/cli/internal/codegen"
	"encr.dev/cli/internal/update"
	daemonpb "encr.dev/proto/encore/daemon"
	meta "encr.dev/proto/encore/parser/meta/v1"
	"encr.dev/proto/encore/server/remote"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/mod/semver"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ daemonpb.DaemonServer = (*Server)(nil)

// Server implements daemonpb.DaemonServer.
type Server struct {
	version string
	mgr     *run.Manager
	cm      *sqldb.ClusterManager
	sm      *secret.Manager
	rc      remote.RemoteClient

	mu       sync.Mutex
	streams  map[string]*streamLog // run id -> stream
	appRoots map[string]string     // cache of app id -> app root

	availableVerInit sync.Once
	availableVer     atomic.Value // string

	daemonpb.UnimplementedDaemonServer
}

// New creates a new Server.
func New(version string, mgr *run.Manager, cm *sqldb.ClusterManager, sm *secret.Manager, rc remote.RemoteClient) *Server {
	srv := &Server{
		version:  version,
		mgr:      mgr,
		cm:       cm,
		sm:       sm,
		rc:       rc,
		streams:  make(map[string]*streamLog),
		appRoots: make(map[string]string),
	}
	mgr.AddListener(srv)
	// Check immediately for the latest version to avoid blocking 'encore run'
	go srv.availableUpdate()
	return srv
}

// GenClient generates a client based on the app's API.
func (s *Server) GenClient(ctx context.Context, params *daemonpb.GenClientRequest) (*daemonpb.GenClientResponse, error) {
	var md *meta.Data
	if params.EnvName == "local" {
		// Determine the app root
		s.mu.Lock()
		appRoot, ok := s.appRoots[params.AppId]
		s.mu.Unlock()
		if !ok {
			return nil, status.Errorf(codes.FailedPrecondition, "the app %s must be run locally before generating a client for the 'local' environment.",
				params.AppId)
		}

		// Get the app metadata
		result, err := s.parseApp(appRoot, ".", false)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "failed to parse app metadata: %v", err)
		}
		md = result.Meta
	} else {
		meta, err := s.rc.Meta(ctx, &remote.MetaRequest{
			AppSlug: params.AppId,
			EnvName: params.EnvName,
		})
		if err != nil {
			return nil, status.Errorf(status.Code(err), "could not fetch API metadata: %v", err)
		}
		md = meta
	}

	lang := codegen.Lang(params.Lang)
	code, err := codegen.Client(lang, params.AppId, md)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &daemonpb.GenClientResponse{Code: code}, nil
}

// SetSecret sets a secret key on the encore.dev platform.
func (s *Server) SetSecret(ctx context.Context, req *daemonpb.SetSecretRequest) (*daemonpb.SetSecretResponse, error) {
	// Get the app id from the app file
	appSlug, err := appfile.Slug(req.AppRoot)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	} else if appSlug == "" {
		return nil, errNotLinked
	}

	resp, err := s.rc.SetSecret(ctx, &remote.SetSecretRequest{
		AppSlug: appSlug,
		Key:     req.Key,
		Value:   req.Value,
		Type:    remote.SetSecretRequest_Type(req.Type),
	})
	if err != nil {
		return nil, err
	}
	go s.sm.UpdateKey(appSlug, req.Key, req.Value)
	return &daemonpb.SetSecretResponse{Created: resp.Created}, nil
}

// Version reports the daemon version.
func (s *Server) Version(context.Context, *empty.Empty) (*daemonpb.VersionResponse, error) {
	return &daemonpb.VersionResponse{Version: s.version}, nil
}

// Logs streams logs from the encore.dev platform.
func (s *Server) Logs(params *daemonpb.LogsRequest, stream daemonpb.Daemon_LogsServer) error {
	appSlug, err := appfile.Slug(params.AppRoot)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, err.Error())
	} else if appSlug == "" {
		return errNotLinked
	}

	logs, err := s.rc.Logs(stream.Context(), &remote.LogsRequest{
		AppSlug: appSlug,
		EnvName: params.EnvName,
	})
	if err != nil {
		return err
	}
	for {
		msg, err := logs.Recv()
		if status.Code(err) == codes.Canceled {
			return nil
		} else if err != nil {
			return err
		}
		err = stream.Send(&daemonpb.LogsMessage{
			Lines:      msg.Lines,
			DropNotice: msg.DropNotice,
		})
		if err != nil {
			return err
		}
	}
}

// availableUpdate checks for updates to Encore.
// If there is a new version it returns it as a semver string.
func (s *Server) availableUpdate() string {
	check := func() string {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ver, err := update.Check(ctx)
		if err != nil {
			log.Error().Err(err).Msg("could not check for new encore release")
		}
		return ver
	}

	s.availableVerInit.Do(func() {
		ver := check()
		s.availableVer.Store(ver)
		go func() {
			for {
				time.Sleep(1 * time.Hour)
				if ver := check(); ver != "" {
					s.availableVer.Store(ver)
				}
			}
		}()
	})

	curr := s.version
	latest := s.availableVer.Load().(string)
	if semver.Compare(latest, curr) > 0 {
		return latest
	}
	return ""
}

// cacheAppRoot adds the appID -> appRoot mapping to the app root cache.
func (s *Server) cacheAppRoot(appID, appRoot string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appRoots[appID] = appRoot
}

var errNotLinked = (func() error {
	st, err := status.New(codes.FailedPrecondition, "app not linked").WithDetails(
		&errdetails.PreconditionFailure{
			Violations: []*errdetails.PreconditionFailure_Violation{
				{
					Type:        "NOT_LINKED",
					Description: "app is not linked with Encore Cloud",
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}
	return st.Err()
})()

type commandStream interface {
	Send(msg *daemonpb.CommandMessage) error
}

func newStreamLogger(slog *streamLog) zerolog.Logger {
	return zerolog.New(zerolog.ConsoleWriter{Out: zerolog.SyncWriter(slog.Stdout())})
}

type streamWriter struct {
	mu     *sync.Mutex
	stream commandStream
	stderr bool // if true write to stderr, otherwise stdout
}

func (w streamWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	out := &daemonpb.CommandOutput{}
	if w.stderr {
		out.Stderr = b
	} else {
		out.Stdout = b
	}
	err := w.stream.Send(&daemonpb.CommandMessage{
		Msg: &daemonpb.CommandMessage_Output{
			Output: out,
		},
	})
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

func streamExit(stream commandStream, code int) {
	stream.Send(&daemonpb.CommandMessage{Msg: &daemonpb.CommandMessage_Exit{
		Exit: &daemonpb.CommandExit{
			Code: int32(code),
		},
	}})
}

type streamLog struct {
	stream commandStream
	mu     sync.Mutex
}

func (log *streamLog) Stdout() io.Writer {
	return streamWriter{mu: &log.mu, stream: log.stream, stderr: false}
}

func (log *streamLog) Stderr() io.Writer {
	return streamWriter{mu: &log.mu, stream: log.stream, stderr: true}
}
