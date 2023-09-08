//go:build !windows

package editors

import (
	"syscall"
)

// detachSysProcAttr returns attributes which ensure the new process is detached from
// the current process
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setsid: true,
	}
}
