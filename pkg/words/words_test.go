package words

import (
	"strings"
	"testing"
)

func TestWords(t *testing.T) {
	list := shortWords.Get()
	if len(list) != 1295 {
		t.Errorf("expected 1295 words, got %d", len(list))
	}

	seen := make(map[string]bool)
	for _, w := range list {
		if seen[w] {
			t.Errorf("duplicate word %q", w)
		} else if len(w) < 3 || len(w) > 5 {
			t.Errorf("expected word to be 3-5 5 characters, got %q", w)
		} else if strings.TrimSpace(w) != w {
			t.Errorf("expected word to be trimmed, got %q", w)
		}
		seen[w] = true
	}
}
