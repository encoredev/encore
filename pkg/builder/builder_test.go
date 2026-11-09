package builder

import "testing"

func TestTestCompilesBinary(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{args: nil},
		{args: []string{"./...", "-count=1"}},
		{args: []string{"./svc", "-c"}, want: true},
		{args: []string{"-c", "-o", "tests.bin"}, want: true},
		{args: []string{"-o", "tests.bin"}},
		{args: []string{"-cpuprofile=cpu.prof"}},
	} {
		if got := TestCompilesBinary(tc.args); got != tc.want {
			t.Errorf("TestCompilesBinary(%q) = %v, want %v", tc.args, got, tc.want)
		}
	}
}
