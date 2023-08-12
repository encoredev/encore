package redis

import (
	mathrand "math/rand" // nosemgrep
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/cockroachdb/errors"
	"go4.org/syncutil"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

type Server struct {
	startOnce syncutil.Once
	mini      *miniredis.Miniredis
	cleanup   *time.Ticker
	quit      chan struct{}
	addr      string
}

const tickInterval = 1 * time.Second

func New() *Server {
	return &Server{
		mini: miniredis.NewMiniRedis(),
		quit: make(chan struct{}),
	}
}

func (s *Server) Start() error {
	return s.startOnce.Do(func() error {
		if err := s.mini.Start(); err != nil {
			return errors.Wrap(err, "failed to start redis server")
		}
		s.addr = s.mini.Addr()
		s.cleanup = time.NewTicker(tickInterval)
		go s.doCleanup()
		return nil
	})
}
func (s *Server) Stop() {
	s.mini.Close()
	s.cleanup.Stop()
	close(s.quit)
}

func (s *Server) Miniredis() *miniredis.Miniredis {
	return s.mini
}

func (s *Server) Addr() string {
	// Ensure the server has been started
	if err := s.Start(); err != nil {
		panic(err)
	}
	return s.addr
}

func (s *Server) doCleanup() {
	var acc time.Duration
	const cleanupInterval = 15 * time.Second

	for {
		select {
		case <-s.quit:
			return
		case <-s.cleanup.C:
		}
		s.mini.FastForward(tickInterval)

		// Clean up keys every so often
		acc += tickInterval
		if acc > cleanupInterval {
			acc -= cleanupInterval
			s.clearKeys()
		}
	}
}

// clearKeys clears random keys to get the redis server
// down to 100 persisted keys, as a simple way to bound
// the max memory usage.
func (s *Server) clearKeys() {
	const maxKeys = 100
	keys := s.mini.Keys()
	if n := len(keys); n > maxKeys {
		toDelete := n - maxKeys
		deleted := 0
		for deleted < toDelete {
			id := mathrand.Intn(len(keys))
			if keys[id] != "" {
				s.mini.Del(keys[id])
				keys[id] = "" // mark it as deleted
				deleted++
			}
		}
	}
}

// IsUsed reports whether the application uses redis at all.
func IsUsed(md *meta.Data) bool {
	return len(md.CacheClusters) > 0
}
