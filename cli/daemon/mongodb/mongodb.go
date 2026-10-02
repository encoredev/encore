// Package mongodb runs a local MongoDB server in Docker for Encore apps
// that declare MongoDB databases.
package mongodb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog"
	"go4.org/syncutil"

	"encr.dev/cli/daemon/namespace"
	"encr.dev/pkg/idents"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

const (
	// Image is the MongoDB docker image to use.
	Image = "mongo:8.0"

	// ReplicaSet is the name of the single-node replica set the server runs as.
	// MongoDB transactions require a replica set.
	ReplicaSet = "rs0"

	containerPort = "27017"
	dataDir       = "/data/db"
)

// IsUsed reports whether the application uses MongoDB at all.
func IsUsed(md *meta.Data) bool {
	return len(md.MongoDatabases) > 0
}

// Server is a local MongoDB server running in a Docker container.
type Server struct {
	ns       *namespace.Namespace
	forTests bool
	log      zerolog.Logger

	startOnce syncutil.Once
	container string
	addr      string
}

// New returns a new Server for the given namespace.
// If forTests is true, the server keeps its data in memory only.
func New(ns *namespace.Namespace, forTests bool, log zerolog.Logger) *Server {
	return &Server{ns: ns, forTests: forTests, log: log}
}

// Start starts the server, creating the container if needed,
// and waits until it accepts writes.
func (s *Server) Start(ctx context.Context) error {
	return s.startOnce.Do(func() error {
		return s.start(ctx)
	})
}

func (s *Server) start(ctx context.Context) error {
	if err := CheckRequirements(ctx); err != nil {
		return err
	}

	if ok, err := imageExists(ctx); err != nil {
		return errors.Wrap(err, "check docker image")
	} else if !ok {
		s.log.Debug().Msg("MongoDB image does not exist, pulling")
		if err := pullImage(ctx); err != nil {
			return errors.Wrap(err, "pull docker image")
		}
	}

	s.container = containerName(s.ns, s.forTests)
	running, exists, err := containerStatus(ctx, s.container)
	if err != nil {
		return err
	}

	switch {
	case running:
		s.log.Debug().Str("container", s.container).Msg("MongoDB container already running")
	case exists:
		s.log.Debug().Str("container", s.container).Msg("MongoDB container stopped, restarting")
		if out, err := exec.CommandContext(ctx, "docker", "start", s.container).CombinedOutput(); err != nil {
			return errors.Wrapf(err, "could not start MongoDB container: %s", out)
		}
	default:
		s.log.Debug().Str("container", s.container).Msg("MongoDB container not found, creating")
		args := []string{"run", "-d", "-p", containerPort, "--name", s.container}
		if s.forTests {
			args = append(args, "--mount", "type=tmpfs,destination="+dataDir)
		} else {
			volume := volumeName(s.ns)
			if err := createVolumeIfNeeded(ctx, volume); err != nil {
				return err
			}
			args = append(args, "-v", volume+":"+dataDir)
		}
		args = append(args, Image, "--replSet", ReplicaSet, "--bind_ip_all")
		if out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput(); err != nil {
			return errors.Wrapf(err, "could not start MongoDB as docker container: %s", out)
		}
	}

	addr, err := waitForPort(ctx, s.container)
	if err != nil {
		return err
	}
	s.addr = addr

	if err := waitForReplicaSet(ctx, s.container); err != nil {
		return err
	}
	s.log.Debug().Str("container", s.container).Str("addr", s.addr).Msg("MongoDB is ready")
	return nil
}

// Addr returns the host:port the server listens on.
// It must only be called after Start has succeeded.
func (s *Server) Addr() string {
	return s.addr
}

// DropDatabases drops the given databases. It is used before a test run
// so that tests start from empty databases.
func (s *Server) DropDatabases(ctx context.Context, names []string) error {
	for _, name := range names {
		script := fmt.Sprintf("db.getSiblingDB(%q).dropDatabase()", name)
		if out, err := mongosh(ctx, s.container, script); err != nil {
			return errors.Wrapf(err, "drop database %s: %s", name, out)
		}
	}
	return nil
}

// Stop implements infra.Resource. The container is left running,
// like the PostgreSQL container, so data survives between runs
// and the next start is fast.
func (s *Server) Stop() {}

// CheckRequirements reports an error if Docker is not available.
func CheckRequirements(ctx context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.New("This application requires docker to run since it uses a MongoDB database. Install docker first.")
	} else if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
		return errors.New("The docker daemon is not running. Start it first.")
	}
	return nil
}

// containerName computes the container name for the given namespace.
func containerName(ns *namespace.Namespace, forTests bool) string {
	appID := ns.App.PlatformOrLocalID()
	base := "mongodb-" + appID
	if forTests {
		base += "-test"
	}
	nsName := idents.Convert(string(ns.Name), idents.KebabCase)
	return base + "-" + nsName + "-" + string(ns.ID)
}

// volumeName computes the docker volume name for the given namespace.
func volumeName(ns *namespace.Namespace) string {
	nsName := idents.Convert(string(ns.Name), idents.KebabCase)
	return fmt.Sprintf("mongodb-%s-%s-%s", ns.App.PlatformOrLocalID(), ns.ID, nsName)
}

func imageExists(ctx context.Context) (bool, error) {
	out, err := exec.CommandContext(ctx, "docker", "image", "inspect", Image).CombinedOutput()
	switch {
	case err == nil:
		return true, nil
	case bytes.Contains(out, []byte("No such image")):
		return false, nil
	// Podman has a different error message.
	case bytes.Contains(out, []byte("failed to find image")):
		return false, nil
	default:
		return false, errors.Wrapf(err, "docker image inspect failed: %s", Image)
	}
}

func pullImage(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "pull", Image)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func createVolumeIfNeeded(ctx context.Context, name string) error {
	if err := exec.CommandContext(ctx, "docker", "volume", "inspect", name).Run(); err == nil {
		return nil
	}
	out, err := exec.CommandContext(ctx, "docker", "volume", "create", name).CombinedOutput()
	return errors.Wrapf(err, "create volume %s: %s", name, out)
}

// containerStatus reports whether the container is running and whether it exists.
func containerStatus(ctx context.Context, name string) (running, exists bool, err error) {
	out, err := exec.CommandContext(ctx, "docker", "container", "inspect", name).CombinedOutput()
	if err != nil {
		// Docker and Podman report a missing container differently.
		if bytes.Contains(bytes.ToLower(out), []byte("no such container")) {
			return false, false, nil
		}
		return false, false, errors.Wrapf(err, "docker container inspect failed: %s", out)
	}
	var resp []struct {
		State struct{ Running bool }
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return false, false, errors.Wrap(err, "parse `docker container inspect` response")
	}
	if len(resp) == 0 {
		return false, false, nil
	}
	return resp[0].State.Running, true, nil
}

// waitForPort waits until the container's port is published, and returns it as host:port.
func waitForPort(ctx context.Context, name string) (string, error) {
	for i := 0; i < 20; i++ {
		out, err := exec.CommandContext(ctx, "docker", "container", "inspect", name).CombinedOutput()
		if err != nil {
			return "", errors.Wrapf(err, "docker container inspect failed: %s", out)
		}
		var resp []struct {
			NetworkSettings struct {
				Ports map[string][]struct {
					HostIP   string
					HostPort string
				}
			}
		}
		if err := json.Unmarshal(out, &resp); err != nil {
			return "", errors.Wrap(err, "parse `docker container inspect` response")
		}
		if len(resp) > 0 {
			if ports := resp[0].NetworkSettings.Ports[containerPort+"/tcp"]; len(ports) > 0 {
				hostIP := ports[0].HostIP
				// Podman can keep HostIP empty or 0.0.0.0.
				if hostIP == "" || hostIP == "0.0.0.0" {
					hostIP = "127.0.0.1"
				}
				return hostIP + ":" + ports[0].HostPort, nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return "", errors.New("timed out waiting for MongoDB container to start")
}

// waitForReplicaSet waits until mongod answers, initiates the single-node
// replica set if it isn't yet, and waits until the node accepts writes.
func waitForReplicaSet(ctx context.Context, name string) error {
	const script = `
try {
  rs.status();
} catch (e) {
  rs.initiate({_id: "` + ReplicaSet + `", members: [{_id: 0, host: "localhost:` + containerPort + `"}]});
}
print(db.hello().isWritablePrimary);
`
	var lastOut []byte
	for i := 0; i < 120; i++ {
		out, err := mongosh(ctx, name, script)
		if err == nil && strings.TrimSpace(string(lastLine(out))) == "true" {
			return nil
		}
		lastOut = out
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return errors.Newf("MongoDB did not become ready: %s", lastOut)
}

// mongosh runs a script inside the container with the MongoDB shell.
func mongosh(ctx context.Context, name, script string) ([]byte, error) {
	return exec.CommandContext(ctx, "docker", "exec", name, "mongosh", "--quiet", "--eval", script).CombinedOutput()
}

func lastLine(b []byte) []byte {
	b = bytes.TrimSpace(b)
	if i := bytes.LastIndexByte(b, '\n'); i >= 0 {
		return b[i+1:]
	}
	return b
}
