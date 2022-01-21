package sqldb

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgproto3/v2"
	"github.com/rs/zerolog/log"

	"encr.dev/cli/internal/platform"
	"encr.dev/pkg/pgproxy"
	"encr.dev/pkg/pgproxy2"
)

// OneshotProxy listens on a random port for a single connection, and proxies that connection to a remote db.
// It reports the one-time password and port to use.
// Once a connection has been established, it stops listening.
func OneshotProxy(appSlug, envSlug string) (port int, passwd string, err error) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, "", err
	}
	var passwdBytes [8]byte
	if _, err := rand.Read(passwdBytes[:]); err != nil {
		return 0, "", err
	}
	passwd = base64.RawURLEncoding.EncodeToString(passwdBytes[:])

	go oneshotServer(context.Background(), ln, passwd, appSlug, envSlug)
	return ln.Addr().(*net.TCPAddr).Port, passwd, nil
}

func oneshotServer(ctx context.Context, ln net.Listener, passwd, appSlug, envSlug string) error {
	proxy := &pgproxy2.SingleBackendProxy{
		RequirePassword: passwd != "",
		FrontendTLS:     nil,
		DialBackend: func(ctx context.Context, startup *pgproxy2.StartupMessage) (pgproxy2.LogicalConn, error) {
			if startup.Password != passwd {
				return nil, fmt.Errorf("bad password")
			}
			startupData := startup.Raw.Encode(nil)
			ws, err := platform.DBConnect(ctx, appSlug, envSlug, startup.Database, startupData)
			if err != nil {
				return nil, err
			}
			return &wsLogicalConn{Conn: ws}, nil
		},
	}

	return proxy.Serve(ctx, ln)
}

type wsLogicalConn struct {
	*websocket.Conn
	buf []byte
}

var _ pgproxy2.LogicalConn = (*wsLogicalConn)(nil)

func (c *wsLogicalConn) Write(p []byte) (int, error) {
	err := c.Conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *wsLogicalConn) Read(p []byte) (int, error) {
	// If we have remaining data from the previous message we received
	// from the stream, simply return that.
	if len(c.buf) > 0 {
		n := copy(p, c.buf)
		c.buf = c.buf[n:]
		return n, nil
	}

	// No more buffered data, wait for a new message from the stream.
	for {
		typ, data, err := c.Conn.ReadMessage()
		if err != nil {
			return 0, err
		} else if typ != websocket.BinaryMessage {
			continue
		}

		// Read as much data as possible directly to the waiting caller.
		// Anything remaining beyond that gets buffered until the next Read call.
		n := copy(p, data)
		c.buf = data[n:]
		return n, nil
	}
}

func (c *wsLogicalConn) Cancel(req *pgproxy2.CancelMessage) error {
	enc := base64.StdEncoding
	data := req.Raw.Encode(nil)
	encoded := make([]byte, enc.EncodedLen(len(data)))
	enc.Encode(encoded, data)
	log.Info().Msgf("sending cancel request %x", data)
	return c.Conn.WriteMessage(websocket.TextMessage, encoded)
}

func (c *wsLogicalConn) SetDeadline(t time.Time) error {
	_ = c.Conn.SetReadDeadline(t)
	err := c.Conn.SetWriteDeadline(t)
	return err
}

// ProxyRemoteConn proxies a frontend to the remote database pointed at by appSlug and envSlug.
// The passwd is what we expect the frontend to provide to authenticate the connection.
func ProxyRemoteConn(ctx context.Context, frontend net.Conn, passwd, appSlug, envSlug string) {
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

	// TODO
	ws, err := platform.DBConnect(ctx, appSlug, envSlug, data.Database, nil)
	if err != nil {
		writeMsg(frontend, &pgproto3.ErrorResponse{
			Severity: "FATAL",
			Code:     "08006",
			Message:  "could not connect to database: " + err.Error(),
		})
		return
	}

	defer ws.Close()

	sw := wsWriter{stream: ws}
	sr := wsReader{stream: ws}

	backend := &struct {
		wsWriter
		wsReader
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

// TODO(eandre) reimplement
/*
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
*/

/*
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

*/

type wsWriter struct {
	stream *websocket.Conn
}

func (w *wsWriter) Write(p []byte) (int, error) {
	err := w.stream.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

type wsReader struct {
	stream *websocket.Conn
	buf    []byte
}

func (r *wsReader) Read(p []byte) (int, error) {
	// If we have remaining data from the previous message we received
	// from the stream, simply return that.
	if len(r.buf) > 0 {
		n := copy(p, r.buf)
		r.buf = r.buf[n:]
		return n, nil
	}

	// No more buffered data, wait for a new message from the stream.
	for {
		typ, data, err := r.stream.ReadMessage()
		if err != nil {
			return 0, err
		} else if typ != websocket.BinaryMessage {
			continue
		}

		// Read as much data as possible directly to the waiting caller.
		// Anything remaining beyond that gets buffered until the next Read call.
		n := copy(p, data)
		r.buf = data[n:]
		return n, nil
	}
}
