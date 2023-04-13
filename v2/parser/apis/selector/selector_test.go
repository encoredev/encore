package selector

import "testing"

func TestSet_ContainsAny(t *testing.T) {
	tests := []struct {
		name  string
		set   []Selector
		input []Selector
		want  bool
	}{
		{
			name:  "empty_input",
			set:   []Selector{{Type: All}},
			input: nil,
			want:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSet(tt.set...)
			in := NewSet(tt.input...)
			if got := s.ContainsAny(in); got != tt.want {
				t.Errorf("ContainsAny() = %v, want %v", got, tt.want)
			}
		})
	}
}
