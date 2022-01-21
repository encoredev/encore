package pgproxy2

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"encr.dev/pkg/pgproxy2/internal/pgproto3"
	"github.com/rs/zerolog"
)

type (
	StartupMsg    = pgproto3.StartupMessage
	CancelRequest = pgproto3.CancelRequest
)

type LogicalConn interface {
	net.Conn
	Cancel(*CancelMessage) error
}

type SingleBackendProxy struct {
	Log             zerolog.Logger
	RequirePassword bool
	FrontendTLS     *tls.Config
	DialBackend     func(context.Context, *StartupMessage) (LogicalConn, error)

	gotBackend chan struct{} // closed when be is set

	mu sync.Mutex
	be LogicalConn
}

func (p *SingleBackendProxy) Serve(ctx context.Context, ln net.Listener) error {
	defer ln.Close()
	if p.gotBackend != nil {
		panic("SingleBackendProxy: Serve called twice")
	}
	p.gotBackend = make(chan struct{})

	go func() {
		select {
		case <-p.gotBackend:
		case <-time.After(10 * time.Minute):
			ln.Close()
		case <-ctx.Done():
			ln.Close()
		}
	}()

	var tempDelay time.Duration // how long to sleep on accept failure
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

		go p.handleConn(ctx, frontend)
		tempDelay = 0
	}
}

func (p *SingleBackendProxy) handleConn(ctx context.Context, frontend net.Conn) {
	defer frontend.Close()

	hello, err := ReadFrontendHello(ctx, &frontend, p.FrontendTLS, p.RequirePassword)
	if err != nil {
		return
	}

	p.mu.Lock()
	if p.be == nil {
		// setupBackend unlocks the mutex when ready.
		if err := p.setupBackend(ctx, frontend, hello); err != nil {
			p.Log.Error().Err(err).Msg("unable to setup backend")
		}
	} else {
		p.mu.Unlock()
		p.cancelRequest(ctx, hello)
	}
}

func (p *SingleBackendProxy) setupBackend(ctx context.Context, frontend net.Conn, hello FrontendHello) error {
	startup, ok := hello.(*StartupMessage)
	if !ok {
		p.mu.Unlock()
		return fmt.Errorf("first conn must send a startup message, not %T", hello)
	}
	var err error
	p.Log.Info().Msg("pgproxy: dialing backend")
	p.be, err = p.DialBackend(ctx, startup)
	p.mu.Unlock()

	if err2 := CompleteFrontendHandshake(ctx, frontend, err); err != nil {
		err = err2
	}
	if err != nil {
		return err
	}
	close(p.gotBackend)
	p.Log.Info().Msg("pgproxy: proxy established")
	if err := CopySteadyState(ctx, frontend, p.be); err != nil {
		p.Log.Error().Err(err).Msg("pgproxy: connection closed with error")
		return err
	}
	p.Log.Info().Msg("pgproxy: connection closed gracefully")
	return nil
}

func (p *SingleBackendProxy) cancelRequest(ctx context.Context, hello FrontendHello) {
	cancel, ok := hello.(*CancelMessage)
	if !ok {
		p.Log.Error().Msgf("follow-up conn sent %T, expected cancel request", hello)
		return
	}
	p.Log.Info().Msg("received cancel request")
	if err := p.be.Cancel(cancel); err != nil {
		p.Log.Error().Err(err).Msg("unable to send cancel request")
	}
}
