package pgproxy

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"encr.dev/pkg/pgproxy/internal/pgio"
	"encr.dev/pkg/pgproxy/internal/pgproto3"
)

// ErrCancelled is reported when the proxy is closed
// due to requesting cancellation.
var ErrCancelled = errors.New("request cancelled")

// FrontendMessage represents messages received from the frontend.
type FrontendMessage interface {
	pgproto3.FrontendMessage
}

type (
	StartupMessage = pgproto3.StartupMessage
	CancelRequest  = pgproto3.CancelRequest
)

// AuthData is the authentication data received from the frontend.
type AuthData struct {
	Startup  FrontendMessage // *StartupMessage or *CancelRequest
	Database string
	Username string
	Password string // may be empty if ProxyConfig.RequirePassword is false
}

// Proxy proxies the Postgres wire protocol between a client and a server,
// injecting custom behavior into the authentication and server selection
// at startup.
type Proxy struct {
	Debug    bool
	frontend *conn
	backend  *conn
}

// SimpleConfig provides a convenient approach to setting up most common proxy scenarios.
type SimpleConfig struct {
	RequirePassword bool
	FrontendTLS     *tls.Config
	BackendTLS      *tls.Config
	Debug           bool
}

// Proxy begins proxying from frontend to backend.
func (cfg *SimpleConfig) Proxy(ctx context.Context, frontend, backend io.ReadWriter) error {
	p := &Proxy{Debug: cfg.Debug}
	if auth, err := p.FrontendAuth(frontend, cfg.FrontendTLS, cfg.RequirePassword); err != nil {
		return err
	} else if _, err := p.BackendAuth(backend, cfg.BackendTLS, auth); err != nil {
		if errors.Is(err, ErrCancelled) {
			return nil
		}
		return err
	}
	return p.Data(ctx)
}

// FrontendAuth performs the client-side authentication step, and reports the auth data received.
func (p *Proxy) FrontendAuth(frontend io.ReadWriter, tlsCfg *tls.Config, requirePassword bool) (*AuthData, error) {
	if _, ok := frontend.(net.Conn); !ok && tlsCfg != nil {
		panic("pgproxy: frontend connection with TLS must provide a net.Conn")
	}
	p.frontend = &conn{rw: frontend}
	conn, err := p.negotiateSSL(p.frontend, tlsCfg, false)
	if err != nil {
		return nil, err
	}
	p.frontend = conn

	startup, err := p.readStartupMsg(p.frontend)
	if err != nil {
		return nil, err
	}

	data := &AuthData{Startup: startup}
	if startup, ok := startup.(*StartupMessage); ok {
		data.Username = startup.Parameters["user"]
		data.Database = startup.Parameters["database"]
		if data.Database == "" {
			return nil, fmt.Errorf("missing database name in connection string")
		}
		if requirePassword {
			var err error
			data.Password, err = readPassword(p.frontend)
			if err != nil {
				return nil, err
			}
		}
	}

	return data, nil
}

// BackendData is the data reported from the backend after authentication is completed.
type BackendData struct {
	KeyData *pgproto3.BackendKeyData // may be nil
}

// BackendAuth performs the server-side authentication step, and reports any key data received from the server
// for use with cancellation requests.
func (p *Proxy) BackendAuth(backend io.ReadWriter, tlsCfg *tls.Config, auth *AuthData) (*BackendData, error) {
	if _, ok := backend.(net.Conn); !ok && tlsCfg != nil {
		panic("pgproxy: backend connection with TLS must provide a net.Conn")
	}
	p.backend = &conn{rw: backend}
	conn, err := p.negotiateSSL(p.backend, tlsCfg, true)
	if err != nil {
		return nil, err
	}
	p.backend = conn

	switch startup := auth.Startup.(type) {
	case *CancelRequest:
		p.debug("sending cancellation request: %+v", *startup)
		if err := p.backend.WriteMsg(startup); err != nil {
			return nil, fmt.Errorf("pgproxy: could not send cancel request: %v", err)
		}
		return nil, ErrCancelled

	case *StartupMessage:
		// Update the startup parameters in case they were changed
		startup.Parameters["user"] = auth.Username
		startup.Parameters["database"] = auth.Database
		if err := pgio.WriteMsg(p.backend, startup); err != nil {
			return nil, fmt.Errorf("pgproxy: could not send startup msg: %v", err)
		}

		if err := authenticateBackend(p.backend, auth); err != nil {
			var e *pgErr
			if errors.As(err, &e) {
				_ = p.frontend.WriteMsg(&e.msg)
			}
			return nil, fmt.Errorf("pgproxy: could not authenticate: %v", err)
		}

		// Notify the frontend of successful auth
		if err := pgio.WriteMsg(p.frontend, &pgproto3.Authentication{Type: pgproto3.AuthTypeOk}); err != nil {
			return nil, fmt.Errorf("pgproxy: could not write to frontend: %v", err)
		}

		// Read messages from backend until we get ReadyForQuery
		var data BackendData
		for {
			msg, err := p.backend.ReadMsg()
			if err != nil {
				return nil, fmt.Errorf("pgproxy: cannot read from backend: %v", err)
			} else if err := p.frontend.WriteMsg(msg); err != nil {
				return nil, fmt.Errorf("pgproxy: could not write to frontend: %v", err)
			}

			switch msg.Typ {
			case 'K': // BackendKeyData
				var keyData pgproto3.BackendKeyData
				if err := keyData.Decode(msg.Buf); err != nil {
					return nil, fmt.Errorf("pgproxy: could not decode backend key data: %v", err)
				}
				data.KeyData = &keyData
			case 'Z': // ReadyForQuery
				return &data, nil
			}
		}

	default:
		return nil, fmt.Errorf("pgproxy: cannot perform backend handshake: unexpected startup message type %T", startup)
	}
}

// Data proxies the steady-state data between the frontend and backend once both sides are authenticated.
func (p *Proxy) Data(ctx context.Context) error {
	p.debug("proxying data between frontend and backend (steady-state)")
	defer p.debug("proxy closed")

	quit := make(chan struct{}, 2)
	go func() {
		io.Copy(p.frontend, p.backend)
		quit <- struct{}{}
	}()
	go func() {
		io.Copy(p.backend, p.frontend)
		quit <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-quit:
		// Wait up to 1s for graceful exit
		select {
		case <-quit:
		case <-ctx.Done():
		case <-time.After(1 * time.Second):
		}
		return nil
	}
}

func (p *Proxy) debug(format string, args ...interface{}) {
	if p.Debug {
		format = "pgproxy: " + format
		log.Printf(format, args...)
	}
}

func (p *Proxy) negotiateSSL(c *conn, cfg *tls.Config, server bool) (*conn, error) {
	if server {
		if cfg == nil {
			// We don't want SSL so just don't request it.
			p.debug("skipping TLS negotiation with server since no TLS config was given")
			return c, nil
		}
		p.debug("negotiating TLS with server...")
		msg := &pgproto3.SSLMessage{}
		if err := c.WriteMsg(msg); err != nil {
			return nil, err
		}
		var resp [1]byte
		if _, err := io.ReadFull(c, resp[:]); err != nil {
			return nil, err
		}
		switch resp[0] {
		case 'S':
			p.debug("server accepted TLS request")
			c2 := tls.Client(c.rw.(net.Conn), cfg)
			if err := c2.Handshake(); err != nil {
				p.debug("TLS handshake failed: %v", err)
				c2.Close()
				return nil, fmt.Errorf("TLS handshake failed: %v", err)
			}
			p.debug("TLS handshake completed")
			return &conn{rw: c2}, nil
		case 'N':
			p.debug("server rejected TLS request")
			return nil, fmt.Errorf("server rejected TLS")
		case 'E':
			// ErrorMessage
			p.debug("server responded with ErrorResponse to TLS request")
			c.UnreadByte(resp[0])
			var msg pgproto3.ErrorResponse
			if err := c.ReadInto(&msg); err != nil {
				p.debug("could not parse ErrorResponse: %v", err)
			}
			p.debug("TLS negotiation error: %+v", msg)
			return nil, fmt.Errorf("could not negotiate TLS: error %s: %s", msg.Code, msg.Message)
		default:
			return nil, fmt.Errorf("got unexpected response to TLS Request: %v", resp[0])
		}
	}

	p.debug("checking if client wants TLS")
	msg, err := p.readStartupMsg(c)
	if err == nil {
		// readStartupMsg returns err == errSSL if the client requests TLS,
		// so if we get here it means the client did not request TLS.
		p.debug("client did not request TLS")
		if cfg == nil {
			// We don't require TLS, so proceed.
			// Unread the startup message so we pick it up again when we do the startup.
			c.UnreadMsg(msg)
			return c, nil
		}
		return nil, fmt.Errorf("client did not request TLS")
	} else if err != errSSL {
		return nil, err
	}

	p.debug("client requested TLS")
	if cfg == nil {
		// We got an SSL request but we don't want to use TLS
		if _, err := c.Write([]byte{'N'}); err != nil {
			return nil, err
		}
		return c, nil
	}

	if _, err := c.Write([]byte{'S'}); err != nil {
		return nil, err
	}
	c2 := tls.Server(c.rw.(net.Conn), cfg)
	if err := c2.Handshake(); err != nil {
		c2.Close()
		return nil, fmt.Errorf("TLS handshake failed: %v", err)
	}
	return &conn{rw: c2}, nil
}

const sslProtocolVersionNumber = 80877103

var errSSL = errors.New("client requested SSL")

func (p *Proxy) readStartupMsg(c *conn) (FrontendMessage, error) {
	var hdr [8]byte
	if _, err := io.ReadFull(c, hdr[:]); err != nil {
		return nil, err
	}

	// Read the rest of the message. Splice the first 4 bytes from
	// the header which we've already read, and only read the rest from the conn.
	// This is so that we can use pgproto3.StartupMessage.Decode() to parse
	// the message, which expects the protocol version to be part of the buffer.
	msgSize := int(binary.BigEndian.Uint32(hdr[0:4]) - 4)
	if msgSize > (1 << 20) {
		return nil, fmt.Errorf("startup message too big (> 1GiB)")
	}
	buf := make([]byte, msgSize)
	copy(buf[0:4], hdr[4:8])
	if _, err := io.ReadFull(c, buf[4:]); err != nil {
		return nil, err
	}

	code := binary.BigEndian.Uint32(hdr[4:8])
	p.debug("got startup message code: %v", code)
	switch code {
	case sslProtocolVersionNumber:
		return nil, errSSL
	case pgproto3.ProtocolVersionNumber:
		var msg pgproto3.StartupMessage
		if err := msg.Decode(buf); err != nil {
			return nil, fmt.Errorf("could not parse startup message: %v", err)
		}
		return &msg, nil
	case pgproto3.CancelRequestCode:
		var msg pgproto3.CancelRequest
		if err := msg.Decode(buf); err != nil {
			return nil, fmt.Errorf("could not parse cancel request message: %v", err)
		}
		return &msg, nil
	default:
		return nil, fmt.Errorf("unknown startup message code: %d", code)
	}
}

func readPassword(frontend *conn) (string, error) {
	err := frontend.WriteMsg(&pgproto3.Authentication{
		Type: pgproto3.AuthTypeCleartextPassword,
	})
	if err != nil {
		return "", err
	}
	msg, err := frontend.ReadMsg()
	if err != nil {
		return "", err
	}
	if msg.Typ != 'p' {
		return "", fmt.Errorf("expected password msg, got %v", msg.Typ)
	}
	var pwdMsg pgproto3.PasswordMessage
	if err := pwdMsg.Decode(msg.Buf); err != nil {
		return "", fmt.Errorf("could not decode password message: %v", err)
	}
	return pwdMsg.Password, nil
}

func authenticateBackend(backend *conn, data *AuthData) error {
	for {
		msg, err := backend.ReadMsg()
		if err != nil {
			return err
		}
		switch msg.Typ {
		case 'R': // Authentication Required
			var auth pgproto3.Authentication
			if err := auth.Decode(msg.Buf); err != nil {
				return err
			}
			switch auth.Type {
			case pgproto3.AuthTypeOk:
				return nil
			case pgproto3.AuthTypeCleartextPassword:
				pwdMsg := &pgproto3.PasswordMessage{
					Password: data.Password,
				}
				if err := backend.WriteMsg(pwdMsg); err != nil {
					return err
				}
			case pgproto3.AuthTypeMD5Password:
				// Send the password hashed as:
				// concat('md5', md5(concat(md5(concat(password, username)), random-salt)))

				// s1 := md5(concat(password, username))
				s1 := md5.Sum([]byte(data.Password + data.Username))
				// s2 := md5(concat(s1, random-salt))
				s2 := md5.Sum([]byte(hex.EncodeToString(s1[:]) + string(auth.Salt[:])))
				pwdMsg := &pgproto3.PasswordMessage{
					Password: "md5" + hex.EncodeToString(s2[:]),
				}
				if err := backend.WriteMsg(pwdMsg); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unknown authentication type %v", auth.Type)
			}

		case 'E':
			var resp pgproto3.ErrorResponse
			if err := resp.Decode(msg.Buf); err != nil {
				return err
			}
			return &pgErr{msg: resp}
		default:
			return fmt.Errorf("unexpected message type from backend: %x", msg.Typ)
		}
	}
}

type conn struct {
	rw     io.ReadWriter
	unread []byte
}

func (c *conn) ReadInto(msg pgio.PgMsg) error {
	raw, err := c.ReadMsg()
	if err != nil {
		return err
	}
	return msg.Decode(raw.Buf)
}

func (c *conn) ReadMsg() (*pgio.RawMsg, error) {
	return pgio.ReadMsg(c)
}

func (c *conn) UnreadMsg(msg pgio.PgMsg) {
	c.unread = msg.Encode(nil)
}

func (c *conn) UnreadByte(b byte) {
	c.unread = []byte{b}
}

func (c *conn) Read(p []byte) (n int, err error) {
	if len(c.unread) > 0 {
		n = copy(p, c.unread)
		c.unread = c.unread[n:]
		return n, nil
	}
	n, err = c.rw.Read(p)
	return
}

func (c *conn) Write(p []byte) (n int, err error) {
	return c.rw.Write(p)
}

func (c *conn) WriteMsg(msg pgio.PgMsg) error {
	return pgio.WriteMsg(c, msg)
}

type pgErr struct {
	msg pgproto3.ErrorResponse
}

func (e *pgErr) Error() string {
	return fmt.Sprintf("%s - code: %s", e.msg.Message, e.msg.Code)
}
