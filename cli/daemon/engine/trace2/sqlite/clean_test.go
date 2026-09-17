package sqlite

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// seedTraces inserts n traces of one bytesPer-sized event each, oldest first.
func seedTraces(t *testing.T, s *Store, appID string, n, bytesPer int) {
	t.Helper()
	payload := strings.Repeat("x", bytesPer)
	for i := 0; i < n; i++ {
		traceID := fmt.Sprintf("%s-trace-%04d", appID, i)
		if _, err := s.db.Exec(
			`INSERT INTO trace_event (app_id, trace_id, span_id, event_data) VALUES (?, ?, ?, ?)`,
			appID, traceID, "span", payload); err != nil {
			t.Fatalf("insert event: %v", err)
		}
		if _, err := s.db.Exec(
			`INSERT INTO trace_span_index (trace_id, span_id, app_id, span_type, has_response)
			 VALUES (?, ?, ?, 1, true)`, traceID, "span", appID); err != nil {
			t.Fatalf("insert span: %v", err)
		}
	}
}

func appStats(t *testing.T, s *Store, appID string) (traces int, bytes int64) {
	t.Helper()
	err := s.db.QueryRow(
		`SELECT COUNT(DISTINCT trace_id), COALESCE(SUM(octet_length(event_data)), 0)
		 FROM trace_event WHERE app_id = ?`, appID).Scan(&traces, &bytes)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	return
}

func TestDoCleanTrimsToByteBudget(t *testing.T) {
	s := newTestStore(t)
	seedTraces(t, s, "app", 200, 1024)
	const budget = 50 * 1024

	for i := 0; i < 10; i++ {
		if err := s.DoClean(context.Background(), CleanConfig{MaxBytesPerApp: budget, MinTracesKept: 1, BatchSize: 1000}); err != nil {
			t.Fatalf("clean: %v", err)
		}
	}

	traces, bytes := appStats(t, s, "app")
	if bytes > budget {
		t.Errorf("app still over budget: %d bytes > %d", bytes, budget)
	}
	if traces == 0 {
		t.Error("everything was deleted")
	}
	// The survivors must be the newest ones.
	var oldest string
	if err := s.db.QueryRow(`SELECT MIN(trace_id) FROM trace_event WHERE app_id = ?`, "app").Scan(&oldest); err != nil {
		t.Fatalf("oldest: %v", err)
	}
	if oldest < "app-trace-0100" {
		t.Errorf("kept an old trace %q; expected only the newest to survive", oldest)
	}
	t.Logf("kept %d traces / %d bytes (budget %d)", traces, bytes, budget)
}

func TestDoCleanKeepsMinimumRegardlessOfSize(t *testing.T) {
	s := newTestStore(t)
	// Every trace on its own exceeds the budget.
	seedTraces(t, s, "app", 20, 4096)
	const budget = 100

	for i := 0; i < 5; i++ {
		if err := s.DoClean(context.Background(), CleanConfig{MaxBytesPerApp: budget, MinTracesKept: 3, BatchSize: 1000}); err != nil {
			t.Fatalf("clean: %v", err)
		}
	}

	traces, _ := appStats(t, s, "app")
	if traces != 3 {
		t.Errorf("got %d traces, want the 3 the floor guarantees", traces)
	}
}

func TestDoCleanLeavesAppsUnderBudgetAlone(t *testing.T) {
	s := newTestStore(t)
	seedTraces(t, s, "small", 10, 1024)
	seedTraces(t, s, "big", 200, 1024)
	const budget = 50 * 1024

	for i := 0; i < 10; i++ {
		if err := s.DoClean(context.Background(), CleanConfig{MaxBytesPerApp: budget, MinTracesKept: 1, BatchSize: 1000}); err != nil {
			t.Fatalf("clean: %v", err)
		}
	}

	if traces, _ := appStats(t, s, "small"); traces != 10 {
		t.Errorf("under-budget app lost traces: got %d, want 10", traces)
	}
	if _, bytes := appStats(t, s, "big"); bytes > budget {
		t.Errorf("over-budget app not trimmed: %d bytes", bytes)
	}
}

func TestDoCleanDeletesSpanRowsToo(t *testing.T) {
	s := newTestStore(t)
	seedTraces(t, s, "app", 200, 1024)

	for i := 0; i < 10; i++ {
		if err := s.DoClean(context.Background(), CleanConfig{MaxBytesPerApp: 50 * 1024, MinTracesKept: 1, BatchSize: 1000}); err != nil {
			t.Fatalf("clean: %v", err)
		}
	}

	var events, spans int
	s.db.QueryRow(`SELECT COUNT(DISTINCT trace_id) FROM trace_event`).Scan(&events)
	s.db.QueryRow(`SELECT COUNT(DISTINCT trace_id) FROM trace_span_index`).Scan(&spans)
	if events != spans {
		t.Errorf("trace_event has %d traces but trace_span_index has %d; they must be deleted together", events, spans)
	}
}

func TestDoCleanEvictsLeastRecentAppsOverTotalBudget(t *testing.T) {
	s := newTestStore(t)
	// Seeded oldest-first, so "newest" has the highest event ids.
	seedTraces(t, s, "oldest", 10, 1024)
	seedTraces(t, s, "middle", 10, 1024)
	seedTraces(t, s, "newest", 10, 1024)

	// 30 KiB across three apps, with room for two.
	cfg := CleanConfig{MaxBytesPerApp: 1 << 20, MaxBytesTotal: 25 * 1024, MinTracesKept: 1, BatchSize: 1000}
	if err := s.DoClean(context.Background(), cfg); err != nil {
		t.Fatalf("clean: %v", err)
	}
	for app, want := range map[string]int{"oldest": 0, "middle": 10, "newest": 10} {
		if traces, _ := appStats(t, s, app); traces != want {
			t.Errorf("app %q: got %d traces, want %d", app, traces, want)
		}
	}

	// A budget smaller than any single app must still leave the active one.
	cfg.MaxBytesTotal = 1
	if err := s.DoClean(context.Background(), cfg); err != nil {
		t.Fatalf("clean: %v", err)
	}
	if traces, _ := appStats(t, s, "newest"); traces == 0 {
		t.Error("evicted the most recently active app")
	}
}

func TestDoCleanDropsUnknownApps(t *testing.T) {
	s := newTestStore(t)
	seedTraces(t, s, "known", 5, 1024)
	seedTraces(t, s, "gone", 5, 1024)

	cfg := CleanConfig{MaxBytesPerApp: 1 << 20, MinTracesKept: 1, BatchSize: 1000,
		KnownApps: func() ([]string, error) { return []string{"known"}, nil }}
	if err := s.DoClean(context.Background(), cfg); err != nil {
		t.Fatalf("clean: %v", err)
	}
	if traces, _ := appStats(t, s, "gone"); traces != 0 {
		t.Errorf("unknown app kept %d traces", traces)
	}
	if traces, _ := appStats(t, s, "known"); traces != 5 {
		t.Errorf("known app lost traces: got %d, want 5", traces)
	}

	// Neither a failed nor an empty lookup may be read as "no app is known".
	for _, knownApps := range []func() ([]string, error){
		func() ([]string, error) { return nil, errors.New("boom") },
		func() ([]string, error) { return nil, nil },
	} {
		cfg.KnownApps = knownApps
		if err := s.DoClean(context.Background(), cfg); err != nil {
			t.Fatalf("clean: %v", err)
		}
		if traces, _ := appStats(t, s, "known"); traces != 5 {
			t.Errorf("known app dropped after an unusable lookup: got %d, want 5", traces)
		}
	}
}
