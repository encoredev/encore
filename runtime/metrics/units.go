package metrics

import "time"

// TODO Document

type Value interface {
	uint64 | int64 | float64 | time.Duration
}
