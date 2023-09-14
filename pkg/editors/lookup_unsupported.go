//go:build !darwin && !linux && !windows

package editors

import (
	"context"
)

// Returns no editors as we don't know how to find them on this platform.
func getAvailableEditors(ctx context.Context) ([]FoundEditor, error) {
	return []FoundEditor{}, nil
}
