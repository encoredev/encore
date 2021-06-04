package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	daemonpkg "encr.dev/cli/cmd/encore/daemon"
	"encr.dev/cli/internal/version"
	"encr.dev/cli/internal/xos"
	daemonpb "encr.dev/proto/encore/daemon"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
	"google.golang.org/grpc"
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
				if semver.Compare(version.Version, resp.Version) <= 0 {
					return cl
				}
				fmt.Fprintf(os.Stderr, "encore: daemon is running an outdated version (%s), restarting.\n", resp.Version)
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

func dialDaemon(ctx context.Context, socketPath string) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	dialer := func(ctx context.Context, addr string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
	}
	return grpc.DialContext(ctx, "",
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithContextDialer(dialer))
}
