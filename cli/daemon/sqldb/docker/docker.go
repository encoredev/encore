package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"

	"encr.dev/cli/daemon/namespace"
	"encr.dev/cli/daemon/sqldb"
	"encr.dev/pkg/idents"
)

type Driver struct{}

var _ sqldb.Driver = (*Driver)(nil)

const (
	DefaultSuperuserUsername = "postgres"
	DefaultSuperuserPassword = "postgres"
	DefaultRootDatabase      = "postgres"
	defaultDataDir           = "/var/lib/postgresql/data"
)

func (d *Driver) CreateCluster(ctx context.Context, p *sqldb.CreateParams, log zerolog.Logger) (status *sqldb.ClusterStatus, err error) {
	// Ensure the docker image exists first.
	{
		checkExistsCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if ok, err := ImageExists(checkExistsCtx); err != nil {
			return nil, errors.Wrap(err, "check docker image")
		} else if !ok {
			log.Debug().Msg("PostgreSQL image does not exist, pulling")
			pullOp := p.Tracker.Add("Pulling PostgreSQL docker image", time.Now())
			if err := PullImage(context.Background()); err != nil {
				log.Error().Err(err).Msg("failed to pull PostgreSQL image")
				p.Tracker.Fail(pullOp, err)
				return nil, errors.Wrap(err, "pull docker image")
			} else {
				p.Tracker.Done(pullOp, 0)
				log.Info().Msg("successfully pulled sqldb image")
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// If we return with a connection, wait until we can connect.
	defer func() {
		if err != nil {
			return
		}
		// Wait for the database to come up; this might take a little bit
		// when we're racing with spinning up a Docker container.
		uri := status.ConnURI(status.Config.RootDatabase, status.Config.Superuser)

		const sleepTime = 250 * time.Millisecond
		const maxLoops = (30 * time.Second) / sleepTime
		for i := 0; i < int(maxLoops); i++ {
			var conn *pgx.Conn
			connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			conn, err = pgx.Connect(connCtx, uri)
			cancel()

			if err == nil {
				_ = conn.Close(ctx)
				return
			} else if ctx.Err() != nil {
				// We'll never succeed once the context has been canceled.
				// Give up straight away.
				log.Debug().Err(err).Msgf("failed to connect to db")
				err = errors.Wrap(err, "database did not come up")
			} else if errors.Is(err, io.ErrUnexpectedEOF) {
				// This is a transient error that can happen when the database first initialises
				err = errors.Wrap(err, "database is not ready yet")
			} else {
				err = errors.WithStack(err)
			}
			time.Sleep(250 * time.Millisecond)
		}
	}()

	cid := p.ClusterID
	cnames := containerNames(cid)
	status, existingContainerName, err := d.clusterStatus(ctx, cid)
	if err != nil {
		log.Error().Err(err).Msg("failed to get container status")
		return nil, errors.WithStack(err)
	}

	// waitForPort waits for the port to become available before returning.
	waitForPort := func() (*sqldb.ClusterStatus, error) {
		for i := 0; i < 20; i++ {
			status, err = d.ClusterStatus(ctx, cid)
			if err != nil {
				return nil, errors.Wrap(err, "unable to wait for port")
			}
			if status.Config.Host != "" {
				log.Debug().Str("hostport", status.Config.Host).Msg("cluster started")
				return status, nil
			}
			time.Sleep(500 * time.Millisecond)
		}
		return nil, errors.New("timed out waiting for cluster to start")
	}

	switch status.Status {
	case sqldb.Running:
		log.Debug().Str("hostport", status.Config.Host).Msg("cluster already running")
		return status, nil

	case sqldb.Stopped:
		log.Debug().Msg("cluster stopped, restarting")

		if out, err := exec.CommandContext(ctx, "docker", "start", existingContainerName).CombinedOutput(); err != nil {
			return nil, errors.Wrapf(err, "could not start sqldb container: %s", string(out))
		}
		return waitForPort()

	case sqldb.NotFound:
		log.Debug().Msg("cluster not found, creating")
		args := []string{
			"run",
			"-d",
			"-p", "5432",
			"--shm-size=1gb",
			"-e", "POSTGRES_USER=" + DefaultSuperuserUsername,
			"-e", "POSTGRES_PASSWORD=" + DefaultSuperuserPassword,
			"-e", "POSTGRES_DB=" + DefaultRootDatabase,
			"-e", "PGDATA=" + defaultDataDir,
			"--name", cnames[0],
		}
		if p.Memfs {
			args = append(args,
				"--mount", "type=tmpfs,destination="+defaultDataDir,
				Image,
				"-c", "fsync=off",
			)
		} else {
			volumeName := clusterVolumeNames(p.ClusterID.NS)[0] // guaranteed to be non-empty
			if err := d.createVolumeIfNeeded(ctx, volumeName); err != nil {
				return nil, errors.Wrap(err, "create data volume")
			}
			args = append(args,
				"-v", fmt.Sprintf("%s:%s", volumeName, defaultDataDir),
				Image)
		}

		cmd := exec.CommandContext(ctx, "docker", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, errors.Wrapf(err, "could not start sql database as docker container: %s", out)
		}

		log.Debug().Msg("cluster created")
		return waitForPort()

	default:
		return nil, errors.Newf("unknown cluster status %q", status.Status)
	}
}

func (d *Driver) ClusterStatus(ctx context.Context, id sqldb.ClusterID) (*sqldb.ClusterStatus, error) {
	status, _, err := d.clusterStatus(ctx, id)
	return status, errors.WithStack(err)
}

func (d *Driver) CheckRequirements(ctx context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.New("This application requires docker to run since it uses an SQL database. Install docker first.")
	} else if !isDockerRunning(ctx) {
		return errors.New("The docker daemon is not running. Start it first.")
	}
	return nil
}

// clusterStatus reports both the standard ClusterStatus but also the container name we actually resolved to.
func (d *Driver) clusterStatus(ctx context.Context, id sqldb.ClusterID) (status *sqldb.ClusterStatus, containerName string, err error) {
	var output []byte

	// Try the candidate container names in order.
	cnames := containerNames(id)
	for _, cname := range cnames {
		var err error
		out, err := exec.CommandContext(ctx, "docker", "container", "inspect", cname).CombinedOutput()
		if errors.Is(err, exec.ErrNotFound) {
			return nil, "", errors.New("docker not found: is it installed and in your PATH?")
		} else if err != nil {
			// Docker returns a non-zero exit code if the container does not exist.
			// Try to tell this apart from an error by parsing the output.
			if bytes.Contains(out, []byte("No such container")) {
				continue
			}
			// Podman has slightly different output when a container is not found.
			if bytes.Contains(out, []byte("no such container")) {
				continue
			}
			return nil, "", errors.Wrapf(err, "docker container inspect failed: %s", out)
		} else {
			// Found our container; use it.
			output, containerName = out, cname
			break
		}
	}
	if output == nil {
		return &sqldb.ClusterStatus{Status: sqldb.NotFound}, containerName, nil
	}

	var resp []struct {
		Name  string
		State struct {
			Running bool
		}
		Config struct {
			Env []string
		}
		NetworkSettings struct {
			Ports map[string][]struct {
				HostIP   string
				HostPort string
			}
		}
	}
	if err := json.Unmarshal(output, &resp); err != nil {
		return nil, "", errors.Wrap(err, "parse `docker container inspect` response")
	}
	for _, c := range resp {
		// Docker prefixes `/` to the container name, Podman doesn't.
		if c.Name == "/"+containerName || c.Name == containerName {
			status := &sqldb.ClusterStatus{Status: sqldb.Stopped, Config: &sqldb.ConnConfig{
				// Defaults if we don't find anything else configured.
				Superuser: sqldb.Role{
					Type:     sqldb.RoleSuperuser,
					Username: DefaultSuperuserUsername,
					Password: DefaultSuperuserPassword,
				},
				RootDatabase: DefaultRootDatabase,
			}}
			if c.State.Running {
				status.Status = sqldb.Running
			}
			ports := c.NetworkSettings.Ports["5432/tcp"]
			if len(ports) > 0 {
				hostIP := ports[0].HostIP

				// Podman can keep HostIP empty or 0.0.0.0.
				// https://github.com/containers/podman/issues/17780
				if hostIP == "" || hostIP == "0.0.0.0" {
					hostIP = "127.0.0.1"
				}

				status.Config.Host = hostIP + ":" + ports[0].HostPort
			}

			// Read the Postgres config from the docker container's environment.
			for _, env := range c.Config.Env {
				if name, value, ok := strings.Cut(env, "="); ok {
					switch name {
					case "POSTGRES_USER":
						status.Config.Superuser.Username = value
					case "POSTGRES_PASSWORD":
						status.Config.Superuser.Password = value
					case "POSTGRES_DB":
						status.Config.RootDatabase = value
					}
				}
			}

			return status, containerName, nil
		}
	}
	return &sqldb.ClusterStatus{Status: sqldb.NotFound}, containerName, nil
}

func (d *Driver) CanDestroyCluster(ctx context.Context, id sqldb.ClusterID) error {
	// Check that we can communicate with Docker.
	if !isDockerRunning(ctx) {
		return errors.New("cannot delete sql database: docker is not running")
	}
	return nil
}

func (d *Driver) DestroyCluster(ctx context.Context, id sqldb.ClusterID) error {
	cnames := containerNames(id)
	for _, cname := range cnames {
		out, err := exec.CommandContext(ctx, "docker", "rm", "-f", cname).CombinedOutput()
		if err != nil {
			if bytes.Contains(out, []byte("No such container")) {
				continue
			}
			return errors.Wrapf(err, "could not delete cluster: %s", out)
		}
	}
	return nil
}

func (d *Driver) DestroyNamespaceData(ctx context.Context, ns *namespace.Namespace) error {
	candidates := clusterVolumeNames(ns)
	for _, c := range candidates {
		if err := exec.CommandContext(ctx, "docker", "volume", "rm", "-f", c).Run(); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "no such volume") {
				continue
			}
			return errors.Wrapf(err, "could not delete volume %s", c)
		}
	}
	return nil
}

func (d *Driver) createVolumeIfNeeded(ctx context.Context, name string) error {
	if err := exec.CommandContext(ctx, "docker", "volume", "inspect", name).Run(); err == nil {
		return nil
	}
	out, err := exec.CommandContext(ctx, "docker", "volume", "create", name).CombinedOutput()
	return errors.Wrapf(err, "create volume %s: %s", name, out)
}

func (d *Driver) Meta() sqldb.DriverMeta {
	return sqldb.DriverMeta{ClusterIsolation: true}
}

// containerName computes the container name candidates for a given clusterID.
func containerNames(id sqldb.ClusterID) []string {
	// candidates returns possible candidate names for a given app id.
	candidates := func(appID string) (names []string) {
		base := "sqldb-" + appID

		if id.Type != sqldb.Run {
			base += "-" + string(id.Type)
		}

		// Convert the namespace to kebab case to remove invalid characters like ':'.
		nsName := idents.Convert(string(id.NS.Name), idents.KebabCase)

		names = []string{base + "-" + nsName + "-" + string(id.NS.ID)}
		// If this is the default namespace look up the container without
		// the namespace suffix as well, for backwards compatibility.
		if id.NS.Name == "default" {
			names = append(names, base)
		}
		return names
	}

	var names []string
	if pid := id.NS.App.PlatformID(); pid != "" {
		names = append(names, candidates(pid)...)
	}
	names = append(names, candidates(id.NS.App.LocalID())...)
	return names
}

// ImageExists reports whether the docker image exists.
func ImageExists(ctx context.Context) (ok bool, err error) {
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
		return false, errors.WithStack(errors.Wrapf(err, "docker image inspect failed: %s", Image))
	}
}

// PullImage pulls the image.
func PullImage(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "pull", Image)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

const Image = "encoredotdev/postgres:15"

func isDockerRunning(ctx context.Context) bool {
	err := exec.CommandContext(ctx, "docker", "info").Run()
	return err == nil
}

// clusterVolumeName reports the candidate names for the docker volume.
func clusterVolumeNames(ns *namespace.Namespace) (candidates []string) {
	nsName := idents.Convert(string(ns.Name), idents.KebabCase)
	suffix := fmt.Sprintf("%s-%s", ns.ID, nsName)

	for _, id := range [...]string{ns.App.PlatformID(), ns.App.LocalID()} {
		if id != "" {
			candidates = append(candidates, fmt.Sprintf("sqldb-%s-%s", id, suffix))
		}
	}
	return candidates
}
