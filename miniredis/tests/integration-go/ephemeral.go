package main

// Start a miniredis-rs server on a random port.

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
)

const executable = "miniredis-rs-server"

type ephemeral exec.Cmd

// Redis starts a miniredis-rs on a random port. Will panic if that
// doesn't work.
// Returns something which you'll have to Close(), and a string to give to Dial()
func Redis() (*ephemeral, string) {
	return runRedis("")
}

// RedisAuth starts a miniredis-rs on a random port with authentication enabled.
func RedisAuth(passwd string) (*ephemeral, string) {
	return runRedis(fmt.Sprintf("requirepass %s", passwd))
}

// RedisUserAuth starts a miniredis-rs on a random port with ACL rules enabled.
func RedisUserAuth(users map[string]string) (*ephemeral, string) {
	acls := "user default on -@all +hello\n"
	for user, pass := range users {
		acls += fmt.Sprintf("user %s on +@all ~* >%s\n", user, pass)
	}
	return runRedis(acls)
}

// RedisCluster starts a miniredis-rs on a random port in cluster mode.
func RedisCluster() (*ephemeral, string) {
	return runRedis("cluster-enabled yes\ncluster-config-file nodes.conf")
}

func RedisTLS() (*ephemeral, string) {
	return runRedis(`
		tls-port 0
		tls-cert-file ../../testdata/server.crt
		tls-key-file ../../testdata/server.key
		tls-ca-cert-file ../../testdata/ca.crt
	`)
}

func runRedis(extraConfig string) (*ephemeral, string) {
	c := exec.Command(executable, "-")
	stdin, err := c.StdinPipe()
	if err != nil {
		panic(err)
	}
	stdout, err := c.StdoutPipe()
	if err != nil {
		panic(err)
	}
	c.Stderr = nil // inherit stderr for debugging

	if err := c.Start(); err != nil {
		panic(fmt.Sprintf("starting %s: %s", executable, err))
	}

	// Send config and close stdin so the server knows config is complete.
	fmt.Fprintf(stdin, "port 0\nbind 127.0.0.1\nappendonly no\n%s", extraConfig)
	stdin.Close()

	// Read the PORT=<n> readiness line from stdout.
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		c.Process.Kill()
		c.Wait()
		panic("miniredis-rs-server: no readiness line on stdout")
	}
	line := scanner.Text()
	if !strings.HasPrefix(line, "PORT=") {
		c.Process.Kill()
		c.Wait()
		panic(fmt.Sprintf("miniredis-rs-server: unexpected output: %q", line))
	}
	port := strings.TrimPrefix(line, "PORT=")
	addr := fmt.Sprintf("127.0.0.1:%s", port)

	e := ephemeral(*c)
	return &e, addr
}

func (e *ephemeral) Close() {
	((*exec.Cmd)(e)).Process.Kill()
	((*exec.Cmd)(e)).Wait()
}
