//go:build !windows

package desktop

import (
	"os/exec"

	"github.com/cockroachdb/errors"

	"encr.dev/pkg/xos"
)

// runCmd starts a command and returns as soon as it has been launched.
//
// The child gets its own process group so the program it opens isn't killed along
// with the daemon.
func runCmd(name string, args ...string) error {
	// nosemgrep
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = xos.CreateNewProcessGroup()
	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "run %s", name)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
