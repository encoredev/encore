package mcp

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestStringifyBody_ConvertsBytesToString(t *testing.T) {
	in := map[string]any{
		"status": 200,
		"body":   []byte(`{"hello":"world"}`),
	}
	got := stringifyBody(in)
	want := map[string]any{
		"status": 200,
		"body":   `{"hello":"world"}`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestStringifyBody_PassesThroughOtherTypes(t *testing.T) {
	in := map[string]any{"body": "already-a-string"}
	got := stringifyBody(in)
	if got["body"].(string) != "already-a-string" {
		t.Fatalf("unexpected mutation: %+v", got)
	}
}

func TestParseRetryUntil_Empty(t *testing.T) {
	cfg, ok, err := parseRetryUntil(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for empty input")
	}
	_ = cfg
}

func TestParseRetryUntil_StatusPredicate(t *testing.T) {
	in := map[string]any{
		"predicate":   map[string]any{"status": float64(200)},
		"timeout_ms":  float64(5000),
		"interval_ms": float64(250),
	}
	cfg, ok, err := parseRetryUntil(in)
	if err != nil || !ok {
		t.Fatalf("err: %v ok: %v", err, ok)
	}
	if cfg.Predicate.Status != 200 {
		t.Errorf("status: %d", cfg.Predicate.Status)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("timeout: %v", cfg.Timeout)
	}
	if cfg.Interval != 250*time.Millisecond {
		t.Errorf("interval: %v", cfg.Interval)
	}
}

func TestParseRetryUntil_BodyPathPredicate(t *testing.T) {
	in := map[string]any{
		"predicate": map[string]any{
			"body_path": map[string]any{"path": ".id", "equals": float64(7)},
		},
	}
	cfg, ok, err := parseRetryUntil(in)
	if err != nil || !ok {
		t.Fatal("expected ok")
	}
	if cfg.Predicate.Path == nil || cfg.Predicate.Path.Path != ".id" {
		t.Errorf("got: %+v", cfg.Predicate.Path)
	}
}

func TestParseRetryUntil_BodyJqPredicate(t *testing.T) {
	in := map[string]any{
		"predicate": map[string]any{"body_jq": ".events | length > 0"},
	}
	cfg, ok, err := parseRetryUntil(in)
	if err != nil || !ok {
		t.Fatal("expected ok")
	}
	if cfg.Predicate.Jq != ".events | length > 0" {
		t.Errorf("jq: %q", cfg.Predicate.Jq)
	}
}

func TestRunRetryLoop_MatchesAfterRetries(t *testing.T) {
	calls := 0
	doCall := func(ctx context.Context) (map[string]any, error) {
		calls++
		if calls < 3 {
			return map[string]any{"status": "200 OK", "status_code": 200, "body": `{"events":[]}`}, nil
		}
		return map[string]any{"status": "200 OK", "status_code": 200, "body": `{"events":[{"id":7}]}`}, nil
	}
	cfg := retryConfig{
		Predicate: predicate{Jq: ".events | length > 0"},
		Timeout:   time.Second,
		Interval:  10 * time.Millisecond,
	}
	res, info, err := runRetryLoop(context.Background(), cfg, doCall)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !info.Matched {
		t.Fatal("expected matched")
	}
	if info.Attempts != 3 {
		t.Errorf("attempts: %d, want 3", info.Attempts)
	}
	if res["body"].(string) != `{"events":[{"id":7}]}` {
		t.Errorf("got body: %v", res["body"])
	}
}

func TestRunRetryLoop_TimeoutReturnsLastBody(t *testing.T) {
	doCall := func(ctx context.Context) (map[string]any, error) {
		return map[string]any{"status": "200 OK", "status_code": 200, "body": `{"events":[]}`}, nil
	}
	cfg := retryConfig{
		Predicate:     predicate{Jq: ".events | length > 0"},
		Timeout:       80 * time.Millisecond,
		Interval:      20 * time.Millisecond,
		FailOnTimeout: false,
	}
	res, info, err := runRetryLoop(context.Background(), cfg, doCall)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if info.Matched {
		t.Fatal("expected not matched")
	}
	if res["body"].(string) != `{"events":[]}` {
		t.Errorf("expected last body to be returned")
	}
	if info.Attempts < 2 {
		t.Errorf("expected at least 2 attempts, got %d", info.Attempts)
	}
}

func TestRunRetryLoop_FailOnTimeout_ReturnsError(t *testing.T) {
	doCall := func(ctx context.Context) (map[string]any, error) {
		return map[string]any{"status": "200 OK", "status_code": 200, "body": `{"events":[]}`}, nil
	}
	cfg := retryConfig{
		Predicate:     predicate{Jq: ".events | length > 0"},
		Timeout:       50 * time.Millisecond,
		Interval:      20 * time.Millisecond,
		FailOnTimeout: true,
	}
	_, _, err := runRetryLoop(context.Background(), cfg, doCall)
	if err == nil {
		t.Fatal("expected error")
	}
	var rte *retryTimeoutError
	if !errors.As(err, &rte) {
		t.Fatalf("expected retryTimeoutError, got %T: %v", err, err)
	}
	if rte.Attempts < 2 {
		t.Errorf("expected attempts >= 2, got %d", rte.Attempts)
	}
}

func TestParseRetryUntil_EmptyObjectMeansNoRetry(t *testing.T) {
	cfg, ok, err := parseRetryUntil(map[string]any{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for empty object")
	}
	_ = cfg
}

func TestParseRetryUntil_StringFormParsedAsJSON(t *testing.T) {
	cfg, ok, err := parseRetryUntil(`{"predicate":{"status":200},"timeout_ms":1000}`)
	if err != nil || !ok {
		t.Fatalf("err: %v ok: %v", err, ok)
	}
	if cfg.Predicate.Status != 200 {
		t.Errorf("status: %d", cfg.Predicate.Status)
	}
	if cfg.Timeout != time.Second {
		t.Errorf("timeout: %v", cfg.Timeout)
	}
}

func TestParseRetryUntil_InvalidStringReturnsError(t *testing.T) {
	_, _, err := parseRetryUntil(`not json`)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRetryUntil_EmptyStringMeansNoRetry(t *testing.T) {
	cfg, ok, err := parseRetryUntil("")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for empty string")
	}
	_ = cfg
}

func TestRunRetryLoop_StatusPredicateMatchesIntStatusCode(t *testing.T) {
	// Pin the bug: run.CallAPI returns status_code as int, not float64,
	// and the numeric code lives under "status_code" not "status".
	calls := 0
	doCall := func(ctx context.Context) (map[string]any, error) {
		calls++
		if calls == 1 {
			return map[string]any{"status": "404 Not Found", "status_code": 404, "body": ""}, nil
		}
		return map[string]any{"status": "200 OK", "status_code": 200, "body": ""}, nil
	}
	cfg := retryConfig{
		Predicate: predicate{Status: 200},
		Timeout:   500 * time.Millisecond,
		Interval:  10 * time.Millisecond,
	}
	res, info, err := runRetryLoop(context.Background(), cfg, doCall)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !info.Matched {
		t.Fatal("expected matched after status flipped to 200")
	}
	if info.Attempts != 2 {
		t.Errorf("attempts: %d, want 2", info.Attempts)
	}
	if got := extractStatusCode(res); got != 200 {
		t.Errorf("expected status_code 200, got %d", got)
	}
}
