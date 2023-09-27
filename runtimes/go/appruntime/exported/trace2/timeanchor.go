package trace2

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// NewTimeAnchor constructs a new TimeAnchor.
func NewTimeAnchor(nano int64, real time.Time) TimeAnchor {
	return TimeAnchor{nano: nano, real: real}
}

// NewTimeAnchorNow constructs a new TimeAnchor based on the current time.
func NewTimeAnchorNow() TimeAnchor {
	now := time.Now()
	nano := nanotime()
	return NewTimeAnchor(nano, now)
}

// TimeAnchor represents a mapping between nanotime() timestamps
// and real-world time.Time instants.
type TimeAnchor struct {
	nano int64
	real time.Time
}

// ToReal converts a nanotime() timestamp to a real-world time.Time instant.
func (ta TimeAnchor) ToReal(nano int64) time.Time {
	return ta.real.Add(time.Duration(nano - ta.nano))
}

// MarshalText marshals the anchor as text. It never fails.
func (ta TimeAnchor) MarshalText() ([]byte, error) {
	nano := strconv.FormatInt(ta.nano, 10)
	return []byte(nano + " " + ta.real.Format(time.RFC3339Nano)), nil
}

// UnmarshalText unmarshals the anchor from text.
func (ta *TimeAnchor) UnmarshalText(text []byte) error {
	a, b, ok := strings.Cut(string(text), " ")
	if !ok {
		return fmt.Errorf("invalid time anchor format: %q", text)
	}
	nano, err := strconv.ParseInt(a, 10, 64)
	if err != nil {
		return err
	}
	real, err := time.Parse(time.RFC3339Nano, b)
	if err != nil {
		return err
	}

	ta.nano = nano
	ta.real = real
	return nil
}
