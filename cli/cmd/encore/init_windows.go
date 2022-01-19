//go:build windows
// +build windows

package main

import (
	"golang.org/x/sys/windows"
)

// init activates virtual terminal feature on "windows", this enables colored
// terminal output.
func init() {
	setConsoleMode(windows.Stdout, windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	setConsoleMode(windows.Stderr, windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}

// setConsoleMode enables VT processing on stout and stderr.
func setConsoleMode(handle windows.Handle, flag uint32) {
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err == nil {
		windows.SetConsoleMode(handle, mode|flag)
	}
}
