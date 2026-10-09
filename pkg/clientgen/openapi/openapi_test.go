package openapi

import (
	"strings"
	"testing"
)

func TestMarkdownDocUsesSpacesForCodeBlocks(t *testing.T) {
	got := markdownDoc("Description.\n\n    code")
	if strings.ContainsRune(got, '\t') {
		t.Fatalf("markdownDoc() contains a tab: %q", got)
	}
	if want := "Description.\n\n    code\n"; got != want {
		t.Fatalf("markdownDoc() = %q, want %q", got, want)
	}
}
