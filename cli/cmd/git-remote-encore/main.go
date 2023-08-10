// Command git-remote-encore provides a gitremote helper for
// interacting with Encore's git hosting without SSH keys,
// by piggybacking on Encore's auth tokens.
package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"encr.dev/internal/conf"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
		os.Exit(1)
	}
}

var isLocalTest = (func() bool {
	return filepath.Base(os.Args[0]) == "git-remote-encorelocal"
})()

// remoteScheme is the remote scheme we expect.
// It's "encore" in general but "encorelocal" for local development.
var remoteScheme = (func() string {
	if isLocalTest {
		return "encorelocal"
	} else {
		return "encore"
	}
})()

func run(args []string) error {
	stdin := bufio.NewReader(os.Stdin)
	stdout := os.Stdout

	// Read commands from stdin.
	for {
		cmd, err := stdin.ReadString('\n')
		if err != nil {
			return fmt.Errorf("unexpected error reading stdin: %v", err)
		}
		cmd = cmd[:len(cmd)-1] // skip trailing newline
		switch {
		case cmd == "capabilities":
			if _, err := stdout.Write([]byte("*connect\n\n")); err != nil {
				return err
			}
		case strings.HasPrefix(cmd, "connect "):
			service := cmd[len("connect "):]
			return connect(args, service)
		default:
			return fmt.Errorf("unsupported command: %s", cmd)
		}
	}
}

// connect implements the "connect" capability by copying data
// to and from the remote end over gRPC.
func connect(args []string, svc string) error {
	uri, err := url.Parse(args[2])
	if err != nil {
		return fmt.Errorf("connect %s: invalid remote uri: %v", os.Args[2], err)
	} else if uri.Scheme != remoteScheme {
		return fmt.Errorf("connect %s: expected remote scheme %q, got %q", os.Args[2], remoteScheme, uri.Scheme)
	}
	appID := uri.Hostname()

	ts := conf.NewTokenSource()
	tok, err := ts.Token()
	if err != nil {
		return fmt.Errorf("could not get Encore auth token: %v", err)
	}

	f, err := os.CreateTemp("", "encore-token-auth-sentinel-key")
	if err != nil {
		return err
	}
	keyPath := f.Name()
	defer os.Remove(keyPath)
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	} else if _, err := f.Write([]byte(SentinelPrivateKey)); err != nil {
		f.Close()
		return err
	} else if err := f.Close(); err != nil {
		return err
	}

	// Create a dummy config file so that we can work around any host overrides
	// present on the system.
	cfg, err := os.CreateTemp("", "encore-dummy-ssh-config")
	if err != nil {
		return err
	}
	cfgPath := cfg.Name()
	defer os.Remove(cfgPath)

	// Communicate to Git that the connection is established.
	os.Stdout.Write([]byte("\n"))

	sshServer, port := "git.encore.dev", "22"
	if isLocalTest {
		sshServer, port = "localhost", "9040"
	}

	// Set up an SSH tunnel with a sentinel key as a way to signal
	// Encore to use token-based authentication, and pass the token
	// as part of the command.
	cmd := exec.Command("ssh",
		"-x", "-T",
		"-F", cfgPath,
		"-o", "IdentitiesOnly=yes",
		"-i", keyPath,
		"-p", port,
		sshServer,
		fmt.Sprintf("token=%s %s '%s'", tok.AccessToken, svc, appID))
	cmd.Env = []string{}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SentinelPrivateKey is a sentinel private key that Encore recognizes as
// the key that communicates that the user wishes to do token-based authentication
// instead of key-based authentication.
//
// NOTE: This is not a security problem. The key is meant to be public
// and does not serve as a means of authentication.
const SentinelPrivateKey = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACCyj3F5Tp1eBIp7rMohszumYzlys/BFfmX/LVkXJS8magAAAJjsp3yz7Kd8
swAAAAtzc2gtZWQyNTUxOQAAACCyj3F5Tp1eBIp7rMohszumYzlys/BFfmX/LVkXJS8mag
AAAEDMiwRrf5WET2mTKjKjX7z6vox3n6hKGKbP7V4MDtVre7KPcXlOnV4EinusyiGzO6Zj
OXKz8EV+Zf8tWRclLyZqAAAAE2VuY29yZS1zZW50aW5lbC1rZXkBAg==
-----END OPENSSH PRIVATE KEY-----
`
