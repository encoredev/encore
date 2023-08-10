package sqldb

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgproto3/v2"
	"github.com/rs/zerolog/log"

	"encr.dev/cli/daemon/namespace"
	"encr.dev/pkg/pgproxy"
)

// ServeProxy serves the database proxy using the given listener.
func (cm *ClusterManager) ServeProxy(ln net.Listener) error {
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		conn, e := ln.Accept()
		if e != nil {
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Error().Err(e).Msgf("dbproxy: accept error, retrying in %v", tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return fmt.Errorf("dbproxy: could not accept: %v", e)
		}
		tempDelay = 0
		go func() {
			if err := cm.ProxyConn(conn, true); err != nil && err != context.Canceled {
				log.Error().Err(err).Msg("dbproxy: proxy error")
			}
		}()
	}
}

// ProxyConn authenticates and proxies a conn to the appropriate
// database cluster and database.
// If waitForSetup is true, it will wait for initial setup to complete
// before proxying the connection.
func (cm *ClusterManager) ProxyConn(client net.Conn, waitForSetup bool) error {
	defer client.Close()
	cl, err := pgproxy.SetupClient(client, &pgproxy.ClientConfig{
		TLS:          nil,
		WantPassword: true,
	})
	if err != nil {
		return err
	}

	if cancel, ok := cl.Hello.(*pgproxy.CancelData); ok {
		cm.cancelRequest(client, cancel)
		return nil
	}
	startup := cl.Hello.(*pgproxy.StartupData)

	// If the username is "encore" we're connecting to a database cluster
	// which may not be local
	var cluster *Cluster
	if startup.Username == "encore" {
		password := startup.Password
		found, ok := cm.LookupPassword(password)
		if !ok {
			cm.log.Error().Msg("dbproxy: could not find cluster")
			_ = cl.Backend.Send(&pgproto3.ErrorResponse{
				Severity: "FATAL",
				Code:     "08006",
				Message:  "database cluster not found or invalid connection string",
			})
			return nil
		}
		cluster = found
	} else {
		// The username is the app slug we want to connect to
		app, err := cm.apps.FindLatestByPlatformOrLocalID(startup.Username)
		if err != nil {
			cm.log.Error().Err(err).Msg("dbproxy: could not find app")
			_ = cl.Backend.Send(&pgproto3.ErrorResponse{
				Severity: "FATAL",
				Code:     "08006",
				Message:  "unknown app ID",
			})
			return nil
		}

		ctx := context.Background()

		clusterType, nsID, ok := strings.Cut(startup.Password, "-")

		// Look up the namespace to use.
		var ns *namespace.Namespace
		if !ok {
			ns, err = cm.ns.GetActive(ctx, app)
		} else {
			ns, err = cm.ns.GetByID(ctx, app, namespace.ID(nsID))
		}
		if err != nil {
			cm.log.Error().Err(err).Msg("dbproxy: could not find infra namespace")
			_ = cl.Backend.Send(&pgproto3.ErrorResponse{
				Severity: "FATAL",
				Code:     "08006",
				Message:  "unknown active infra namespace",
			})
			return nil
		}

		// Resolve the cluster type.
		var ct ClusterType
		switch clusterType {
		case "local":
			ct = Run
		case "test":
			ct = Test
		default:
			cm.log.Error().Str("password", startup.Password).Msg("dbproxy: invalid password for connection URI")
			_ = cl.Backend.Send(&pgproto3.ErrorResponse{
				Severity: "FATAL",
				Code:     "28P01", // 28P01 = invalid password
				Message:  "if connecting with an app slug as the username, the only accepted passwords are 'local' or 'test' to route to those instances on your local system",
			})
			return nil
		}

		// Create the cluster if it doesn't exist in memory yet
		// This might be because the daemon is running, but the hasn't done anything
		// with the app in question yet on this run
		cluster = cm.Create(context.Background(), &CreateParams{
			ClusterID: GetClusterID(app, ct, ns),
			Memfs:     false,
		})

		// Ensure the cluster is started
		_, err = cluster.Start(context.Background(), nil)
		if err != nil {
			cm.log.Error().Err(err).Msg("dbproxy: could not start cluster")
			_ = cl.Backend.Send(&pgproto3.ErrorResponse{
				Severity: "FATAL",
				Code:     "08006",
				Message:  "could not start database cluster",
			})
			return nil
		}
	}

	// If Encore knows about the database, check if it's ready
	// however if the cluster doesn't know about the database, skip this part.
	//
	// This is because either:
	//   1. The database exists and is connected to
	//   2. The database does not exist, and the remote server will return a "database doesn't exist" error.
	dbname := startup.Database
	db, ok := cluster.GetDB(dbname)
	if ok {
		var ready <-chan struct{}
		if waitForSetup {
			ready = db.Ready()
		} else {
			s := make(chan struct{})
			close(s)
			ready = s
		}

		// Wait for up to 60s for the cluster and database to come online.
		select {
		case <-db.Ctx.Done():
			_ = cl.Backend.Send(&pgproto3.ErrorResponse{
				Severity: "FATAL",
				Code:     "08006",
				Message:  "db is shutting down",
			})
			return nil
		case <-time.After(60 * time.Second):
			cm.log.Error().Str("db", db.DriverName).Msg("dbproxy: timed out waiting for database to come online")
			_ = cl.Backend.Send(&pgproto3.ErrorResponse{
				Severity: "FATAL",
				Code:     "08006",
				Message:  "timed out waiting for db to complete setup",
			})
			return nil

		case <-ready:
			// Continue connecting to backend, below
		}
	}

	info, err := cluster.Info(context.Background())
	if err != nil {
		_ = cl.Backend.Send(&pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "cluster not running: " + err.Error(),
		})
		return nil
	}

	server, err := net.Dial("tcp", info.Config.Host)
	if err != nil {
		_ = cl.Backend.Send(&pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "database not running: " + err.Error(),
		})
		return nil
	}
	defer server.Close()

	// Send a modified startup message to the backend
	admin, _ := info.Encore.First(RoleAdmin, RoleSuperuser)
	startup.Username = admin.Username
	startup.Password = admin.Password
	if db == nil {
		// We don't know about this database, we'll use the requested name
		// in case it does actually exist within the cluster.
		//
		// If it doesn't the cluster will return an SQL error to the client.
		startup.Database = dbname
	} else {
		startup.Database = db.DriverName
	}
	fe, err := pgproxy.SetupServer(server, &pgproxy.ServerConfig{
		TLS:     nil,
		Startup: startup,
	})
	if err != nil {
		_ = cl.Backend.Send(&pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "could not connect: " + err.Error(),
		})
		return nil
	}
	log.Trace().Msg("backend connection established, notifying client")

	if err := pgproxy.AuthenticateClient(cl.Backend); err != nil {
		return err
	}

	keyData, err := pgproxy.FinalizeInitialHandshake(cl.Backend, fe)
	if err != nil {
		_ = cl.Backend.Send(&pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "could not establish connection: " + err.Error(),
		})
		return nil
	}
	log.Trace().Msg("connection handshake completed, proxying steady-state data")

	// Store the key data so we know where to route cancellation requests.
	if keyData != nil {
		cm.mu.Lock()
		cm.backendKeyData[keyData.SecretKey] = cluster
		cm.mu.Unlock()
		defer func() {
			cm.mu.Lock()
			delete(cm.backendKeyData, keyData.SecretKey)
			cm.mu.Unlock()
		}()
	}

	return pgproxy.CopySteadyState(cl.Backend, fe)
}

// PreauthProxyConn is a pre-authenticated proxy conn directly specifically to the given cluster.
func (cm *ClusterManager) PreauthProxyConn(client net.Conn, id ClusterID) error {
	defer client.Close()
	cl, err := pgproxy.SetupClient(client, &pgproxy.ClientConfig{
		TLS: &tls.Config{},
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to setup client")
		return err
	}

	if cancel, ok := cl.Hello.(*pgproxy.CancelData); ok {
		cm.cancelRequest(client, cancel)
		return nil
	}
	startup := cl.Hello.(*pgproxy.StartupData)

	cluster, ok := cm.Get(id)
	if !ok {
		cm.log.Error().Interface("cluster", id).Msg("dbproxy: could not find cluster")
		_ = cl.Backend.Send(&pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "database cluster not running",
		})
		return nil
	}

	db, ok := cluster.GetDB(startup.Database)
	if !ok {
		_ = cl.Backend.Send(&pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "database not found",
		})
		return nil
	}

	// Wait for up to 60s for the cluster to come online.
	select {
	case <-db.Ctx.Done():
		_ = cl.Backend.Send(&pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "db is shutting down",
		})
		return nil
	case <-time.After(60 * time.Second):
		cm.log.Error().Str("db", startup.Database).Msg("dbproxy: timed out waiting for database to come online")
		_ = cl.Backend.Send(&pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "timed out waiting for db to complete setup",
		})
		return nil

	case <-cluster.Ready():
		// Continue connecting to backend, below
	}

	info, err := cluster.Info(context.Background())
	if err != nil {
		_ = cl.Backend.Send(&pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "cluster not running: " + err.Error(),
		})
		return nil
	}

	server, err := net.Dial("tcp", info.Config.Host)
	if err != nil {
		_ = cl.Backend.Send(&pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "database not running: " + err.Error(),
		})
		return nil
	}
	defer server.Close()

	admin, _ := info.Encore.First(RoleAdmin, RoleSuperuser)
	startup.Username = admin.Username
	startup.Password = admin.Password
	startup.Database = db.DriverName
	fe, err := pgproxy.SetupServer(server, &pgproxy.ServerConfig{
		TLS:     nil,
		Startup: startup,
	})
	if err != nil {
		_ = cl.Backend.Send(&pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "could not connect: " + err.Error(),
		})
		return nil
	}

	if err := pgproxy.AuthenticateClient(cl.Backend); err != nil {
		return err
	}

	keyData, err := pgproxy.FinalizeInitialHandshake(cl.Backend, fe)
	if err != nil {
		_ = cl.Backend.Send(&pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "could not establish connection: " + err.Error(),
		})
		return nil
	}

	// Store the key data so we know where to route cancellation requests.
	if keyData != nil {
		cm.mu.Lock()
		cm.backendKeyData[keyData.SecretKey] = cluster
		cm.mu.Unlock()
		defer func() {
			cm.mu.Lock()
			delete(cm.backendKeyData, keyData.SecretKey)
			cm.mu.Unlock()
		}()
	}

	log.Trace().Msg("successfully completed handshake, copying data back and forth")
	return pgproxy.CopySteadyState(cl.Backend, fe)
}

// cancelRequest handles a cancel request.
func (cm *ClusterManager) cancelRequest(client io.Writer, req *pgproxy.CancelData) {
	cm.mu.Lock()
	cluster, ok := cm.backendKeyData[req.Raw.SecretKey]
	cm.mu.Unlock()
	if !ok {
		return
	}

	info, err := cluster.Info(context.Background())
	if err != nil {
		msg := &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "database cluster not running",
		}
		client.Write(msg.Encode(nil))
		return
	}

	backend, err := net.Dial("tcp", info.Config.Host)
	if err != nil {
		msg := &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "database cluster not running",
		}
		client.Write(msg.Encode(nil))
		return
	}
	defer backend.Close()
	_ = pgproxy.SendCancelRequest(backend, req.Raw)
}

func writeMsg(w io.Writer, msg pgproto3.Message) error {
	_, err := w.Write(msg.Encode(nil))
	return err
}
