// +build windows

package winsvc

import "testing"

func TestShellEscape(t *testing.T) {
	tests := []struct {
		Arg      string
		Expected string
	}{
		{
			"foo",
			"foo",
		},
		{
			"foo bar",
			`"foo bar"`,
		},
		{
			`foo "bar" baz`,
			`"foo ""bar"" baz"`,
		},
	}
	for _, test := range tests {
		if got, want := ShellEscape(test.Arg), test.Expected; got != want {
			t.Errorf("ShellEscape(%q) = %q, want %q", test.Arg, got, want)
		}
	}
}
