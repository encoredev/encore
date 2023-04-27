package optracker

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/logrusorgru/aurora/v3"

	"encr.dev/pkg/ansi"
	"encr.dev/pkg/errlist"
	daemonpb "encr.dev/proto/encore/daemon"
)

type OutputStream interface {
	Send(*daemonpb.CommandMessage) error
}

func New(w io.Writer, stream OutputStream) *OpTracker {
	return &OpTracker{
		w:      w,
		stream: stream,
	}
}

type OpTracker struct {
	mu          sync.Mutex
	ops         []*slowOp
	w           io.Writer
	started     bool
	quit        bool
	savedCursor sync.Once
	stream      OutputStream
}

type OperationID int

const NoOperationID OperationID = -1

// AllDone marks all ops as done.
// This function is safe to call on a Nil OpTracker.
func (t *OpTracker) AllDone() {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	for _, o := range t.ops {
		if o.done.IsZero() || o.done.After(now) {
			o.done = now
		}
		if o.start.After(now) {
			o.start = now
		}
	}
	t.quit = true
	t.refresh()
}

// Add creates a new item on the operations tracker returning the ID for that op.
// minStart is the time at which the tracker will start to show the task as in progress.
//
// This function is safe to call on a Nil OpTracker and will no-op in that case
func (t *OpTracker) Add(msg string, minStart time.Time) OperationID {
	if t == nil {
		return NoOperationID
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	id := OperationID(len(t.ops))

	start := time.Now()
	if start.Before(minStart) {
		start = minStart
	}
	op := &slowOp{msg: msg, start: start}
	t.ops = append(t.ops, op)
	t.refresh()

	if !t.started {
		go t.spin()
		t.started = true
	}

	return id
}

// Done marks the given operation as done
//
// This function is safe to call on a Nil OpTracker and will no-op in that case
func (t *OpTracker) Done(id OperationID, minDuration time.Duration) {
	if t == nil || id == NoOperationID {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	o := t.ops[id]

	done := time.Now()
	if a := o.start.Add(minDuration); a.After(done) {
		done = a
	}
	o.done = done
	t.refresh()
}

var ErrCanceled = errors.New("operation canceled")

// Fail marks the operation as failed with the given error
//
// This function is safe to call on a Nil OpTracker and will no-op in that case
func (t *OpTracker) Fail(id OperationID, err error) {
	if t == nil || id == NoOperationID {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.ops[id].done.IsZero() {
		return
	}
	t.ops[id].err = err
	t.ops[id].done = time.Now()
	t.refresh()
}

// Cancel marks the operation as canceled.
// It is equivalent to t.Fail(id, ErrCanceled).
func (t *OpTracker) Cancel(id OperationID) {
	t.Fail(id, ErrCanceled)
}

// refresh refreshes the display by writing to t.w.
// The mutex must be held by the caller.
func (t *OpTracker) refresh() {
	t.savedCursor.Do(func() {
		fmt.Fprint(t.w, ansi.SaveCursorPosition)
	})
	fmt.Fprint(t.w, ansi.RestoreCursorPosition+ansi.ClearScreen(ansi.CursorToBottom))

	now := time.Now()

	// Sort ops by start time
	ops := make([]*slowOp, len(t.ops))
	copy(ops, t.ops)
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].start.Before(ops[j].start)
	})

	var errlistToSend *errlist.List
	for _, o := range ops {
		started := o.start.Before(now)
		done := !o.done.IsZero() && o.done.Before(now)
		if !started && !done {
			continue
		}

		var msg aurora.Value
		format := "  %s %s... "
		switch {
		case done && o.err != nil:
			if errors.Is(o.err, ErrCanceled) {
				msg = aurora.Yellow(fmt.Sprintf(format+"Canceled", canceled, o.msg))
			} else {
				if errlist := errlist.Convert(o.err); errlist != nil {
					errlistToSend = errlist
					if len(errlist.List) > 0 {
						msg = aurora.Red(fmt.Sprintf(format+"Failed: %v", fail, o.msg, errlist.List[0].Title()))
					} else {
						msg = aurora.Red(fmt.Sprintf(format+"Failed: %v", fail, o.msg, errlist))
					}
				} else {
					msg = aurora.Red(fmt.Sprintf(format+"Failed: %v", fail, o.msg, o.err))
				}
			}
		case done && o.err == nil:
			msg = aurora.Green(fmt.Sprintf(format+"Done!", success, o.msg))
		case !done:
			msg = aurora.Cyan(fmt.Sprintf(format, spinner[o.spinIdx], o.msg))
			o.spinIdx = (o.spinIdx + 1) % len(spinner)
		}
		str := msg.String()
		fmt.Fprintf(t.w, "%s%s%s\n",
			ansi.MoveCursorLeft(1000),
			ansi.ClearLine(ansi.WholeLine),
			str,
		)
	}
	if errlistToSend != nil {
		// We sent this after we clear and repaint the screen
		errlistToSend.SendToStream(t.stream)
	}
}

func (t *OpTracker) spin() {
	refresh := 100 * time.Millisecond
	if runtime.GOOS == "windows" {
		// Window's terminal is quite slow at rendering.
		// Reduce the refresh rate to avoid excessive flickering.
		refresh = 250 * time.Millisecond
	}
	for {
		time.Sleep(refresh)
		(func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			if !t.quit {
				t.refresh()
			}
		})()
	}

}

type slowOp struct {
	msg     string
	err     error
	spinIdx int
	start   time.Time
	done    time.Time
}

var (
	success  = "✔"
	fail     = "❌"
	canceled = "⚠️"
	spinner  = []string{"⠋", "⠙", "⠚", "⠒", "⠂", "⠂", "⠒", "⠲", "⠴", "⠦", "⠖", "⠒", "⠐", "⠐", "⠒", "⠓", "⠋"}
)
