package reqtrack

import (
	"sync"

	"encore.dev/appruntime/exported/trace2"
)

type TraceStreamer interface {
	StreamTrace(trace2.Logger) error
}

type noopTraceStreamer struct{}

// StreamTrace implements TraceStreamer by consuming the trace log
// but not doing anything with it.
func (noopTraceStreamer) StreamTrace(log trace2.Logger) error {
	for {
		_, done := log.WaitAndClear()
		if done {
			return nil
		}
	}
}

func newLazyTrace(rt *RequestTracker) *lazyTraceInit {
	log := rt.trace.NewLogger()
	return &lazyTraceInit{rt: rt, log: log}
}

// lazyTraceInit is a lazily initialized trace logger.
// It is used to defer trace streaming until the trace is actually used.
type lazyTraceInit struct {
	rt       *RequestTracker
	log      trace2.Logger
	initOnce sync.Once
}

func (l *lazyTraceInit) MarkDone() {
	l.log.MarkDone()
	l.initStream()
}

func (l *lazyTraceInit) Logger() trace2.Logger {
	l.initStream()
	return l.log
}

func (l *lazyTraceInit) initStream() {
	l.initOnce.Do(func() {
		go func() {
			if err := l.rt.streamer.StreamTrace(l.log); err != nil {
				l.rt.rootLogger.Error().Err(err).Msg("failed to stream trace")
			}
		}()
	})
}
