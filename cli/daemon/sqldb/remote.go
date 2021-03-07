package sqldb

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"time"

	"encr.dev/pkg/pgproxy"
	"encr.dev/proto/encore/server/remote"
	"github.com/jackc/pgproto3/v2"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/metadata"
)

// OneshotProxy listens on a random port for a single connection, and proxies that connection to a remote db.
// It reports the one-time password and port to use.
// Once a connection has been established, it stops listening.
func OneshotProxy(rc remote.RemoteClient, appSlug, envSlug string) (port int, passwd string, err error) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, "", err
	}
	var passwdBytes [8]byte
	if _, err := rand.Read(passwdBytes[:]); err != nil {
		return 0, "", err
	}
	passwd = base64.RawURLEncoding.EncodeToString(passwdBytes[:])

	go oneshotServer(context.Background(), rc, ln, passwd, appSlug, envSlug)
	return ln.Addr().(*net.TCPAddr).Port, passwd, nil
}

func oneshotServer(ctx context.Context, rc remote.RemoteClient, ln net.Listener, passwd, appSlug, envSlug string) error {
	defer ln.Close()

	gotMainConn := make(chan struct{}) // closed when accepted
	go func() {
		// Wait for the first conn at most 60s before giving up
		select {
		case <-gotMainConn:
		case <-time.After(60 * time.Second):
			ln.Close()
		case <-ctx.Done():
			ln.Close()
		}
	}()

	var tempDelay time.Duration // how long to sleep on accept failure
	first := true
	for {
		frontend, e := ln.Accept()
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
				log.Printf("sqldb: accept error: %v; retrying in %v", e, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return fmt.Errorf("sqldb: could not accept: %v", e)
		}

		tempDelay = 0

		if first {
			// If this is the first connection, treat it as the main connection
			// and close the listener when it exits.
			first = false
			close(gotMainConn)
			go ProxyRemoteConn(ctx, rc, frontend, passwd, appSlug, envSlug)
		} else {
			go ProxyRemoteConn(ctx, rc, frontend, passwd, appSlug, envSlug)
		}
	}
}

// ProxyRemoteConn proxies a frontend to the remote database pointed at by appSlug and envSlug.
// The passwd is what we expect the frontend to provide to authenticate the connection.
func ProxyRemoteConn(ctx context.Context, rc remote.RemoteClient, frontend net.Conn, passwd, appSlug, envSlug string) {
	defer frontend.Close()
	var proxy pgproxy.Proxy
	data, err := proxy.FrontendAuth(frontend, nil, passwd != "")
	if err != nil {
		log.Printf("sqldb: proxy handshake error: %v", err)
		return
	}

	// If we are setting up a real connection (as opposed to issuing a cancel request, which
	// does not use password based auth), make sure the password matches.
	if _, ok := data.Startup.(*pgproxy.StartupMessage); ok && data.Password != passwd {
		writeMsg(frontend, &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "invalid password",
		})
		return
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "appSlug", appSlug, "envSlug", envSlug)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := rc.DBConnect(ctx)
	if err != nil {
		log.Printf("sqldb: proxy: could not connect to remote db: %v", err)
		return
	}

	sw := dbConnectWriter{stream: stream}
	sr := dbConnectReader{stream: stream}
	backend := &struct {
		dbConnectWriter
		dbConnectReader
	}{sw, sr}

	data.Username = "encore"
	data.Password = ""
	if _, err := proxy.BackendAuth(backend, nil, data); err != nil {
		log.Printf("sqldb: proxy: could not connect to remote db: %v", err)
		writeMsg(frontend, &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "could not connect to remote db: " + err.Error(),
		})
		return
	}

	proxy.Data(ctx)
}

type dbConnectWriter struct {
	stream remote.Remote_DBConnectClient
}

func (w *dbConnectWriter) Write(p []byte) (int, error) {
	err := w.stream.Send(&remote.Data{Data: p})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

type dbConnectReader struct {
	stream remote.Remote_DBConnectClient
	buf    []byte
}

func (r *dbConnectReader) Read(p []byte) (int, error) {
	// If we have remaining data from the previous message we received
	// from the stream, simply return that.
	if len(r.buf) > 0 {
		n := copy(p, r.buf)
		r.buf = r.buf[n:]
		return n, nil
	}

	// No more buffered data, wait for a new message from the stream.
	msg, err := r.stream.Recv()
	if err != nil {
		return 0, err
	}
	// Read as much data as possible directly to the waiting caller.
	// Anything remaining beyond that gets buffered until the next Read call.
	n := copy(p, msg.Data)
	r.buf = msg.Data[n:]
	return n, nil
}
