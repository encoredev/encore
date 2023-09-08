package editors

import (
	"context"
	goerrors "errors"
	"strings"
	"sync"

	"github.com/cockroachdb/errors"
)

// FoundEditor is a found external editor on the user's machine
type FoundEditor struct {
	// The friendly name of the editor, to be used in labels
	Editor string `json:"editor"`
	// The executable associated with the editor to launch
	Path string `json:"path"`
	// The editor requires a shell to spawn
	UsesShell bool `json:"usesShell,omitempty"`
}

var (
	editorCacheMu sync.Mutex
	// editorCache is a cache of the available editors on the user's machine
	editorCache []FoundEditor

	// ErrEditorNotFound is returned when an editor is not found when called from
	// the Find function
	ErrEditorNotFound = goerrors.New("editor not found")
)

// Resolve a list of installed editors on the user's machine, using the known
// install identifiers that each OS supports.
func Resolve(ctx context.Context) ([]FoundEditor, error) {
	editorCacheMu.Lock()
	defer editorCacheMu.Unlock()

	// If we don't have a cache yet, populate it
	if len(editorCache) == 0 && cap(editorCache) == 0 {
		var err error
		editorCache, err = getAvailableEditors(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "unable to get available editors")
		}
	}

	return editorCache, nil
}

// Find searches to an editor by name, returning the editor if found, or
// [ErrEditorNotFound] if not found
func Find(ctx context.Context, name string) (FoundEditor, error) {
	editors, err := Resolve(ctx)
	if err != nil {
		return FoundEditor{}, err
	}

	for _, editor := range editors {
		if strings.EqualFold(editor.Editor, name) {
			return editor, nil
		}
	}

	return FoundEditor{}, errors.WithStack(ErrEditorNotFound)
}
