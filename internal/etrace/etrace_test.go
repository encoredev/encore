package etrace

import (
	"context"
	"testing"
)

// Tracing is off unless a tracer is set, so a traced step must cost nothing then.
func TestUntracedStepsDoNoWork(t *testing.T) {
	ctx := context.Background()
	step := func(context.Context) {}
	if n := testing.AllocsPerRun(100, func() { Sync0(ctx, "cat", "name", step) }); n != 0 {
		t.Errorf("Sync0 with no tracer allocates %v times per call, want 0", n)
	}
	if n := testing.AllocsPerRun(100, func() { Async0(ctx, "cat", "name", step) }); n != 0 {
		t.Errorf("Async0 with no tracer allocates %v times per call, want 0", n)
	}
}

func TestGoroutineID(t *testing.T) {
	main := goroutineID()
	if main <= 0 {
		t.Fatalf("goroutineID() = %d, want > 0", main)
	}
	if again := goroutineID(); again != main {
		t.Fatalf("goroutineID() changed within one goroutine: %d then %d", main, again)
	}
	other := make(chan int64)
	go func() { other <- goroutineID() }()
	if id := <-other; id == main || id <= 0 {
		t.Fatalf("goroutineID() in another goroutine = %d, main = %d", id, main)
	}
}
