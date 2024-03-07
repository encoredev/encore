// Package daemon implements the Encore daemon gRPC server.
package daemon

import (
	"bytes"
	"context"
	"io"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/namespace"
	"encr.dev/cli/daemon/run"
	"encr.dev/cli/daemon/secret"
	"encr.dev/cli/daemon/sqldb"
	"encr.dev/cli/internal/platform"
	"encr.dev/cli/internal/update"
	"encr.dev/internal/clientgen"
	"encr.dev/internal/version"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/builder/builderimpl"
	"encr.dev/pkg/errlist"
	"encr.dev/pkg/fns"
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
	ns   *namespace.Manager

	mu      sync.Mutex
	streams map[string]*streamLog // run id -> stream

	availableVerInit sync.Once
	availableVer     atomic.Value // string

	appDebounceMu sync.Mutex
	appDebouncers map[*apps.Instance]*regenerateCodeDebouncer

	daemonpb.UnimplementedDaemonServer
}

// New creates a new Server.
func New(appsMgr *apps.Manager, mgr *run.Manager, cm *sqldb.ClusterManager, sm *secret.Manager, ns *namespace.Manager) *Server {
	srv := &Server{
		apps:    appsMgr,
		mgr:     mgr,
		cm:      cm,
		sm:      sm,
		ns:      ns,
		streams: make(map[string]*streamLog),

		appDebouncers: make(map[*apps.Instance]*regenerateCodeDebouncer),
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
		expSet, err := app.Experiments(nil)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "failed to parse app experiments: %v", err)
		}

		// Parse the app to figure out what infrastructure is needed.
		bld := builderimpl.Resolve(expSet)
		defer fns.CloseIgnore(bld)
		parse, err := bld.Parse(ctx, builder.ParseParams{
			Build:       builder.DefaultBuildInfo(),
			App:         app,
			Experiments: expSet,
			WorkingDir:  ".",
			ParseTests:  false,
		})
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "failed to parse app metadata: %v", err)
		}
		md = parse.Meta

		if err := app.CacheMetadata(md); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to cache app metadata: %v", err)
		}
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
	serviceNames := params.Services
	if slices.Contains(serviceNames, "*") {
		serviceNames = nil
	}
	code, err := clientgen.Client(lang, params.AppId, md, serviceNames)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &daemonpb.GenClientResponse{Code: code}, nil
}

func (s *Server) SecretsRefresh(ctx context.Context, req *daemonpb.SecretsRefreshRequest) (*daemonpb.SecretsRefreshResponse, error) {
	app, err := s.apps.Track(req.AppRoot)
	if err != nil {
		return nil, err
	}
	s.sm.UpdateKey(app.PlatformID(), req.Key, req.Value)
	return &daemonpb.SecretsRefreshResponse{}, nil
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
	_ = stream.Send(&daemonpb.CommandMessage{Msg: &daemonpb.CommandMessage_Exit{
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
	_ = err.SendToStream(log.stream)
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

	_, _ = log.writeStream(false, stderr)
	_, _ = log.writeStream(true, stdout)
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
