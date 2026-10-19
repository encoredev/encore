package trace2

import (
	"encoding/binary"
	"net/http"
	"testing"
)

func TestLogHeadersScrubbing(t *testing.T) {
	headers := http.Header{
		"Authorization": {"Bearer secret-token"},
		"Cookie":        {"session=secret-session; theme=dark"},
		"Content-Type":  {"application/json"},
	}

	var l Log
	tb := NewEventBuffer(0)
	l.logHeaders(&tb, headers, map[string]bool{"Authorization": true}, []string{"session"})

	got := decodeHeaders(t, tb.Buf())
	want := map[string]string{
		"Authorization": "[REDACTED]",
		"Cookie":        "session=[REDACTED]; theme=dark",
		"Content-Type":  "application/json",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d headers, want %d: %v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("header %q = %q, want %q", k, got[k], v)
		}
	}
}

// decodeHeaders decodes headers written by logHeaders.
func decodeHeaders(t *testing.T, buf []byte) map[string]string {
	t.Helper()
	uvarint := func() uint64 {
		v, n := binary.Uvarint(buf)
		if n <= 0 {
			t.Fatal("invalid uvarint")
		}
		buf = buf[n:]
		return v
	}
	str := func() string {
		n := uvarint()
		s := string(buf[:n])
		buf = buf[n:]
		return s
	}

	out := make(map[string]string)
	for i, n := 0, int(uvarint()); i < n; i++ {
		k := str()
		out[k] = str()
	}
	return out
}
