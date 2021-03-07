package sqldb

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"encr.dev/pkg/pgproxy"
	"github.com/jackc/pgproto3/v2"
	"github.com/rs/zerolog/log"
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
func (cm *ClusterManager) ProxyConn(frontend net.Conn, waitForSetup bool) error {
	defer frontend.Close()
	var proxy pgproxy.Proxy

	data, err := proxy.FrontendAuth(frontend, nil, true)
	if err != nil {
		return err
	}

	if cancel, ok := data.Startup.(*pgproxy.CancelRequest); ok {
		cm.cancelRequest(frontend, cancel)
		return nil
	}

	clusterID := data.Password
	cluster, ok := cm.Get(clusterID)
	if !ok {
		cm.log.Error().Str("cluster", clusterID).Msg("dbproxy: could not find cluster")
		writeMsg(frontend, &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "database cluster not running",
		})
		return nil
	}

	db, ok := cluster.GetDB(data.Database)
	if !ok {
		writeMsg(frontend, &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "database not found",
		})
		return nil
	}

	var ready <-chan struct{}
	if waitForSetup {
		ready = db.Ready()
	} else {
		s := make(chan struct{})
		close(s)
		ready = s
	}

	// Wait for up to 15s for the cluster and database to come online.
	select {
	case <-db.Ctx.Done():
		writeMsg(frontend, &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "db is shutting down",
		})
		return nil
	case <-time.After(15 * time.Second):
		cm.log.Error().Str("db", data.Database).Msg("dbproxy: timed out waiting for database to come online")
		writeMsg(frontend, &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "timed out waiting for db to complete setup",
		})
		return nil

	case <-ready:
		// Continue connecting to backend, below
	}

	backend, err := net.Dial("tcp", cluster.HostPort)
	if err != nil {
		writeMsg(frontend, &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "database not running: " + err.Error(),
		})
		return nil
	}
	defer backend.Close()

	data.Username = "encore"
	data.Password = clusterID
	beData, err := proxy.BackendAuth(backend, nil, data)
	if err != nil {
		writeMsg(frontend, &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "could not connect: " + err.Error(),
		})
		return nil
	}

	// Store the key data so we know where to route cancellation requests.
	if key := beData.KeyData; key != nil {
		cm.mu.Lock()
		cm.backendKeyData[key.SecretKey] = cluster
		cm.mu.Unlock()
		defer func() {
			cm.mu.Lock()
			delete(cm.backendKeyData, key.SecretKey)
			cm.mu.Unlock()
		}()
	}

	return proxy.Data(db.Ctx)
}

// PreauthProxyConn is a pre-authenticated proxy conn directly specifically to the given cluster.
func (cm *ClusterManager) PreauthProxyConn(frontend net.Conn, clusterID string) error {
	defer frontend.Close()
	var proxy pgproxy.Proxy

	data, err := proxy.FrontendAuth(frontend, nil, true)
	if err != nil {
		return err
	}

	if cancel, ok := data.Startup.(*pgproxy.CancelRequest); ok {
		cm.cancelRequest(frontend, cancel)
		return nil
	}

	cluster, ok := cm.Get(clusterID)
	if !ok {
		cm.log.Error().Str("cluster", clusterID).Msg("dbproxy: could not find cluster")
		writeMsg(frontend, &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "database cluster not running",
		})
		return nil
	}

	db, ok := cluster.GetDB(data.Database)
	if !ok {
		writeMsg(frontend, &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "database not found",
		})
		return nil
	}

	// Wait for up to 15s for the cluster to come online.
	select {
	case <-db.Ctx.Done():
		writeMsg(frontend, &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "db is shutting down",
		})
		return nil
	case <-time.After(15 * time.Second):
		cm.log.Error().Str("db", data.Database).Msg("dbproxy: timed out waiting for database to come online")
		writeMsg(frontend, &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "timed out waiting for db to complete setup",
		})
		return nil

	case <-cluster.Ready():
		// Continue connecting to backend, below
	}

	backend, err := net.Dial("tcp", cluster.HostPort)
	if err != nil {
		writeMsg(frontend, &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "database not running: " + err.Error(),
		})
		return nil
	}
	defer backend.Close()

	data.Username = "encore"
	data.Password = clusterID
	beData, err := proxy.BackendAuth(backend, nil, data)
	if err != nil {
		writeMsg(frontend, &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "could not connect: " + err.Error(),
		})
		return nil
	}

	// Store the key data so we know where to route cancellation requests.
	if key := beData.KeyData; key != nil {
		cm.mu.Lock()
		cm.backendKeyData[key.SecretKey] = cluster
		cm.mu.Unlock()
		defer func() {
			cm.mu.Lock()
			delete(cm.backendKeyData, key.SecretKey)
			cm.mu.Unlock()
		}()
	}

	return proxy.Data(db.Ctx)
}

// cancelRequest handles a cancel request.
func (cm *ClusterManager) cancelRequest(frontend io.ReadWriter, req *pgproxy.CancelRequest) {
	cm.mu.Lock()
	cluster, ok := cm.backendKeyData[req.SecretKey]
	cm.mu.Unlock()
	if !ok {
		return
	}

	backend, err := net.Dial("tcp", cluster.HostPort)
	if err != nil {
		writeMsg(frontend, &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "database cluster not running",
		})
		return
	}
	writeMsg(backend, req)
	backend.Close()
}

func writeMsg(w io.Writer, msg pgproto3.Message) error {
	_, err := w.Write(msg.Encode(nil))
	return err
}
