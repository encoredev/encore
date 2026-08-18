//go:build windows

package desktop

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/cockroachdb/errors"
	"golang.org/x/sys/windows"
)

// open launches the path with its registered handler. For a directory this opens
// an Explorer window on the directory itself.
func open(path string) error {
	target, err := shellPath(path)
	if err != nil {
		return err
	}

	file, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}

	// A nil verb means the file type's default verb, falling back to "open" and
	// then to whatever the registry lists first.
	err = windows.ShellExecute(0, nil, file, nil, nil, windows.SW_SHOWNORMAL)
	runtime.KeepAlive(file)
	if err != nil {
		return errors.Wrapf(err, "open %s", target)
	}
	return nil
}

// reveal opens Explorer on the directory holding the path, with the item selected.
func reveal(path string) error {
	target, err := shellPath(path)
	if err != nil {
		return err
	}

	// The command line is built by hand below, because Explorer doesn't accept the
	// quoting Go applies. A double quote is the one character that could break out
	// of it, and an existing file can't contain one, so this check is dead code kept
	// as a backstop for a path that never passed Reveal's stat.
	if strings.Contains(target, `"`) {
		return errors.Newf("cannot reveal a path containing a quote: %s", target)
	}

	// Resolve explorer.exe absolutely rather than off PATH.
	windowsDir, err := windows.GetWindowsDirectory()
	if err != nil {
		return errors.Wrap(err, "locate the Windows directory")
	}
	explorer := filepath.Join(windowsDir, "explorer.exe")

	// nosemgrep
	cmd := exec.Command(explorer)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: `"` + explorer + `" /select,"` + target + `"`,
	}
	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "reveal %s", target)
	}
	// Explorer's exit status is not meaningful, so it is only reaped.
	go func() { _ = cmd.Wait() }()
	return nil
}

func supported() bool {
	return true
}

// shellPath prepares a path for Explorer, which is not the Win32 file API. The \\?\
// verbatim prefix has to go, because shell entry points reject such a path even
// when it is well under MAX_PATH, so the path must never come from a canonicalizing
// step. And a bare drive designator needs its trailing separator: "C:\" resolves
// where "C:" doesn't.
func shellPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("empty path")
	}

	switch {
	case strings.HasPrefix(path, `\\?\UNC\`), strings.HasPrefix(path, `\\.\UNC\`):
		path = `\\` + path[len(`\\?\UNC\`):] // \\?\UNC\srv\share -> \\srv\share
	case strings.HasPrefix(path, `\\?\`), strings.HasPrefix(path, `\\.\`):
		path = path[len(`\\?\`):] // \\?\C:\dir -> C:\dir
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", errors.Wrapf(err, "resolve %s", path)
	}
	abs = filepath.Clean(abs)
	if len(abs) == 2 && abs[1] == ':' {
		// Clean("C:\\") keeps the separator, but Clean("C:") drops it.
		abs += `\`
	}

	// The shell namespace is bound by MAX_PATH with no \\?\ escape hatch, and deep
	// object keys reach that in practice. Say so rather than opening a window on
	// the wrong thing.
	if len(abs) >= 260 {
		return "", errors.Newf("path is %d characters, which exceeds the Windows shell's limit of 260: %s", len(abs), abs)
	}
	return abs, nil
}
