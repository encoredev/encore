package scrub

import "testing"

func TestCookie(t *testing.T) {
	const r = "[REDACTED]"
	tests := []struct {
		name   string
		header string
		names  []string
		want   string
	}{
		{"no_names", "session=abc; theme=dark", nil, "session=abc; theme=dark"},
		{"empty_header", "", []string{"session"}, ""},
		{"single", "session=abc", []string{"session"}, "session=" + r},
		{"first_of_many", "session=abc; theme=dark", []string{"session"}, "session=" + r + "; theme=dark"},
		{"last_of_many", "theme=dark; session=abc", []string{"session"}, "theme=dark; session=" + r},
		{"multiple_names", "a=1; b=2; c=3", []string{"a", "c"}, "a=" + r + "; b=2; c=" + r},
		{"repeated_name", "session=abc; session=def", []string{"session"}, "session=" + r + "; session=" + r},
		{"no_match", "theme=dark", []string{"session"}, "theme=dark"},
		{"case_sensitive", "Session=abc", []string{"session"}, "Session=abc"},
		{"quoted_value", `session="abc"; theme=dark`, []string{"session"}, "session=" + r + "; theme=dark"},
		{"value_contains_eq", "session=a=b; theme=dark", []string{"session"}, "session=" + r + "; theme=dark"},
		{"empty_value", "session=; theme=dark", []string{"session"}, "session=" + r + "; theme=dark"},
		{"no_space_after_semicolon", "theme=dark;session=abc", []string{"session"}, "theme=dark;session=" + r},
		{"trailing_semicolon", "session=abc;", []string{"session"}, "session=" + r + ";"},
		{"malformed_part", "theme=dark; abc", []string{"session"}, "theme=dark; " + r},
		{"prefix_is_not_match", "session_id=abc", []string{"session"}, "session_id=abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Cookie(tt.header, tt.names, r); got != tt.want {
				t.Errorf("Cookie(%q, %q) = %q, want %q", tt.header, tt.names, got, tt.want)
			}
		})
	}
}
