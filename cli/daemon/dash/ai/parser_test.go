package ai

import (
	"testing"
)

func TestParseDocList_StopsAtNonListLine(t *testing.T) {
	doc := `Some preamble
Errors:
- foo: something
- bar: another thing
Next section starts here
- baz: should not be included`

	_, items := parseDocList(doc, "Errors")

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d: %+v", len(items), items)
	}
	if items[0].Key != "foo" || items[0].Doc != "something" {
		t.Errorf("unexpected item[0]: %+v", items[0])
	}
	if items[1].Key != "bar" || items[1].Doc != "another thing" {
		t.Errorf("unexpected item[1]: %+v", items[1])
	}
}
