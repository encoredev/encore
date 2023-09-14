package editors

import (
	"context"
	goerrors "errors"
	"sort"

	"github.com/cockroachdb/errors"
	"go4.org/syncutil"
)

// FoundEditor is a found external editor on the user's machine
type FoundEditor struct {
	// The friendly name of the editor, to be used in labels
	Editor EditorName `json:"editor"`
	// The executable associated with the editor to launch
	Path string `json:"path"`
	// The editor requires a shell to spawn
	UsesShell bool `json:"usesShell,omitempty"`
}

var (
	editorCacheOnce syncutil.Once
	// editorCache is a cache of the available editors on the user's machine
	editorCache []FoundEditor

	// ErrEditorNotFound is returned when an editor is not found when called from
	// the Find function
	ErrEditorNotFound = goerrors.New("editor not found")
)

// Resolve a list of installed editors on the user's machine, using the known
// install identifiers that each OS supports.
func Resolve(ctx context.Context) ([]FoundEditor, error) {
	err := editorCacheOnce.Do(func() error {
		var err error
		editorCache, err = getAvailableEditors(ctx)
		if err != nil {
			return errors.Wrap(err, "unable to get available editors")
		}

		sort.Slice(editorCache, func(i, j int) bool {
			return editorCache[i].Editor < editorCache[j].Editor
		})
		return nil
	})

	return editorCache, err
}

// Find searches to an editor by name, returning the editor if found, or
// [ErrEditorNotFound] if not found
func Find(ctx context.Context, name EditorName) (FoundEditor, error) {
	editors, err := Resolve(ctx)
	if err != nil {
		return FoundEditor{}, err
	}

	for _, editor := range editors {
		if editor.Editor == name {
			return editor, nil
		}
	}

	return FoundEditor{}, errors.WithStack(ErrEditorNotFound)
}
