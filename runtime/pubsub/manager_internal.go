package pubsub

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/api"
	"encore.dev/appruntime/config"
	"encore.dev/appruntime/reqtrack"
	"encore.dev/appruntime/testsupport"
	"encore.dev/pubsub/internal/gcp"
	"encore.dev/pubsub/internal/nsq"
)

type Manager struct {
	ctx        context.Context
	cancelCtx  func()
	cfg        *config.Config
	rt         *reqtrack.RequestTracker
	ts         *testsupport.Manager
	rootLogger zerolog.Logger
	gcp        *gcp.Manager
	nsq        *nsq.Manager

	publishCounter uint64

	outstanding *outstandingMessageTracker
}

func NewManager(cfg *config.Config, rt *reqtrack.RequestTracker, ts *testsupport.Manager, server *api.Server, rootLogger zerolog.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	gcpMgr := gcp.NewManager(ctx, cfg, server)
	nsqMgr := nsq.NewManager(ctx, cfg, rt)
	return &Manager{
		ctx:         ctx,
		cancelCtx:   cancel,
		cfg:         cfg,
		rt:          rt,
		ts:          ts,
		rootLogger:  rootLogger,
		gcp:         gcpMgr,
		nsq:         nsqMgr,
		outstanding: newOutstandingMessageTracker(),
	}
}

func (mgr *Manager) Shutdown(force context.Context) {
	mgr.cancelCtx()
	mgr.outstanding.ArmForShutdown()

	select {
	case <-mgr.outstanding.Done():
	case <-force.Done():
	}
}

// outstandingMessageTracker tracks the number of outstanding messages.
// Once Shutdown() has been called, the next time the number of outstanding
// messages reaches zero (or if it's already zero), the Done() channel is closed.
type outstandingMessageTracker struct {
	active int64

	closeOnce sync.Once
	done      chan struct{}

	mu    sync.Mutex
	armed bool
}

func newOutstandingMessageTracker() *outstandingMessageTracker {
	return &outstandingMessageTracker{
		done: make(chan struct{}),
	}
}

func (t *outstandingMessageTracker) Inc() {
	atomic.AddInt64(&t.active, 1)
}

func (t *outstandingMessageTracker) Dec() {
	val := atomic.AddInt64(&t.active, -1)
	if val < 0 {
		panic("outstandingMessageTracker: active < 0")
	}
	if val == 0 {
		t.mu.Lock()
		defer t.mu.Unlock()
		if t.armed {
			t.markDone()
		}
	}
}

func (t *outstandingMessageTracker) ArmForShutdown() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.armed = true

	// If we're already at zero, mark it as done right away.
	if atomic.LoadInt64(&t.active) == 0 {
		t.markDone()
	}
}

// markDone marks the tracker as being done.
func (t *outstandingMessageTracker) markDone() {
	t.closeOnce.Do(func() { close(t.done) })
}

func (t *outstandingMessageTracker) Done() <-chan struct{} {
	return t.done
}
