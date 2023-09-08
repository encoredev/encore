package editors

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/cockroachdb/errors"
)

// LaunchExternalEditor opens a given file or folder in the desired external editor.
func LaunchExternalEditor(fullPath string, editor FoundEditor) error {
	_, err := os.Stat(editor.Path)
	if os.IsNotExist(err) {
		return errors.Wrapf(err, "editor %s not found", editor.Editor)
	}

	var cmd *exec.Cmd

	//goland:noinspection GoBoolExpressions
	if editor.UsesShell {
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd.exe", "/c", editor.Path, fullPath)
		} else {
			cmd = exec.Command("sh", "-c", editor.Path+" "+fullPath)
		}
	} else if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", "-a", editor.Path, fullPath)
	} else {
		cmd = exec.Command(editor.Path, fullPath)
	}

	// Make sure the editor processes are detached from the Encore daemon.
	// Otherwise, some editors (like Notepad++) will be killed when the
	// Encore daemon shutsdown.
	cmd.SysProcAttr = detachSysProcAttr()

	return errors.Wrap(cmd.Start(), "failed to start editor")
}
