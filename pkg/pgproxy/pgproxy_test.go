package pgproxy

import (
	"fmt"
	"net"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/pgproto3"
)

// pipe returns a connected pair of conns that fail instead of hanging
// if a message is never flushed.
func pipe(c *qt.C) (net.Conn, net.Conn) {
	a, b := net.Pipe()
	deadline := time.Now().Add(5 * time.Second)
	c.Assert(a.SetDeadline(deadline), qt.IsNil)
	c.Assert(b.SetDeadline(deadline), qt.IsNil)
	c.Cleanup(func() {
		_ = a.Close()
		_ = b.Close()
	})
	return a, b
}

// TestProxyRoundTrip runs a client through the proxy to a fake server:
// startup, password capture, MD5 authentication against the server,
// the initial handshake and a query in the steady state.
func TestProxyRoundTrip(t *testing.T) {
	c := qt.New(t)
	clientConn, proxyClientConn := pipe(c)
	proxyServerConn, serverConn := pipe(c)

	salt := [4]byte{1, 2, 3, 4}
	secretKey := []byte{5, 6, 7, 8}

	// The fake server.
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- func() error {
			be := pgproto3.NewBackend(serverConn, serverConn)
			msg, err := be.ReceiveStartupMessage()
			if err != nil {
				return err
			}
			startup, ok := msg.(*pgproto3.StartupMessage)
			if !ok {
				return fmt.Errorf("expected StartupMessage, got %T", msg)
			} else if db, user := startup.Parameters["database"], startup.Parameters["user"]; db != "real-db" || user != "real-user" {
				return fmt.Errorf("got database %q and user %q", db, user)
			}

			be.Send(&pgproto3.AuthenticationMD5Password{Salt: salt})
			if err := be.Flush(); err != nil {
				return err
			}
			msg, err = be.Receive()
			if err != nil {
				return err
			}
			passwd, ok := msg.(*pgproto3.PasswordMessage)
			if !ok {
				return fmt.Errorf("expected PasswordMessage, got %T", msg)
			} else if want := computeMD5("real-user", "real-password", salt); passwd.Password != want {
				return fmt.Errorf("got password %q, want %q", passwd.Password, want)
			}

			be.Send(&pgproto3.AuthenticationOk{})
			be.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "16.0"})
			be.Send(&pgproto3.BackendKeyData{ProcessID: 42, SecretKey: secretKey})
			be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
			if err := be.Flush(); err != nil {
				return err
			}

			msg, err = be.Receive()
			if err != nil {
				return err
			} else if q, ok := msg.(*pgproto3.Query); !ok || q.String != "SELECT 1" {
				return fmt.Errorf("expected Query, got %#v", msg)
			}
			be.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 1")})
			be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
			if err := be.Flush(); err != nil {
				return err
			}

			msg, err = be.Receive()
			if err != nil {
				return err
			} else if _, ok := msg.(*pgproto3.Terminate); !ok {
				return fmt.Errorf("expected Terminate, got %T", msg)
			}
			return nil
		}()
	}()

	// The proxy.
	type proxyResult struct {
		hello *StartupData
		key   *pgproto3.BackendKeyData
		err   error
	}
	proxyDone := make(chan proxyResult, 1)
	go func() {
		var res proxyResult
		res.err = func() error {
			cl, err := SetupClient(proxyClientConn, &ClientConfig{WantPassword: true})
			if err != nil {
				return err
			}
			res.hello = cl.Hello.(*StartupData)
			startup := *res.hello
			startup.Database = "real-db"
			startup.Username = "real-user"
			startup.Password = "real-password"
			fe, err := SetupServer(proxyServerConn, &ServerConfig{Startup: &startup})
			if err != nil {
				return err
			}
			if err := AuthenticateClient(cl.Backend); err != nil {
				return err
			}
			res.key, err = FinalizeInitialHandshake(cl.Backend, fe)
			if err != nil {
				return err
			}
			return CopySteadyState(cl.Backend, fe)
		}()
		proxyDone <- res
	}()

	// The client.
	fe := pgproto3.NewFrontend(clientConn, clientConn)
	fe.Send(&pgproto3.StartupMessage{
		ProtocolVersion: pgproto3.ProtocolVersionNumber,
		Parameters:      map[string]string{"database": "db", "user": "user"},
	})
	c.Assert(fe.Flush(), qt.IsNil)

	receive := func() pgproto3.BackendMessage {
		msg, err := fe.Receive()
		c.Assert(err, qt.IsNil)
		return msg
	}
	c.Assert(receive(), qt.DeepEquals, &pgproto3.AuthenticationCleartextPassword{})
	fe.Send(&pgproto3.PasswordMessage{Password: "client-password"})
	c.Assert(fe.Flush(), qt.IsNil)

	c.Assert(receive(), qt.DeepEquals, &pgproto3.AuthenticationOk{})
	c.Assert(receive(), qt.DeepEquals, &pgproto3.ParameterStatus{Name: "server_version", Value: "16.0"})
	c.Assert(receive(), qt.DeepEquals, &pgproto3.BackendKeyData{ProcessID: 42, SecretKey: secretKey})
	c.Assert(receive(), qt.DeepEquals, &pgproto3.ReadyForQuery{TxStatus: 'I'})

	fe.Send(&pgproto3.Query{String: "SELECT 1"})
	c.Assert(fe.Flush(), qt.IsNil)
	c.Assert(receive(), qt.DeepEquals, &pgproto3.CommandComplete{CommandTag: []byte("SELECT 1")})
	c.Assert(receive(), qt.DeepEquals, &pgproto3.ReadyForQuery{TxStatus: 'I'})

	fe.Send(&pgproto3.Terminate{})
	c.Assert(fe.Flush(), qt.IsNil)

	res := <-proxyDone
	c.Assert(res.err, qt.IsNil)
	c.Assert(res.hello.Database, qt.Equals, "db")
	c.Assert(res.hello.Username, qt.Equals, "user")
	c.Assert(res.hello.Password, qt.Equals, "client-password")
	c.Assert(res.key, qt.DeepEquals, &pgproto3.BackendKeyData{ProcessID: 42, SecretKey: secretKey})
	c.Assert(<-serverErr, qt.IsNil)
}

func TestSendCancelRequest(t *testing.T) {
	c := qt.New(t)
	clientConn, serverConn := pipe(c)

	req := &pgproto3.CancelRequest{ProcessID: 42, SecretKey: []byte{5, 6, 7, 8}}
	received := make(chan pgproto3.FrontendMessage, 1)
	go func() {
		be := pgproto3.NewBackend(serverConn, serverConn)
		msg, err := be.ReceiveStartupMessage()
		if err != nil {
			received <- nil
		} else {
			received <- msg
		}
		_ = serverConn.Close()
	}()

	c.Assert(SendCancelRequest(clientConn, req), qt.IsNil)
	c.Assert(<-received, qt.DeepEquals, req)
}
