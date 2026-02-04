package urlutil

import "testing"

func TestJoinURL(t *testing.T) {
	cases := []struct {
		base string
		path string
		want string
	}{
		{"https://a.com", "x", "https://a.com/x"},
		{"https://a.com/", "x", "https://a.com/x"},
		{"https://a.com", "/x", "https://a.com/x"},
		{"https://a.com/", "/x", "https://a.com/x"},
		{"https://a.com", "https://b.com/y", "https://b.com/y"}, // Guard check
		{"", "/x", "x"},                         // Empty base check
		{"   ", "/x", "x"},                      // Whitespace base check
		{"https://a.com", "", "https://a.com/"}, // Empty path
	}
	for _, c := range cases {
		if got := JoinURL(c.base, c.path); got != c.want {
			t.Errorf("JoinURL(%q,%q)=%q want %q", c.base, c.path, got, c.want)
		}
	}
}
