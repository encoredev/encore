//go:build !windows
// +build !windows

// Package xos provides cross-platform helper functions.
package xos

import (
	"os"
	"os/exec"
	"os/user"
	"syscall"

	"github.com/cockroachdb/errors"
	"github.com/google/renameio/v2"
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

// WriteFile writes the given file with the given data and permissions.
//
// Where possible (i.e. not on windows) it will use an atomic write process
// which removes the possibility of a partial file being written during a crash
// or error.
func WriteFile(filename string, data []byte, perm os.FileMode) error {
	return errors.WithStack(renameio.WriteFile(filename, data, perm))
}
