package cache

import (
	"testing"
	"time"
)

func TestExpireDailyAt(t *testing.T) {
	west := time.FixedZone("UTC-08", -8*60*60)
	east := time.FixedZone("UTC+14", 14*60*60)
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	parse := func(s string) time.Time {
		t.Helper()
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	for _, tc := range []struct {
		name                 string
		now                  string
		loc                  *time.Location
		hour, minute, second int
		want                 string
	}{
		{"utc before", "2026-11-16T01:00:00Z", time.UTC, 20, 0, 0, "2026-11-16T20:00:00Z"},
		{"utc after", "2026-11-16T21:00:00Z", time.UTC, 20, 0, 0, "2026-11-17T20:00:00Z"},
		{"utc equal", "2026-11-16T20:00:00Z", time.UTC, 20, 0, 0, "2026-11-16T20:00:00Z"},
		{"previous local date", "2026-11-16T01:00:00Z", west, 20, 15, 30, "2026-11-16T04:15:30Z"},
		{"next local date", "2026-11-16T20:00:00Z", east, 9, 0, 0, "2026-11-17T19:00:00Z"},
		{"non utc input", "2026-11-17T10:00:00+14:00", time.UTC, 23, 0, 0, "2026-11-16T23:00:00Z"},
		{"year rollover", "2026-12-31T23:00:00Z", time.UTC, 1, 0, 0, "2027-01-01T01:00:00Z"},
		{"spring previous local date", "2026-03-08T01:00:00Z", newYork, 22, 0, 0, "2026-03-08T03:00:00Z"},
		{"spring next day", "2026-03-07T23:00:00Z", newYork, 17, 0, 0, "2026-03-08T21:00:00Z"},
		{"fall previous local date", "2026-11-01T00:30:00Z", newYork, 22, 0, 0, "2026-11-01T02:00:00Z"},
		{"fall next day", "2026-10-31T23:00:00Z", newYork, 17, 0, 0, "2026-11-01T22:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now, want := parse(tc.now), parse(tc.want)
			expiry := ExpireDailyAt(tc.hour, tc.minute, tc.second, tc.loc)
			if got := expiry(now); !got.Equal(want) {
				t.Errorf("expiry = %s, want %s", got, want)
			}
			if got := expiry(now.In(tc.loc)); !got.Equal(want) {
				t.Errorf("expiry with input in target location = %s, want %s", got, want)
			}
		})
	}
}
