package sqldb

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"encr.dev/cli/internal/platform"
	"encr.dev/pkg/pgproxy"
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
	proxy := &pgproxy.SingleBackendProxy{
		RequirePassword: passwd != "",
		FrontendTLS:     nil,
		DialBackend: func(ctx context.Context, startup *pgproxy.StartupData) (pgproxy.LogicalConn, error) {
			if startup.Password != passwd {
				return nil, fmt.Errorf("bad password")
			}
			startupData := startup.Raw.Encode(nil)
			ws, err := platform.DBConnect(ctx, appSlug, envSlug, startup.Database, startupData)
			if err != nil {
				return nil, err
			}
			conn := &WebsocketLogicalConn{Conn: ws}
			return conn, nil
		},
	}

	return proxy.Serve(ctx, ln)
}

type WebsocketLogicalConn struct {
	*websocket.Conn
	buf []byte
}

var _ pgproxy.LogicalConn = (*WebsocketLogicalConn)(nil)

func (c *WebsocketLogicalConn) Write(p []byte) (int, error) {
	err := c.Conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *WebsocketLogicalConn) Read(p []byte) (int, error) {
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

func (c *WebsocketLogicalConn) Cancel(req *pgproxy.CancelData) error {
	enc := base64.StdEncoding
	data := req.Raw.Encode(nil)
	encoded := make([]byte, enc.EncodedLen(len(data)))
	enc.Encode(encoded, data)
	log.Info().Msgf("sending cancel request %x", data)
	return c.Conn.WriteMessage(websocket.TextMessage, encoded)
}

func (c *WebsocketLogicalConn) SetDeadline(t time.Time) error {
	_ = c.Conn.SetReadDeadline(t)
	err := c.Conn.SetWriteDeadline(t)
	return err
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
