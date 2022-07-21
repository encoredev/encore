package optracker

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/logrusorgru/aurora/v3"
)

func New(w io.Writer) *OpTracker {
	return &OpTracker{
		w: w,
	}
}

type OpTracker struct {
	mu      sync.Mutex
	ops     []*slowOp
	w       io.Writer
	nl      int // number of lines written
	started bool
	quit    bool
}

type OperationID int

const NoOperationID OperationID = -1

// AllDone marks all ops as done.
// This function is safe to call on a Nil OpTracker and will no-op in that case
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
	fmt.Fprint(t.w, "\u001b[0;0H\u001b[0J\n")

	nl := 0
	now := time.Now()

	// Sort ops by start time
	ops := make([]*slowOp, len(t.ops))
	copy(ops, t.ops)
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].start.Before(ops[j].start)
	})

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
				msg = aurora.Red(fmt.Sprintf(format+"Failed: %v", fail, o.msg, o.err))
			}
		case done && o.err == nil:
			msg = aurora.Green(fmt.Sprintf(format+"Done!", success, o.msg))
		case !done:
			msg = aurora.Cyan(fmt.Sprintf(format, spinner[o.spinIdx], o.msg))
			o.spinIdx = (o.spinIdx + 1) % len(spinner)
		}
		str := msg.String()
		fmt.Fprintf(t.w, "\u001b[2K%s\n", str)
		nl += strings.Count(str, "\n") + 1
	}
	t.nl = nl
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
