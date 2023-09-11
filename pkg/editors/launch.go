package editors

import (
	"io/fs"
	"os"
	"os/exec"
	"runtime"

	"github.com/cockroachdb/errors"
	"github.com/pkg/browser"
	"github.com/rs/zerolog/log"
)

// LaunchExternalEditor opens a given file or folder in the desired external editor.
func LaunchExternalEditor(fullPath string, startLine int, startCol int, editor FoundEditor) error {
	_, err := os.Stat(editor.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return errors.Wrapf(err, "editor %s not found", editor.Editor)
	}

	// Encore patch to allow opening to a specific line and column in the file
	// if supported by the IDE
	toExecute, executeAsURL := convertFilePathToURLScheme(editor.Editor, fullPath, startLine, startCol)
	if executeAsURL {
		log.Info().Str("full_path", fullPath).Str("editor", editor.Editor).Str("url", toExecute).Msg("attempting to open file via URL")
		if err := browser.OpenURL(toExecute); err == nil {
			return nil
		} else {
			log.Warn().Err(err).Str("url", toExecute).Msg("failed to open URL, falling back to file appraoch")
			// If the URL scheme failed to open, then we'll just open the file normally
			toExecute = fullPath
		}
	} else if toExecute == "" {
		toExecute = fullPath
	}

	var cmd *exec.Cmd

	//goland:noinspection GoBoolExpressions
	if editor.UsesShell {
		if runtime.GOOS == "windows" {
			// nosemgrep
			cmd = exec.Command("cmd.exe", "/c", editor.Path, toExecute)
		} else {
			// nosemgrep
			cmd = exec.Command("sh", "-c", editor.Path+" "+toExecute)
		}
	} else if runtime.GOOS == "darwin" {
		// nosemgrep
		cmd = exec.Command("open", "-a", editor.Path, toExecute)
	} else {
		// nosemgrep
		cmd = exec.Command(editor.Path, toExecute)
	}

	// Make sure the editor processes are detached from the Encore daemon.
	// Otherwise, some editors (like Notepad++) will be killed when the
	// Encore daemon shutsdown.
	cmd.SysProcAttr = detachSysProcAttr()

	log.Info().Str("full_path", fullPath).Str("editor", editor.Editor).Str("cmd", cmd.String()).Msg("attempting to open file")

	return errors.Wrap(cmd.Start(), "failed to start editor")
}
