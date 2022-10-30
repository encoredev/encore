package daemon

import (
	"context"
	"database/sql"
	_ "embed" // for go:embed
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3" // for "sqlite3" driver
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"encr.dev/cli/daemon"
	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/dash"
	"encr.dev/cli/daemon/engine"
	"encr.dev/cli/daemon/engine/trace"
	"encr.dev/cli/daemon/run"
	"encr.dev/cli/daemon/secret"
	"encr.dev/cli/daemon/sqldb"
	"encr.dev/cli/daemon/sqldb/docker"
	"encr.dev/cli/daemon/sqldb/external"
	"encr.dev/cli/internal/xos"
	"encr.dev/internal/conf"
	daemonpb "encr.dev/proto/encore/daemon"
)

// Main runs the daemon.
func Main() {
	if err := redirectLogOutput(); err != nil {
		log.Error().Err(err).Msg("could not setup daemon log file, skipping")
	}
	dev := os.Getenv("ENCORE_DAEMON_DEV") != ""
	if err := runMain(dev); err != nil {
		log.Fatal().Err(err).Msg("daemon failed")
	}
}

func runMain(dev bool) (err error) {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT)
	defer cancel()

	// exit receives signals from the different subsystems
	// that something went wrong and it's time to exit.
	// Sending nil indicates it's time to gracefully exit.
	exit := make(chan error)

	d := &Daemon{dev: dev, exit: exit}
	defer handleBailout(&err)
	defer d.closeAll()

	d.init()
	d.serve()

	select {
	case err := <-exit:
		return err
	case <-ctx.Done():
		return nil
	}
}

// Daemon orchestrates setting up the different daemon subsystems.
type Daemon struct {
	Daemon   *net.UnixListener
	Runtime  *retryingTCPListener
	DBProxy  *retryingTCPListener
	Dash     *retryingTCPListener
	EncoreDB *sql.DB

	Apps       *apps.Manager
	Secret     *secret.Manager
	RunMgr     *run.Manager
	ClusterMgr *sqldb.ClusterManager
	Trace      *trace.Store
	DashSrv    *dash.Server
	Server     *daemon.Server

	dev bool // whether we're in development mode

	// exit is a channel that shuts down the daemon when sent on.
	// A nil error indicates graceful exit.
	exit chan<- error

	// close are the things to close when exiting.
	close []io.Closer
}

func (d *Daemon) init() {
	d.Daemon = d.listenDaemonSocket()
	d.Dash = d.listenTCPRetry("dashboard", 9400)
	d.DBProxy = d.listenTCPRetry("dbproxy", 9500)
	d.Runtime = d.listenTCPRetry("runtime", 9600)
	d.EncoreDB = d.openDB()

	d.Apps = apps.NewManager(d.EncoreDB)

	// If ENCORE_SQLDB_HOST is set, use the external cluster instead of
	// creating our own docker container cluster.
	var sqldbDriver sqldb.Driver = &docker.Driver{}
	if host := os.Getenv("ENCORE_SQLDB_HOST"); host != "" {
		sqldbDriver = &external.Driver{
			Host:              host,
			Database:          os.Getenv("ENCORE_SQLDB_DATABASE"),
			SuperuserUsername: os.Getenv("ENCORE_SQLDB_USER"),
			SuperuserPassword: os.Getenv("ENCORE_SQLDB_PASSWORD"),
		}
		log.Info().Msgf("using external postgres cluster: %s", host)
	}
	d.ClusterMgr = sqldb.NewClusterManager(sqldbDriver, d.Apps)

	d.Trace = trace.NewStore()
	d.Secret = secret.New()
	d.RunMgr = &run.Manager{
		RuntimePort: d.Runtime.Port(),
		DBProxyPort: d.DBProxy.Port(),
		DashPort:    d.Dash.Port(),
		Secret:      d.Secret,
		ClusterMgr:  d.ClusterMgr,
	}
	d.DashSrv = dash.NewServer(d.RunMgr, d.Trace)

	d.Server = daemon.New(d.Apps, d.RunMgr, d.ClusterMgr, d.Secret)
}

func (d *Daemon) serve() {
	go d.serveDaemon()
	go d.serveRuntime()
	go d.serveDBProxy()
	go d.serveDash()
}

// listenDaemonSocket listens on the encored.sock UNIX socket
// and arranges to exit when the socket is closed.
func (d *Daemon) listenDaemonSocket() *net.UnixListener {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		fatal(err)
	}
	socketPath := filepath.Join(userCacheDir, "encore", "encored.sock")
	if err := os.MkdirAll(filepath.Dir(socketPath), 0755); err != nil {
		fatal(err)
	}

	// If the daemon socket already exists, remove it so we can take over listening.
	if _, err := xos.SocketStat(socketPath); err == nil {
		os.Remove(socketPath)
	}
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		fatal(err)
	}
	d.closeOnExit(ln)

	// Detect when the socket is closed.
	go func() {
		d.exit <- detectSocketClose(ln, socketPath)
	}()
	return ln
}

func failedPreconditionError(msg, typ, desc string) error {
	st, err := status.New(codes.FailedPrecondition, msg).WithDetails(
		&errdetails.PreconditionFailure{
			Violations: []*errdetails.PreconditionFailure_Violation{
				{
					Type:        typ,
					Description: desc,
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}
	return st.Err()
}

func ErrInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	resp, err = handler(ctx, req)
	if errors.Is(err, conf.ErrInvalidRefreshToken) {
		return nil, failedPreconditionError("invalid refresh token", "INVALID_REFRESH_TOKEN", "invalid refresh token")
	} else if errors.Is(err, conf.ErrNotLoggedIn) {
		return nil, status.Error(codes.Unauthenticated, "not logged in")
	}
	return resp, err
}

func (d *Daemon) serveDaemon() {
	log.Info().Stringer("addr", d.Daemon.Addr()).Msg("serving daemon")
	srv := grpc.NewServer(grpc.UnaryInterceptor(ErrInterceptor))
	daemonpb.RegisterDaemonServer(srv, d.Server)
	d.exit <- srv.Serve(d.Daemon)
}

func (d *Daemon) serveRuntime() {
	log.Info().Stringer("addr", d.Runtime.Addr()).Msg("serving runtime")
	srv := runtime.NewServer(d.RunMgr, d.Trace)
	d.exit <- http.Serve(d.Runtime, srv)
}

func (d *Daemon) serveDBProxy() {
	log.Info().Stringer("addr", d.DBProxy.Addr()).Msg("serving dbproxy")
	d.exit <- d.ClusterMgr.ServeProxy(d.DBProxy)
}

func (d *Daemon) serveDash() {
	log.Info().Stringer("addr", d.Dash.Addr()).Msg("serving dash")
	srv := dash.NewServer(d.RunMgr, d.Trace)
	d.exit <- http.Serve(d.Dash, srv)
}

// listenTCPRetry listens for TCP connections on the given port, retrying
// in the background if it's already in use.
func (d *Daemon) listenTCPRetry(component string, port int) *retryingTCPListener {
	ln := listenLocalhostTCP(component, port)
	d.closeOnExit(ln)
	return ln
}

// listenTCP listens for TCP connections on a random port on localhost.
// If the daemon is in development mode it always listens on devPort instead.
func (d *Daemon) listenTCP(devPort int) *net.TCPListener {
	port := 0
	if d.dev {
		port = devPort
	}
	addr := "127.0.0.1:" + strconv.Itoa(port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fatal(err)
	}
	d.closeOnExit(ln)
	return ln.(*net.TCPListener)
}

func (d *Daemon) openDB() *sql.DB {
	dir, err := conf.Dir()
	if err != nil {
		fatal(err)
	} else if err := os.MkdirAll(dir, 0755); err != nil {
		fatal(err)
	}
	dbPath := filepath.Join(dir, "encore.db")
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared", dbPath))
	if err != nil {
		fatal(err)
	}

	// Initialize db schema
	if _, err := db.Exec(dbSchema); err != nil {
		fatal(err)
	}
	d.closeOnExit(db)

	return db
}

//go:embed schema.sql
var dbSchema string

func tcpPort(ln net.Listener) int {
	return ln.Addr().(*net.TCPAddr).Port
}

// detectSocketClose polls for the unix socket at socketPath to be removed
// or changed to a different underlying inode.
func detectSocketClose(ln *net.UnixListener, socketPath string) error {
	orig, err := xos.SocketStat(socketPath)
	if err != nil {
		return err
	}

	// When this function exits, the socket has been changed.
	// In that case, don't unlink the socket since it has already been changed.
	defer ln.SetUnlinkOnClose(false)

	// Sleep until the socket changes
	errs := 0
	for {
		time.Sleep(200 * time.Millisecond)
		fi, err := xos.SocketStat(socketPath)
		if os.IsNotExist(err) {
			// Socket was removed; don't remove it again
			return nil
		} else if err != nil {
			errs++
			if errs == 3 {
				return err
			}
			time.Sleep(1 * time.Second)
			continue
		}
		if !xos.SameSocket(orig, fi) {
			return nil
		}
	}
}

func (d *Daemon) closeOnExit(c io.Closer) {
	d.close = append(d.close, c)
}

func (d *Daemon) closeAll() {
	for _, c := range d.close {
		c.Close()
	}
}

type bailout struct {
	err error
}

func fatal(err error) {
	panic(bailout{err})
}

func fatalf(format string, args ...interface{}) {
	panic(bailout{fmt.Errorf(format, args...)})
}

func handleBailout(err *error) {
	if e := recover(); e != nil {
		if b, ok := e.(bailout); ok {
			*err = b.err
		} else {
			panic(e)
		}
	}
}

// redirectLogOutput redirects the global logger to also write to a file.
func redirectLogOutput() error {
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	logPath := filepath.Join(cache, "encore", "daemon.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	log.Info().Msgf("writing output to %s", logPath)
	log.Logger = log.Output(io.MultiWriter(zerolog.ConsoleWriter{Out: os.Stderr}, f))
	return nil
}

// retryingTCPListener is a TCP listener that attempts multiple times
// to listen on a given port. It is designed to handle race conditions
// between multiple daemon processes handing off to each other
// and the port still being in use momentarily.
type retryingTCPListener struct {
	component string
	port      int
	ctx       context.Context
	cancel    func() // call to cancel ctx

	// doneListening is closed when the underlying listener is open,
	// or it gave up due to an error.
	doneListening chan struct{}
	underlying    net.Listener
	listenErr     error
}

func listenLocalhostTCP(component string, port int) *retryingTCPListener {
	ctx, cancel := context.WithCancel(context.Background())
	ln := &retryingTCPListener{
		component:     component,
		port:          port,
		ctx:           ctx,
		cancel:        cancel,
		doneListening: make(chan struct{}),
	}
	go ln.listen()
	return ln
}

func (ln *retryingTCPListener) Accept() (net.Conn, error) {
	select {
	case <-ln.ctx.Done():
		return nil, net.ErrClosed
	case <-ln.doneListening:
		if ln.listenErr != nil {
			return nil, ln.listenErr
		}
		return ln.underlying.Accept()
	}
}

func (ln *retryingTCPListener) Close() error {
	ln.cancel()
	select {
	case <-ln.doneListening:
		if ln.listenErr == nil {
			return ln.underlying.Close()
		}
	default:
	}
	return nil
}

func (ln *retryingTCPListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IP{127, 0, 0, 1}, Port: ln.port}
}

func (ln *retryingTCPListener) Port() int {
	return ln.port
}

func (ln *retryingTCPListener) listen() {
	defer close(ln.doneListening)

	logger := log.With().Str("component", ln.component).Int("port", ln.port).Logger()
	addr := "127.0.0.1:" + strconv.Itoa(ln.port)

	retrySleep := 0 * time.Second
	ctx, cancel := context.WithTimeout(ln.ctx, 5*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			// We're being told to abort; ensure we always store
			// a non-nil listenErr.
			if ln.listenErr == nil {
				ln.listenErr = ctx.Err()
			}
			logger.Error().Err(ln.listenErr).Msg("unable to listen, giving up")
			return
		case <-time.After(retrySleep):
			ln.underlying, ln.listenErr = net.Listen("tcp", addr)
			retrySleep += 100 * time.Millisecond
			if ln.listenErr == nil {
				logger.Info().Msg("listening on port")
				return
			}
			logger.Error().Err(ln.listenErr).Msg("unable to listen, retrying")
		}
	}
}
