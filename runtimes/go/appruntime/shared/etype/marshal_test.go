package etype

import (
	"testing"
	"time"
)

func TestMarshalTime(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+30*60)
	tests := []struct {
		name string
		in   time.Time
		want string
	}{
		{"whole seconds", time.Date(2026, 10, 28, 9, 30, 0, 0, time.UTC), "2026-10-28T09:30:00Z"},
		{"microseconds", time.Date(2026, 10, 28, 9, 30, 0, 123456000, time.UTC), "2026-10-28T09:30:00.123456Z"},
		{"nanoseconds", time.Date(2026, 10, 28, 9, 30, 0, 123456789, time.UTC), "2026-10-28T09:30:00.123456789Z"},
		{"non-utc offset", time.Date(2026, 10, 28, 15, 0, 0, 500000000, ist), "2026-10-28T15:00:00.5+05:30"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, present := MarshalTime(tt.in)
			if !present {
				t.Fatalf("MarshalTime(%v) reported not present", tt.in)
			}
			if got != tt.want {
				t.Errorf("MarshalTime(%v) = %q, want %q", tt.in, got, tt.want)
			}

			back, err := UnmarshalTime(got)
			if err != nil {
				t.Fatalf("UnmarshalTime(%q): %v", got, err)
			}
			if !back.Equal(tt.in) {
				t.Errorf("round trip changed the value: got %v, want %v", back, tt.in)
			}
		})
	}
}
