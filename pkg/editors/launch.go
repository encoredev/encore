package editors

import (
	"io/fs"
	"os"
	"os/exec"
	"runtime"

	"github.com/cockroachdb/errors"
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
	toExecute := convertFilePathToURLScheme(editor.Editor, fullPath, startLine, startCol)

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
