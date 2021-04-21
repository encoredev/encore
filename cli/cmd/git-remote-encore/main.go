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
	"strings"

	"encr.dev/cli/internal/conf"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
		os.Exit(1)
	}
}

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
	} else if uri.Scheme != "encore" {
		return fmt.Errorf("connect %s: expected remote scheme \"encore\", got %q", os.Args[2], uri.Scheme)
	}
	appID := uri.Hostname()

	ts := &conf.TokenSource{}
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

	// Set up an SSH tunnel with a sentinel key as a way to signal
	// Encore to use token-based authentication, and pass the token
	// as part of the command.
	cmd := exec.Command("ssh", "-x", "-T", "-F", cfgPath, "-o", "IdentitiesOnly=yes", "-i", keyPath,
		"git.encore.dev", fmt.Sprintf("token=%s %s '%s'", tok.AccessToken, svc, appID))
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
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAlwAAAAdzc2gtcn
NhAAAAAwEAAQAAAIEA1ZrV6bnLgKI7cZHGn3Z93jTATaGjw6ytPdSorrnwYRP3K833BC19
ANPSWAoXcYXNDIR90j/V+sd5ILv5NUoctdV1+2J8jzW/hedj0HuDou1YruNHVowfE3JFYr
6eMK15kvc/K9EsIl/TfH9/RiWVnWq1wHwOdZtH2UZE9QdT+r0AAAIIrcJlP63CZT8AAAAH
c3NoLXJzYQAAAIEA1ZrV6bnLgKI7cZHGn3Z93jTATaGjw6ytPdSorrnwYRP3K833BC19AN
PSWAoXcYXNDIR90j/V+sd5ILv5NUoctdV1+2J8jzW/hedj0HuDou1YruNHVowfE3JFYr6e
MK15kvc/K9EsIl/TfH9/RiWVnWq1wHwOdZtH2UZE9QdT+r0AAAADAQABAAAAgBndpgmndf
0dqBUYkfS9ZICD4sWDzVDkmBXkqoh9+53FzSiAyGi5GWoAPHhswGn+ydW6NYJAOKklfoV4
PbU2REOHwXYblAZmDmPksSN1IbjDdFZ+0vXFUmS2k30eqIgIEGOrN1tnLXoK+B4kwFQ1IN
UMMpB39vRyhyrEGv+S4gQBAAAAQFiOrnRAtY50ZiqXND3SdCnQxnjmUxcE7pcQaaQK6KMP
A7bQpMNzJop/UpNRIjLb5bPG9FPgTzQ5+5l4fGL5OwYAAABBAP4V8q7KQLqoPsHaWG7pga
iE9cUzE9hle2zXiRCcXt2qXxB7P1U9DQVdzVwarfAggIGRsqjJmEDe69F/I4QAkj8AAABB
ANc20AXzRmnneRyZuOEUhTsdNWcQf9qv+tQh3DDr7SW7NhuSKW9CqC18nbDckEp0yOCjIR
k5HAPXd2pDop0UvAMAAAAPZWFuZHJlQG0xLmxvY2FsAQIDBA==
-----END OPENSSH PRIVATE KEY-----
`
