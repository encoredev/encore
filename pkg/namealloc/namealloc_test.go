package namealloc

import (
	"reflect"
	"testing"
)

func TestAlloc(t *testing.T) {
	tests := []struct {
		name     string
		reserved []string // reserved words
		in       []string
		out      []string
	}{
		{
			name: "simple",
			in:   []string{"hello", "hello", "there", "hello"},
			out:  []string{"hello", "hello2", "there", "hello3"},
		},
		{
			name:     "reserved",
			reserved: []string{"hello"},
			in:       []string{"hello", "hello", "there", "hello"},
			out:      []string{"hello_", "hello_2", "there", "hello_3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reserved := func(s string) bool {
				for _, r := range tt.reserved {
					if r == s {
						return true
					}
				}
				return false
			}
			a := &Allocator{Reserved: reserved}

			got := make([]string, 0, len(tt.in))
			for _, in := range tt.in {
				v := a.Get(in)
				got = append(got, v)
			}
			if !reflect.DeepEqual(got, tt.out) {
				t.Errorf("Alloc(%+v) = %+v, want %+v", tt.in, got, tt.out)
			}
		})
	}
}
