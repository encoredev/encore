package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/rs/zerolog"

	"encr.dev/cli/daemon/sqldb"
)

type Driver struct{}

var _ sqldb.Driver = (*Driver)(nil)

const (
	DefaultSuperuserUsername = "postgres"
	DefaultSuperuserPassword = "postgres"
	DefaultRootDatabase      = "postgres"
)

func (d *Driver) CreateCluster(ctx context.Context, p *sqldb.CreateParams, log zerolog.Logger) (status *sqldb.ClusterStatus, err error) {
	// Ensure the docker image exists first.
	{
		checkExistsCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if ok, err := ImageExists(checkExistsCtx); err != nil {
			return nil, fmt.Errorf("check docker image: %v", err)
		} else if !ok {
			log.Debug().Msg("PostgreSQL image does not exist, pulling")
			if err := PullImage(context.Background()); err != nil {
				log.Error().Err(err).Msg("failed to pull PostgreSQL image")
				return nil, fmt.Errorf("pull docker image: %v", err)
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
		for i := 0; i < 40; i++ {
			var conn *pgx.Conn
			conn, err = pgx.Connect(ctx, uri)
			if err == nil {
				conn.Close(ctx)
				return
			} else if ctx.Err() != nil {
				// We'll never succeed once the context has been canceled.
				// Give up straight away.
				log.Debug().Err(err).Msgf("failed to connect to db")
				err = fmt.Errorf("database did not come up: %v", err)
			}
			time.Sleep(250 * time.Millisecond)
		}
	}()

	cid := p.ClusterID
	cnames := containerNames(cid)
	status, existingContainerName, err := d.clusterStatus(ctx, cid)
	if err != nil {
		log.Error().Err(err).Msg("failed to get container status")
		return nil, err
	}

	// waitForPort waits for the port to become available before returning.
	waitForPort := func() (*sqldb.ClusterStatus, error) {
		for i := 0; i < 20; i++ {
			status, err = d.ClusterStatus(ctx, cid)
			if err != nil {
				return nil, err
			}
			if status.Config.Host != "" {
				log.Debug().Str("hostport", status.Config.Host).Msg("cluster started")
				return status, nil
			}
			time.Sleep(500 * time.Millisecond)
		}
		return nil, fmt.Errorf("timed out waiting for cluster to start")
	}

	switch status.Status {
	case sqldb.Running:
		log.Debug().Str("hostport", status.Config.Host).Msg("cluster already running")
		return status, nil

	case sqldb.Stopped:
		log.Debug().Msg("cluster stopped, restarting")

		if out, err := exec.CommandContext(ctx, "docker", "start", existingContainerName).CombinedOutput(); err != nil {
			return nil, fmt.Errorf("could not start sqldb container: %s (%v)", string(out), err)
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
			"--name", cnames[0],
		}
		if p.Memfs {
			args = append(args,
				"--mount", "type=tmpfs,destination=/var/lib/postgresql/data",
				Image,
				"-c", "fsync=off",
			)
		} else {
			args = append(args, Image)
		}

		cmd := exec.CommandContext(ctx, "docker", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("could not start sql database as docker container: %s: %v", out, err)
		}

		log.Debug().Msg("cluster created")
		return waitForPort()

	default:
		return nil, fmt.Errorf("unknown cluster status %q", status.Status)
	}
}

func (d *Driver) ClusterStatus(ctx context.Context, id sqldb.ClusterID) (*sqldb.ClusterStatus, error) {
	status, _, err := d.clusterStatus(ctx, id)
	return status, err
}

// clusterStatus reports both the standard ClusterStatus but also the container name we actually resolved to.
func (d *Driver) clusterStatus(ctx context.Context, id sqldb.ClusterID) (status *sqldb.ClusterStatus, containerName string, err error) {
	var output []byte

	// Try the candidate container names in order.
	cnames := containerNames(id)
	for _, cname := range cnames {
		var err error
		out, err := exec.CommandContext(ctx, "docker", "container", "inspect", cname).CombinedOutput()
		if err == exec.ErrNotFound {
			return nil, "", errors.New("docker not found: is it installed and in your PATH?")
		} else if err != nil {
			// Docker returns a non-zero exit code if the container does not exist.
			// Try to tell this apart from an error by parsing the output.
			if bytes.Contains(out, []byte("No such container")) {
				continue
			}
			return nil, "", fmt.Errorf("docker container inspect failed: %s (%v)", out, err)
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
		return nil, "", fmt.Errorf("parse `docker container inspect` response: %v", err)
	}
	for _, c := range resp {
		if c.Name == "/"+containerName {
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
				status.Config.Host = ports[0].HostIP + ":" + ports[0].HostPort
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

func (d *Driver) DestroyCluster(ctx context.Context, id sqldb.ClusterID) error {
	cnames := containerNames(id)
	for _, cname := range cnames {
		out, err := exec.CommandContext(ctx, "docker", "rm", "-f", cname).CombinedOutput()
		if err != nil {
			if bytes.Contains(out, []byte("No such container")) {
				continue
			}
			return fmt.Errorf("could not delete cluster: %s (%v)", out, err)
		}
	}
	return nil
}

// containerName computes the container name candidates for a given clusterID.
func containerNames(id sqldb.ClusterID) []string {
	var names []string
	if pid := id.App.PlatformID(); pid != "" {
		names = append(names, pid)
	}
	names = append(names, id.App.LocalID())

	// Format the names into container names
	for i, name := range names {
		name = "sqldb-" + name
		if id.Type != sqldb.Run {
			name += "-" + string(id.Type)
		}
		names[i] = name
	}
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
	default:
		return false, err
	}
}

// PullImage pulls the image.
func PullImage(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "pull", Image)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

const Image = "postgres:14-alpine"
