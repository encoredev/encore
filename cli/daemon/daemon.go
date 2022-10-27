// Package daemon implements the Encore daemon gRPC server.
package daemon

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/run"
	"encr.dev/cli/daemon/secret"
	"encr.dev/cli/daemon/sqldb"
	"encr.dev/cli/internal/appfile"
	"encr.dev/cli/internal/platform"
	"encr.dev/cli/internal/update"
	"encr.dev/compiler"
	"encr.dev/internal/clientgen"
	"encr.dev/internal/version"
	"encr.dev/pkg/errlist"
	daemonpb "encr.dev/proto/encore/daemon"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

var _ daemonpb.DaemonServer = (*Server)(nil)

// Server implements daemonpb.DaemonServer.
type Server struct {
	apps *apps.Manager
	mgr  *run.Manager
	cm   *sqldb.ClusterManager
	sm   *secret.Manager

	mu      sync.Mutex
	streams map[string]*streamLog // run id -> stream

	availableVerInit sync.Once
	availableVer     atomic.Value // string

	appDebounceMu sync.Mutex
	appDebouncers map[*apps.Instance]debouncer

	daemonpb.UnimplementedDaemonServer
}

// New creates a new Server.
func New(appsMgr *apps.Manager, mgr *run.Manager, cm *sqldb.ClusterManager, sm *secret.Manager) *Server {
	srv := &Server{
		apps:    appsMgr,
		mgr:     mgr,
		cm:      cm,
		sm:      sm,
		streams: make(map[string]*streamLog),

		appDebouncers: make(map[*apps.Instance]debouncer),
	}
	mgr.AddListener(srv)

	// Check immediately for the latest version to avoid blocking 'encore run'
	go srv.availableUpdate()

	// Begin watching known apps for changes
	go srv.watchApps()

	return srv
}

// GenClient generates a client based on the app's API.
func (s *Server) GenClient(ctx context.Context, params *daemonpb.GenClientRequest) (*daemonpb.GenClientResponse, error) {
	var md *meta.Data
	if params.EnvName == "local" {
		// Determine the app root
		app, err := s.apps.FindLatestByPlatformID(params.AppId)
		if errors.Is(err, apps.ErrNotFound) {
			return nil, status.Errorf(codes.FailedPrecondition, "the app %s must be run locally before generating a client for the 'local' environment.",
				params.AppId)
		} else if err != nil {
			return nil, status.Errorf(codes.Internal, "unable to query app info: %v", err)
		}

		// Get the app metadata
		result, err := s.parseApp(app.Root(), ".", false)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "failed to parse app metadata: %v", err)
		}
		md = result.Meta
	} else {
		envName := params.EnvName
		if envName == "" {
			envName = "@primary"
		}
		meta, err := platform.GetEnvMeta(ctx, params.AppId, envName)
		if err != nil {
			if strings.Contains(err.Error(), "env_not_found") || strings.Contains(err.Error(), "env_not_deployed") {
				if envName == "@primary" {
					return nil, status.Error(codes.NotFound, "You have no deployments of this application.\n\nYou can generate the client for your local code by setting `--env=local`.")
				}
				return nil, status.Errorf(codes.NotFound, "A deployed environment called `%s` not found.\n\nYou can generate the client for your local code by setting `--env=local`.", envName)
			}
			return nil, status.Errorf(codes.Unavailable, "could not fetch API metadata: %v", err)
		}
		md = meta
	}

	lang := clientgen.Lang(params.Lang)
	code, err := clientgen.Client(lang, params.AppId, md)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &daemonpb.GenClientResponse{Code: code}, nil
}

// GenWrappers generates Encore wrappers.
func (s *Server) GenWrappers(ctx context.Context, params *daemonpb.GenWrappersRequest) (*daemonpb.GenWrappersResponse, error) {
	if err := compiler.GenUserFacing(params.AppRoot); err != nil {
		return nil, status.Errorf(codes.Internal, "unable to generate wrappers: %v", err)
	}
	return &daemonpb.GenWrappersResponse{}, nil
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

	var kind platform.SecretKind
	switch req.Type {
	case daemonpb.SetSecretRequest_DEVELOPMENT:
		kind = platform.DevelopmentSecrets
	case daemonpb.SetSecretRequest_PRODUCTION:
		kind = platform.ProductionSecrets
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown secret type %v", req.Type)
	}

	ver, err := platform.SetAppSecret(ctx, appSlug, kind, req.Key, req.Value)
	if err != nil {
		return nil, err
	}
	go s.sm.UpdateKey(appSlug, req.Key, req.Value)
	return &daemonpb.SetSecretResponse{Created: ver.Number == 1}, nil
}

// Version reports the daemon version.
func (s *Server) Version(context.Context, *empty.Empty) (*daemonpb.VersionResponse, error) {
	configHash, err := version.ConfigHash()
	if err != nil {
		return nil, err
	}

	return &daemonpb.VersionResponse{
		Version:    version.Version,
		ConfigHash: configHash,
	}, nil
}

// availableUpdate checks for updates to Encore.
// If there is a new version it returns it as a semver string.
func (s *Server) availableUpdate() *update.LatestVersion {
	check := func() *update.LatestVersion {
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
				if ver := check(); ver != nil {
					s.availableVer.Store(ver)
				}
			}
		}()
	})

	curr := version.Version
	latest := s.availableVer.Load().(*update.LatestVersion)
	if latest.IsNewer(curr) {
		return latest
	}
	return nil
}

var errDatabaseNotFound = (func() error {
	st := status.New(codes.NotFound, "database not found")
	return st.Err()
})()

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
	return zerolog.New(zerolog.SyncWriter(slog.Stderr(false))).With().Timestamp().Logger()
}

type streamWriter struct {
	mu     *sync.Mutex
	sl     *streamLog
	stderr bool // if true write to stderr, otherwise stdout
	buffer bool
}

func (w streamWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buffer && w.sl.buffered {
		if w.stderr {
			return w.sl.writeBuffered(&w.sl.stderr, b)
		} else {
			return w.sl.writeBuffered(&w.sl.stdout, b)
		}
	}
	return w.sl.writeStream(w.stderr, b)
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

	buffered bool
	stdout   *bytes.Buffer // lazily allocated
	stderr   *bytes.Buffer // lazily allocated
}

func (log *streamLog) Stdout(buffer bool) io.Writer {
	return streamWriter{mu: &log.mu, sl: log, stderr: false, buffer: buffer}
}

func (log *streamLog) Stderr(buffer bool) io.Writer {
	return streamWriter{mu: &log.mu, sl: log, stderr: true, buffer: buffer}
}

func (log *streamLog) Error(err *errlist.List) {
	log.mu.Lock()
	defer log.mu.Unlock()
	err.SendToStream(log.stream)
}

func (log *streamLog) FlushBuffers() {
	var stdout, stderr []byte
	log.mu.Lock()
	defer log.mu.Unlock()
	if b := log.stdout; b != nil {
		stdout = b.Bytes()
		log.stdout = nil
	}
	if b := log.stderr; b != nil {
		stderr = b.Bytes()
		log.stderr = nil
	}

	log.writeStream(false, stderr)
	log.writeStream(true, stdout)
	log.buffered = false
}

func (log *streamLog) writeBuffered(b **bytes.Buffer, p []byte) (int, error) {
	if *b == nil {
		*b = &bytes.Buffer{}
	}
	return (*b).Write(p)
}

func (log *streamLog) writeStream(stderr bool, b []byte) (int, error) {
	out := &daemonpb.CommandOutput{}
	if stderr {
		out.Stderr = b
	} else {
		out.Stdout = b
	}
	err := log.stream.Send(&daemonpb.CommandMessage{
		Msg: &daemonpb.CommandMessage_Output{
			Output: out,
		},
	})
	if err != nil {
		return 0, err
	}
	return len(b), nil
}
