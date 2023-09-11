//go:build !windows
// +build !windows

// Package xos provides cross-platform helper functions.
package xos

import (
	"os"
	"os/exec"
	"os/user"
	"syscall"
)

func CreateNewProcessGroup() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

func SocketStat(path string) (interface{}, error) {
	return os.Stat(path)
}

func SameSocket(a, b interface{}) bool {
	ai := a.(os.FileInfo)
	bi := b.(os.FileInfo)
	return os.SameFile(ai, bi)
}

func ArrangeExtraFiles(cmd *exec.Cmd, files ...*os.File) error {
	cmd.ExtraFiles = files
	return nil
}

func IsAdminUser() (bool, error) {
	usr, err := user.Current()
	if err != nil {
		return false, err
	}
	return usr.Gid == "0", nil
}
