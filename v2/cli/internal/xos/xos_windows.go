//go:build windows
// +build windows

package xos

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func CreateNewProcessGroup() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
	}
}

func SocketStat(path string) (interface{}, error) {
	fd, err := windows.CreateFile(windows.StringToUTF16Ptr(path), windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, fmt.Errorf("CreateFile %s: %w", path, err)
	}
	defer windows.CloseHandle(fd)

	var d syscall.ByHandleFileInformation
	err = syscall.GetFileInformationByHandle(syscall.Handle(fd), &d)
	if err != nil {
		return nil, &os.PathError{"GetFileInformationByHandle", path, err}
	}
	return &d, nil
}

func SameSocket(a, b interface{}) bool {
	ai := a.(*syscall.ByHandleFileInformation)
	bi := b.(*syscall.ByHandleFileInformation)
	return ai.VolumeSerialNumber == bi.VolumeSerialNumber && ai.FileIndexHigh == bi.FileIndexHigh && ai.FileIndexLow == bi.FileIndexLow
}

func ArrangeExtraFiles(cmd *exec.Cmd, files ...*os.File) error {
	attr := cmd.SysProcAttr
	if attr == nil {
		attr = &syscall.SysProcAttr{}
		cmd.SysProcAttr = attr
	}

	// Flag the files to bbe inherited by the child process
	var fds []string
	for _, f := range files {
		fd := f.Fd()
		fds = append(fds, strconv.FormatUint(uint64(fd), 10))
		err := windows.SetHandleInformation(windows.Handle(fd), windows.HANDLE_FLAG_INHERIT, 1)
		if err != nil {
			return fmt.Errorf("xos.ArrangeExtraFiles: SetHandleInformation: %v", err)
		}
		attr.AdditionalInheritedHandles = append(attr.AdditionalInheritedHandles, syscall.Handle(fd))
	}
	// If the env hasn't been set, copy over this process' env so we preserve the cmd.Env semantics.
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "ENCORE_EXTRA_FILES="+strings.Join(fds, ","))
	return nil
}

func IsAdminUser() (bool, error) {
	// For Windows we elevate permissions on demand, so pretend we are admin
	return true, nil
}
