package pgproxy

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"encr.dev/pkg/fns"
)

type LogicalConn interface {
	net.Conn
	Cancel(*CancelData) error
}

type HelloData interface {
	hello()
}

type StartupData struct {
	Raw      *pgproto3.StartupMessage
	Database string
	Username string
	Password string // may be empty if RequirePassword is false
}

type CancelData struct {
	Raw *pgproto3.CancelRequest
}

func (*StartupData) hello() {}
func (*CancelData) hello()  {}

type SingleBackendProxy struct {
	Log             zerolog.Logger
	RequirePassword bool
	FrontendTLS     *tls.Config
	DialBackend     func(context.Context, *StartupData) (LogicalConn, error)

	gotBackend chan struct{} // closed when first connection is received

	mu      sync.Mutex
	keyData map[backendKey]LogicalConn
}

// backendKey identifies a backend connection for routing cancel requests.
// It mirrors pgproto3.BackendKeyData, which is not comparable since the
// secret key is variable-length as of protocol version 3.2.
type backendKey struct {
	processID uint32
	secretKey string
}

type DatabaseNotFoundError struct {
	Database string
}

func (e DatabaseNotFoundError) Error() string {
	return fmt.Sprintf("database %s not found", e.Database)
}

func (p *SingleBackendProxy) Serve(ctx context.Context, ln net.Listener) error {
	defer fns.CloseIgnore(ln)
	if p.gotBackend != nil {
		panic("SingleBackendProxy: Serve called twice")
	}
	p.gotBackend = make(chan struct{})

	go func() {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		select {
		case <-p.gotBackend:
		case <-ctx.Done():
			_ = ln.Close()
		}
	}()

	var tempDelay time.Duration // how long to sleep on accept failure
	gotBackend := false
	for {
		frontend, err := ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Printf("pgproxy: accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return fmt.Errorf("pgproxy: could not accept: %v", err)
		}

		if !gotBackend {
			close(p.gotBackend)
			gotBackend = true
		}

		go p.ProxyConn(ctx, frontend)
		tempDelay = 0
	}
}

func (p *SingleBackendProxy) ProxyConn(ctx context.Context, client net.Conn) {
	defer fns.CloseIgnore(client)

	cl, err := SetupClient(client, &ClientConfig{
		TLS:          p.FrontendTLS,
		WantPassword: p.RequirePassword,
	})
	if err != nil {
		p.Log.Error().Err(err).Msg("unable to setup frontend")
		return
	}

	switch data := cl.Hello.(type) {
	case *StartupData:
		if err := p.doRunProxy(ctx, cl); err != nil {
			p.Log.Error().Err(err).Msg("unable to run backend proxy")
		}
	case *CancelData:
		p.cancelRequest(ctx, data)
	default:
		p.Log.Error().Msgf("unknown hello message type: %T", data)
	}
}

func (p *SingleBackendProxy) doRunProxy(ctx context.Context, cl *Client) error {
	startup := cl.Hello.(*StartupData)
	server, err := p.DialBackend(ctx, startup)
	if err != nil {
		cl.Backend.Send(&pgproto3.ErrorResponse{
			Severity: "FATAL",
			Message:  err.Error(),
		})
		_ = cl.Backend.Flush()
		return err
	}
	defer fns.CloseIgnore(server)

	fe := pgproto3.NewFrontend(server, server)
	log.Trace().Msg("successfully setup server connection")

	err = AuthenticateClient(cl.Backend)
	if err != nil {
		log.Error().Err(err).Msg("unable to authenticate client")
		return err
	}
	log.Trace().Msg("successfully authenticated client")

	key, err := FinalizeInitialHandshake(cl.Backend, fe)
	if err != nil {
		log.Error().Err(err).Msg("unable to finalize handshake")
		return err
	}

	if key != nil {
		p.mu.Lock()
		if p.keyData == nil {
			p.keyData = make(map[backendKey]LogicalConn)
		}
		p.keyData[backendKey{processID: key.ProcessID, secretKey: string(key.SecretKey)}] = server
		p.mu.Unlock()
	}

	err = CopySteadyState(cl.Backend, fe)
	if err != nil {
		log.Error().Err(err).Msg("unable to copy steady state")
		return err
	}

	return nil
}

type ServerConfig struct {
	TLS     *tls.Config // nil indicates no TLS
	Startup *StartupData
}

// SetupServer sets up a frontend connected to the given server.
func SetupServer(server net.Conn, cfg *ServerConfig) (*pgproto3.Frontend, error) {
	fe, err := serverTLSNegotiate(server, cfg.TLS)
	if err != nil {
		return nil, err
	}

	raw := cfg.Startup.Raw
	raw.Parameters["database"] = cfg.Startup.Database
	raw.Parameters["user"] = cfg.Startup.Username

	log.Trace().Msg("sending startup message to server")
	fe.Send(raw)
	if err := fe.Flush(); err != nil {
		return nil, fmt.Errorf("unable to send startup message: %v", err)
	}

	// Handle authentication
	for {
		msg, err := fe.Receive()
		if err != nil {
			return nil, fmt.Errorf("unexpected message from server: %v", err)
		}

		switch msg := msg.(type) {
		case *pgproto3.ErrorResponse:
			return nil, pgconn.ErrorResponseToPgError(msg)

		case pgproto3.AuthenticationResponseMessage:
			if len(cfg.Startup.Password) == 0 {
				if _, ok := msg.(*pgproto3.AuthenticationOk); !ok {
					return nil, fmt.Errorf("backend requested authentication but no password given")
				}
			}

			switch msg := msg.(type) {
			case *pgproto3.AuthenticationOk:
				// We're done!
				return fe, nil

			case *pgproto3.AuthenticationCleartextPassword:
				fe.Send(&pgproto3.PasswordMessage{Password: cfg.Startup.Password})
				if err := fe.Flush(); err != nil {
					return nil, err
				}

			case *pgproto3.AuthenticationMD5Password:
				password := computeMD5(cfg.Startup.Username, cfg.Startup.Password, msg.Salt)
				fe.Send(&pgproto3.PasswordMessage{Password: password})
				if err := fe.Flush(); err != nil {
					return nil, err
				}

			case *pgproto3.AuthenticationSASL:
				if err := scramAuth(fe, cfg.Startup.Password, msg.AuthMechanisms); err != nil {
					return nil, err
				}

			default:
				return nil, fmt.Errorf("unsupported auth message: %T", msg)
			}

		default:
			return nil, fmt.Errorf("unexpected message type from backend: %T", msg)
		}
	}
}

func serverTLSNegotiate(server net.Conn, tlsConfig *tls.Config) (*pgproto3.Frontend, error) {
	if tlsConfig == nil {
		return pgproto3.NewFrontend(server, server), nil
	}

	log.Trace().Msg("negotiating tls with server")
	sslRequest, err := (&pgproto3.SSLRequest{}).Encode(nil)
	if err != nil {
		return nil, err
	}
	if _, err := server.Write(sslRequest); err != nil {
		return nil, err
	}
	// Read the TLS response. Read exactly one byte, without buffering,
	// since the TLS handshake follows directly on the same connection.
	resp := make([]byte, 1)
	if _, err := io.ReadFull(server, resp); err != nil {
		return nil, err
	}
	switch resp[0] {
	case 'S':
		log.Trace().Msg("server accepted tls request")
		tlsConn := tls.Client(server, tlsConfig)
		if err := tlsConn.Handshake(); err != nil {
			log.Error().Err(err).Msg("server tls handshake failed")
			_ = tlsConn.Close()
			return nil, fmt.Errorf("server tls handshake failed: %v", err)
		}
		log.Trace().Msg("completed server tls handshake")

		// Return a new backend that wraps the tls conn.
		return pgproto3.NewFrontend(tlsConn, tlsConn), nil
	case 'N':
		log.Trace().Msg("server rejected tls request")
		return nil, fmt.Errorf("server rejected tls")
	case 'E':
		// ErrorMessage: we've already consumed the message type byte,
		// so put it back in front of the rest of the message.
		fe := pgproto3.NewFrontend(io.MultiReader(bytes.NewReader(resp), server), server)
		msg, err := fe.Receive()
		if err != nil {
			return nil, err
		}
		errMsg, ok := msg.(*pgproto3.ErrorResponse)
		if !ok {
			return nil, fmt.Errorf("got unexpected response to tls request: %T", msg)
		}
		log.Error().Msgf("server tls negotiation error: %+v", errMsg)
		return nil, fmt.Errorf("could not negotiate tls with server: error %s: %s", errMsg.Code, errMsg.Message)
	default:
		return nil, fmt.Errorf("got unexpected response to tls request: %v", resp[0])
	}
}

type ClientConfig struct {
	// TLS, if non-nil, indicates we support TLS connections.
	TLS *tls.Config

	// WantPassword, if true, indicates we want to capture
	// the password sent by the frontend.
	WantPassword bool
}

type Client struct {
	Backend *pgproto3.Backend
	Hello   HelloData
}

// SetupClient sets up a backend connected to the given client.
// If tlsConfig is non-nil it negotiates TLS if requested by the client.
//
// On successful startup the returned message is either *pgproto3.StartupMessage or *pgproto3.CancelRequest.
//
// It is up to the caller to authenticate the client using AuthenticateClient.
func SetupClient(client net.Conn, cfg *ClientConfig) (*Client, error) {
	log.Trace().Msg("setting up client backend")
	be, msg, err := clientTLSNegotiate(client, cfg.TLS)
	if err != nil {
		return nil, err
	}

	if cancel, ok := msg.(*pgproto3.CancelRequest); ok {
		return &Client{
			Backend: be,
			Hello:   &CancelData{Raw: cancel},
		}, nil
	}

	startup := msg.(*pgproto3.StartupMessage)
	hello := &StartupData{
		Raw:      startup,
		Database: startup.Parameters["database"],
		Username: startup.Parameters["user"],
	}
	if cfg.WantPassword {
		be.Send(&pgproto3.AuthenticationCleartextPassword{})
		if err := be.Flush(); err != nil {
			return nil, err
		}
		msg, err := be.Receive()
		if err != nil {
			return nil, err
		}
		passwd, ok := msg.(*pgproto3.PasswordMessage)
		if !ok {
			return nil, fmt.Errorf("expected PasswordMessage, got %T", msg)
		}
		hello.Password = passwd.Password
	}

	return &Client{
		Backend: be,
		Hello:   hello,
	}, nil
}

func clientTLSNegotiate(client net.Conn, tlsConfig *tls.Config) (*pgproto3.Backend, pgproto3.FrontendMessage, error) {
	log.Trace().Msg("negotiating TLS with client")
	backend := pgproto3.NewBackend(client, client)
	hasTLS := false

StartupMessageLoop:
	for {
		startup, err := backend.ReceiveStartupMessage()
		if err != nil {
			return nil, nil, err
		}
		switch startup := startup.(type) {
		case *pgproto3.SSLRequest:
			if hasTLS {
				return nil, nil, fmt.Errorf("received duplicate SSL request")
			} else if tlsConfig == nil {
				// We got an SSL request but we don't want to use TLS
				if _, err := client.Write([]byte{'N'}); err != nil {
					return nil, nil, err
				}
				continue StartupMessageLoop
			}

			if _, err := client.Write([]byte{'S'}); err != nil {
				return nil, nil, err
			}
			tlsConn := tls.Server(client, tlsConfig)
			if err := tlsConn.Handshake(); err != nil {
				_ = tlsConn.Close()
				return nil, nil, fmt.Errorf("client tls handshake failed: %v", err)
			}
			log.Trace().Msg("client tls handshake successful")

			// The TLS handshake was successful.
			// Create a new backend that reads from the now-encrypted TLS connection.
			hasTLS = true
			backend = pgproto3.NewBackend(tlsConn, tlsConn)
		case *pgproto3.CancelRequest, *pgproto3.StartupMessage:
			// Startup complete.
			log.Debug().Msg("startup completed")
			return backend, startup, nil
		case *pgproto3.GSSEncRequest:
			// We got a GSSAPI encryption request but we don't support it.
			if _, err := client.Write([]byte{'N'}); err != nil {
				return nil, nil, err
			}
			continue StartupMessageLoop
		}
	}
}

type AuthData struct {
	Username string
	Password string
}

// AuthenticateClient tells the client they've successfully authenticated.
func AuthenticateClient(be *pgproto3.Backend) error {
	_ = be.SetAuthType(pgproto3.AuthTypeOk)
	be.Send(&pgproto3.AuthenticationOk{})
	return be.Flush()
}

func computeMD5(username, password string, salt [4]byte) string {
	// concat('md5', md5(concat(md5(concat(password, username)), random-salt)))

	// s1 := md5(concat(password, username))
	// nosemgrep
	s1 := md5.Sum([]byte(password + username))
	// s2 := md5(concat(s1, random-salt))
	// nosemgrep
	s2 := md5.Sum([]byte(hex.EncodeToString(s1[:]) + string(salt[:])))
	return "md5" + hex.EncodeToString(s2[:])
}

func SendCancelRequest(conn io.ReadWriter, req *pgproto3.CancelRequest) error {
	buf, err := req.Encode(nil)
	if err != nil {
		return err
	}
	_, err = conn.Write(buf)
	if err != nil {
		return err
	}

	_, err = conn.Read(buf)
	if err != io.EOF {
		return err
	}
	return nil
}

// FinalizeInitialHandshake completes the handshake between client and server,
// snooping the BackendKeyData from the server if sent.
// It is nil if the server did not send any backend key data.
func FinalizeInitialHandshake(client *pgproto3.Backend, server *pgproto3.Frontend) (*pgproto3.BackendKeyData, error) {
	var keyData *pgproto3.BackendKeyData

	// Read messages from backend until we get ReadyForQuery
	for {
		msg, err := server.Receive()
		if err != nil {
			return nil, fmt.Errorf("pgproxy: cannot read from backend: %v", err)
		}
		client.Send(msg)
		if err := client.Flush(); err != nil {
			return nil, fmt.Errorf("pgproxy: could not write to frontend: %v", err)
		}

		switch msg := msg.(type) {
		case *pgproto3.BackendKeyData:
			// Make a copy; this object is only valid until the next call to Receive()
			copy := *msg
			copy.SecretKey = slices.Clone(msg.SecretKey)
			keyData = &copy

		case *pgproto3.ReadyForQuery:
			// Handshake completed
			return keyData, nil
		}
	}
}

// CopySteadyState copies messages back and forth after the initial handshake.
func CopySteadyState(client *pgproto3.Backend, server *pgproto3.Frontend) error {
	errChan := make(chan error, 2)
	msgAck := make(chan struct{})
	done := make(chan struct{})
	defer close(done)

	runTask := func(task func() error) {
		go func() {
			errChan <- task()
		}()
	}
	clientMsgs := make(chan pgproto3.FrontendMessage)

	runTask(func() (err error) {
		for {
			msg, err := server.Receive()
			if errors.Is(err, io.ErrUnexpectedEOF) {
				log.Trace().Msg("pgproxy: connection terminated by server, closing connection")
				return nil
			} else if err != nil {
				return err
			}
			client.Send(msg)
			if err := client.Flush(); err != nil {
				return err
			}
			select {
			case <-done:
				return nil
			default:
			}
		}
	})

	runTask(func() error {
		for {
			msg, err := client.Receive()
			if err != nil {
				return err
			}
			select {
			case <-done:
				return nil
			case clientMsgs <- msg:
			}
			select {
			case <-done:
				return nil
			case <-msgAck:
			}
		}
	})

	// Copy from client to server.
	for {
		select {
		case err := <-errChan:
			return err
		case msg := <-clientMsgs:
			server.Send(msg)
			err := server.Flush()

			if err != nil {
				return err
			}
			if _, ok := msg.(*pgproto3.Terminate); ok {
				log.Trace().Msg("received terminate from client, closing connection")
				return nil
			}
			msgAck <- struct{}{}
		}
	}
}

func (p *SingleBackendProxy) cancelRequest(ctx context.Context, cancel *CancelData) {
	p.Log.Trace().Msg("received cancel request")
	key := backendKey{
		processID: cancel.Raw.ProcessID,
		secretKey: string(cancel.Raw.SecretKey),
	}
	p.mu.Lock()
	conn, ok := p.keyData[key]
	p.mu.Unlock()

	if ok {
		if err := conn.Cancel(cancel); err != nil {
			p.Log.Error().Err(err).Msg("unable to send cancel request")
		}
	} else {
		p.Log.Error().Msg("could not find backend key data")
	}
}
