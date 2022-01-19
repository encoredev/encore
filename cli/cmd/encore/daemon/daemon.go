package daemon

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"encr.dev/cli/daemon"
	"encr.dev/cli/daemon/dash"
	"encr.dev/cli/daemon/run"
	"encr.dev/cli/daemon/runtime"
	"encr.dev/cli/daemon/runtime/trace"
	"encr.dev/cli/daemon/secret"
	"encr.dev/cli/daemon/sqldb"
	"encr.dev/cli/internal/xos"
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
	// exit receives signals from the different subsystems
	// that something went wrong and it's time to exit.
	// Sending nil indicates it's time to gracefully exit.
	exit := make(chan error)

	d := &Daemon{dev: dev, exit: exit}
	defer handleBailout(&err)
	defer d.closeAll()

	d.init()
	d.serve()

	return <-exit
}

// Daemon orchestrates setting up the different daemon subsystems.
type Daemon struct {
	Log     zerolog.Logger
	Daemon  *net.UnixListener
	Runtime *net.TCPListener
	DBProxy *net.TCPListener
	Dash    *net.TCPListener

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
	d.Dash = d.listenTCP(9400)
	d.Runtime = d.listenTCP(9401)
	d.DBProxy = d.listenTCP(9402)

	d.Trace = trace.NewStore()
	d.ClusterMgr = sqldb.NewClusterManager()
	d.Secret = secret.New()
	d.RunMgr = &run.Manager{
		RuntimePort: tcpPort(d.Runtime),
		DBProxyPort: tcpPort(d.DBProxy),
		DashPort:    tcpPort(d.Dash),
		Secret:      d.Secret,
	}
	d.DashSrv = dash.NewServer(d.RunMgr, d.Trace)
	d.Server = daemon.New(d.RunMgr, d.ClusterMgr, d.Secret)
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

func (d *Daemon) serveDaemon() {
	log.Info().Stringer("addr", d.Daemon.Addr()).Msg("serving daemon")
	srv := grpc.NewServer()
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
