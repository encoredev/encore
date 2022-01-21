package pgproxy2

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"encr.dev/pkg/pgproxy2/internal/pgio"
	"encr.dev/pkg/pgproxy2/internal/pgproto3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// FrontendMessage represents messages received from the frontend.
type FrontendMessage interface {
	pgproto3.FrontendMessage
}

type FrontendHello interface {
	hello()
}

type StartupMessage struct {
	Raw      *StartupMsg
	Database string
	Username string
	Password string // may be empty if ProxyConfig.RequirePassword is false
}

type CancelMessage struct {
	Raw *CancelRequest
}

func (*StartupMessage) hello() {}
func (*CancelMessage) hello()  {}

// ReadFrontendHello reads the frontend data from the conn.
func ReadFrontendHello(ctx context.Context, frontend *net.Conn, tls *tls.Config, requirePassword bool) (FrontendHello, error) {
	log := log.Logger
	fe := &peekableConn{conn: *frontend}

	var err error
	fe, err = negotiateClientTLS(log, fe, tls)
	if err != nil {
		return nil, err
	}
	*frontend = fe.conn

	startup, err := readStartupMsg(log, fe)
	if err != nil {
		return nil, err
	}

	switch msg := startup.(type) {
	case *pgproto3.StartupMessage:
		data := &StartupMessage{
			Raw:      msg,
			Username: msg.Parameters["user"],
			Database: msg.Parameters["database"],
		}
		if data.Database == "" {
			return nil, fmt.Errorf("missing database name in connection string")
		}
		if requirePassword {
			var err error
			data.Password, err = readPassword(fe)
			if err != nil {
				return nil, err
			}
		}
		return data, nil

	case *pgproto3.CancelRequest:
		return &CancelMessage{Raw: msg}, nil
	default:
		return nil, fmt.Errorf("unsupported startup message type %T", msg)
	}
}

// WriteBackendHello authenticates with the backend.
func WriteBackendHello(ctx context.Context, backend *net.Conn, tls *tls.Config, startup *StartupMessage) error {
	log := log.Logger
	be := &peekableConn{conn: *backend}
	var err error
	be, err = negotiateServerTLS(log, be, tls)
	if err != nil {
		return err
	}
	*backend = be.conn

	// Update the startup parameters in case they were changed
	raw := startup.Raw
	raw.Parameters["user"] = startup.Username
	raw.Parameters["database"] = startup.Database
	if err := pgio.WriteMsg(be, raw); err != nil {
		return fmt.Errorf("pgproxy: could not send startup msg: %v", err)
	}

	if err := authenticateBackend(log, be, startup); err != nil {
		return fmt.Errorf("pgproxy: could not authenticate: %w", err)
	}
	return nil
}

func CompleteFrontendHandshake(ctx context.Context, frontend net.Conn, backendErr error) error {
	if backendErr != nil {
		log.Error().Err(backendErr).Msg("got backend handshake err")
		var e *pgErr
		if errors.As(backendErr, &e) {
			pgio.WriteMsg(frontend, &e.msg)
		} else {
			pgio.WriteMsg(frontend, &pgproto3.ErrorResponse{
				Severity: "FATAL",
				Code:     "08006",
				Message:  "could not authenticate with backend",
			})
		}
		return fmt.Errorf("pgproxy: could not authenticate with backend: %v", backendErr)
	}

	if err := pgio.WriteMsg(frontend, &pgproto3.Authentication{Type: pgproto3.AuthTypeOk}); err != nil {
		return fmt.Errorf("pgproxy: could not write to frontend: %v", err)
	}
	return nil
}

func SniffBackendKeyData(ctx context.Context, frontend, backend net.Conn) (*pgproto3.BackendKeyData, error) {
	for {
		msg, err := pgio.ReadMsg(backend)
		if err != nil {
			return nil, fmt.Errorf("pgproxy: cannot read from backend: %v", err)
		} else if err := pgio.WriteMsg(frontend, msg); err != nil {
			return nil, fmt.Errorf("pgproxy: could not write to frontend: %v", err)
		}

		switch msg.Typ {
		case 'K': // BackendKeyData
			var keyData pgproto3.BackendKeyData
			if err := keyData.Decode(msg.Buf); err != nil {
				return nil, fmt.Errorf("pgproxy: could not decode backend key data: %v", err)
			}
			return &keyData, nil
		case 'Z': // ReadyForQuery
			return nil, nil
		}
	}
}

func CopySteadyState(ctx context.Context, frontend, backend net.Conn) error {
	quit := make(chan struct{}, 2)
	go func() {
		io.Copy(frontend, backend)
		quit <- struct{}{}
	}()
	go func() {
		io.Copy(backend, frontend)
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

func negotiateClientTLS(log zerolog.Logger, c *peekableConn, cfg *tls.Config) (*peekableConn, error) {
	log.Debug().Msg("checking if client wants tls")
	msg, err := readStartupMsg(log, c)
	if err == nil {
		// readStartupMsg returns err == errSSL if the client requests TLS,
		// so if we get here it means the client did not request TLS.
		log.Debug().Msg("client did not request tls")
		if cfg == nil {
			// We don't require TLS, so proceed.
			// Unread the startup message so we pick it up again when we do the startup.
			c.UnreadMsg(msg)
			return c, nil
		}
		return nil, fmt.Errorf("client did not request tls")
	} else if err != errWantTLS {
		return nil, err
	}

	log.Debug().Msg("client requested tls")
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
	c2 := tls.Server(c.conn, cfg)
	if err := c2.Handshake(); err != nil {
		c2.Close()
		log.Error().Err(err).Msg("client tls handshake failed")
		return nil, fmt.Errorf("client tls handshake failed: %v", err)
	}
	log.Debug().Msg("client tls handshake successful")
	return &peekableConn{conn: c2}, nil
}

func negotiateServerTLS(log zerolog.Logger, c *peekableConn, cfg *tls.Config) (*peekableConn, error) {
	if cfg == nil {
		// We don't want SSL so just don't request it.
		log.Debug().Msg("skipping tls negotiation with server since no TLS config was given")
		return c, nil
	}
	log.Debug().Msg("negotiating tls with server")
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
		log.Debug().Msg("server accepted tls request")
		c2 := tls.Client(c.conn, cfg)
		if err := c2.Handshake(); err != nil {
			log.Error().Err(err).Msg("server tls handshake failed")
			c2.Close()
			return nil, fmt.Errorf("server tls handshake failed: %v", err)
		}
		log.Debug().Msg("completed server tls handshake")
		return &peekableConn{conn: c2}, nil
	case 'N':
		log.Debug().Msg("server rejected tls request")
		return nil, fmt.Errorf("server rejected tls")
	case 'E':
		// ErrorMessage
		log.Debug().Msg("server responded with ErrorResponse to TLS request")
		c.UnreadByte(resp[0])
		var msg pgproto3.ErrorResponse
		if err := c.ReadInto(&msg); err != nil {
			log.Error().Err(err).Msg("could not parse ErrorResponse")
		}
		log.Error().Msgf("server tls negotiation error: %+v", msg)
		return nil, fmt.Errorf("could not negotiate tls with server: error %s: %s", msg.Code, msg.Message)
	default:
		return nil, fmt.Errorf("got unexpected response to tls request: %v", resp[0])
	}
}

const sslProtocolVersionNumber = 80877103

var errWantTLS = errors.New("client requested TLS")

func readStartupMsg(log zerolog.Logger, c *peekableConn) (pgproto3.FrontendMessage, error) {
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
	log.Debug().Uint32("code", code).Msg("got startup message code")
	switch code {
	case sslProtocolVersionNumber:
		return nil, errWantTLS
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

func readPassword(frontend io.ReadWriter) (string, error) {
	err := pgio.WriteMsg(frontend, &pgproto3.Authentication{
		Type: pgproto3.AuthTypeCleartextPassword,
	})
	if err != nil {
		return "", err
	}
	msg, err := pgio.ReadMsg(frontend)
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

func authenticateBackend(log zerolog.Logger, be *peekableConn, data *StartupMessage) error {
	for {
		msg, err := be.ReadMsg()
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
				if err := be.WriteMsg(pwdMsg); err != nil {
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
				if err := be.WriteMsg(pwdMsg); err != nil {
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

type peekableConn struct {
	conn   net.Conn
	unread []byte
}

func (c *peekableConn) ReadInto(msg pgio.PgMsg) error {
	raw, err := c.ReadMsg()
	if err != nil {
		return err
	}
	return msg.Decode(raw.Buf)
}

func (c *peekableConn) ReadMsg() (*pgio.RawMsg, error) {
	return pgio.ReadMsg(c)
}

func (c *peekableConn) UnreadMsg(msg pgio.PgMsg) {
	c.unread = msg.Encode(nil)
}

func (c *peekableConn) UnreadByte(b byte) {
	c.unread = []byte{b}
}

func (c *peekableConn) Read(p []byte) (n int, err error) {
	if len(c.unread) > 0 {
		n = copy(p, c.unread)
		c.unread = c.unread[n:]
		return n, nil
	}
	n, err = c.conn.Read(p)
	return
}

func (c *peekableConn) Write(p []byte) (n int, err error) {
	return c.conn.Write(p)
}

func (c *peekableConn) WriteMsg(msg pgio.PgMsg) error {
	return pgio.WriteMsg(c, msg)
}

type pgErr struct {
	msg pgproto3.ErrorResponse
}

func (e *pgErr) Error() string {
	return fmt.Sprintf("%s - code: %s", e.msg.Message, e.msg.Code)
}
