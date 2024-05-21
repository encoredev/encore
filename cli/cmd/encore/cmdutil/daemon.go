package cmdutil

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"encr.dev/internal/version"
	"encr.dev/pkg/xos"
	daemonpb "encr.dev/proto/encore/daemon"
)

// DaemonOption is an option for connecting to the Encore daemon.
type DaemonOption func(*daemonOptions)

type daemonOptions struct {
	skipStart bool
}

var (
	// SkipStart skips starting the daemon if it is not already running.
	SkipStart DaemonOption = func(o *daemonOptions) {
		o.skipStart = true
	}
)

// ConnectDaemon returns a client connection to the Encore daemon.
// By default, it will start the daemon if it is not already running.
func ConnectDaemon(ctx context.Context, opts ...DaemonOption) daemonpb.DaemonClient {
	var options daemonOptions
	for _, o := range opts {
		o(&options)
	}
	socketPath, err := daemonSockPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal: ", err)
		os.Exit(1)
	}

	if _, err := xos.SocketStat(socketPath); err == nil {
		// The socket exists; check that it is responsive.
		if cc, err := dialDaemon(ctx, socketPath); err == nil {
			// Make sure the daemon is running an up-to-date version;
			// restart it otherwise.
			cl := daemonpb.NewDaemonClient(cc)
			if resp, err := cl.Version(ctx, &empty.Empty{}); err == nil {
				diff := version.Compare(resp.Version)
				switch {
				case diff < 0:
					// Daemon is running a newer version
					return cl
				case diff == 0:
					if configHash, err := version.ConfigHash(); err != nil {
						Fatal("unable to get config path: ", err)
					} else if configHash == resp.ConfigHash {
						return cl
					}
					// Daemon is running the same version but different config
					fmt.Fprintf(os.Stderr, "encore: restarting daemon due to configuration change.\n")
				case diff > 0:
					fmt.Fprintf(os.Stderr, "encore: daemon is running an outdated version (%s), restarting.\n", resp.Version)
				}
			}
		}
		// Remove the socket file which triggers the daemon to exit.
		_ = os.Remove(socketPath)
	}

	if options.skipStart {
		return nil
	}
	// Start the daemon.
	if err := StartDaemonInBackground(ctx); err != nil {
		Fatal("starting daemon: ", err)
	}
	cc, err := dialDaemon(ctx, socketPath)
	if err != nil {
		Fatal("dialing daemon: ", err)
	}
	return daemonpb.NewDaemonClient(cc)
}

func StopDaemon() {
	socketPath, err := daemonSockPath()
	if err != nil {
		Fatal("stopping daemon: ", err)
	}
	if _, err := xos.SocketStat(socketPath); err == nil {
		_ = os.Remove(socketPath)
	}
}

// daemonSockPath reports the path to the Encore daemon unix socket.
func daemonSockPath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("could not determine cache dir: %v", err)
	}
	return filepath.Join(cacheDir, "encore", "encored.sock"), nil
}

// StartDaemonInBackground starts the Encore daemon in the background.
func StartDaemonInBackground(ctx context.Context) error {
	socketPath, err := daemonSockPath()
	if err != nil {
		return err
	}

	// nosemgrep
	exe, err := os.Executable()
	if err != nil {
		exe, err = exec.LookPath("encore")
	}
	if err != nil {
		return fmt.Errorf("could not determine location of encore executable: %v", err)
	}
	// nosemgrep
	cmd := exec.Command(exe, "daemon", "-f")
	cmd.SysProcAttr = xos.CreateNewProcessGroup()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start encore daemon: %v", err)
	}

	// Wait for it to come up
	for i := 0; i < 50; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond)
		if _, err := xos.SocketStat(socketPath); err == nil {
			return nil
		}
	}
	return fmt.Errorf("timed out waiting for daemon to start")
}

func dialDaemon(ctx context.Context, socketPath string) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	dialer := func(ctx context.Context, addr string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
	}
	return grpc.DialContext(ctx, "",
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithUnaryInterceptor(errInterceptor),
		grpc.WithContextDialer(dialer))
}

func errInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	err := invoker(ctx, method, req, reply, cc, opts...)
	if err != nil {
		if st, ok := status.FromError(err); ok {
			if st.Code() == codes.Unauthenticated {
				Fatal("not logged in: run 'encore auth login' first")
			}
			for _, detail := range st.Details() {
				switch t := detail.(type) {
				case *errdetails.PreconditionFailure:
					for _, violation := range t.Violations {
						if violation.Type == "INVALID_REFRESH_TOKEN" {
							Fatal("OAuth refresh token was invalid. Please run `encore auth login` again.")
						}
					}
				}
			}
		}
	}
	return err
}
