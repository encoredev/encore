package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	daemonpkg "encr.dev/cli/cmd/encore/daemon"
	"encr.dev/cli/internal/xos"
	"encr.dev/internal/env"
	"encr.dev/internal/version"
	daemonpb "encr.dev/proto/encore/daemon"
)

var daemonizeForeground bool

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Starts the encore daemon",
	Run: func(cc *cobra.Command, args []string) {
		if daemonizeForeground {
			daemonpkg.Main()
		} else {
			if err := daemonize(context.Background()); err != nil {
				fatal(err)
			}
			fmt.Fprintln(os.Stdout, "encore daemon is now running")
		}
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.Flags().BoolVarP(&daemonizeForeground, "foreground", "f", false, "Start the daemon in the foreground")
	daemonCmd.AddCommand(daemonEnvCmd)
}

// daemonize starts the Encore daemon in the background.
func daemonize(ctx context.Context) error {
	socketPath, err := daemonSockPath()
	if err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		exe, err = exec.LookPath("encore")
	}
	if err != nil {
		return fmt.Errorf("could not determine location of encore executable: %v", err)
	}
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

// daemonSockPath reports the path to the Encore daemon unix socket.
func daemonSockPath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("could not determine cache dir: %v", err)
	}
	return filepath.Join(cacheDir, "encore", "encored.sock"), nil
}

// setupDaemon sets up the Encore daemon if it isn't already running
// and returns a client connected to it.
func setupDaemon(ctx context.Context) daemonpb.DaemonClient {
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
				diff := semver.Compare(version.Version, resp.Version)
				switch {
				case diff < 0:
					// Daemon is running a newer version
					return cl
				case diff == 0:
					if configHash, err := version.ConfigHash(); err != nil {
						fatal("unable to get config path: ", err)
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
		os.Remove(socketPath)
	}

	// Start the daemon.
	if err := daemonize(ctx); err != nil {
		fatal("starting daemon: ", err)
	}
	cc, err := dialDaemon(ctx, socketPath)
	if err != nil {
		fatal("dialing daemon: ", err)
	}
	return daemonpb.NewDaemonClient(cc)
}

func stopDaemon() {
	socketPath, err := daemonSockPath()
	if err != nil {
		fatal("stopping daemon: ", err)
	}
	if _, err := xos.SocketStat(socketPath); err == nil {
		_ = os.Remove(socketPath)
	}
}

func ErrInterceptorfunc(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	err := invoker(ctx, method, req, reply, cc, opts...)
	if err != nil {
		if st, ok := status.FromError(err); ok {
			if st.Code() == codes.Unauthenticated {
				fatal("not logged in: run 'encore auth login' first")
			}
			for _, detail := range st.Details() {
				switch t := detail.(type) {
				case *errdetails.PreconditionFailure:
					for _, violation := range t.Violations {
						if violation.Type == "INVALID_REFRESH_TOKEN" {
							fatal("OAuth Refresh Token was invalid. Please run `encore auth login` again.")
						}
					}
				}
			}
		}
	}
	return err
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
		grpc.WithUnaryInterceptor(ErrInterceptorfunc),
		grpc.WithContextDialer(dialer))
}

var daemonEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Prints Encore environment information",
	Run: func(cc *cobra.Command, args []string) {
		envs := env.List()
		for _, e := range envs {
			fmt.Println(e)
		}
	},
}
