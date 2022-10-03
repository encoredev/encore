package echo

import (
	"context"
	"reflect"
	"testing"
)

func TestConfigValues(t *testing.T) {
	values, err := ConfigValues(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if values.SubKeyCount != 3 {
		t.Fatalf("expected 1, got %d", values.SubKeyCount)
	}
}
