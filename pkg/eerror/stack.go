package eerror

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type StackFrame struct {
	// PC is the program counter for the frame (needed for things like sentry reporting)
	PC uintptr `json:"pc"`

	// Human-readable fields
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

var projectSourcePath = getProjectSrcPath()

// getRepoPath returns the path to this repo on the local system
func getProjectSrcPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}

	eerrorPath := filepath.Dir(file)
	pkgPath := filepath.Dir(eerrorPath)
	encoreProjectPath := filepath.Dir(pkgPath)

	return fmt.Sprintf("%s%c", encoreProjectPath, os.PathSeparator)
}

// getStack returns a human read able stack trace
func getStack() []*StackFrame {
	ret := make([]uintptr, 100)

	index := runtime.Callers(1, ret)
	if index == 0 {
		return nil
	}

	cf := runtime.CallersFrames(ret[:index])
	frame, more := cf.Next()

	// Skip over the "eerror" package files
	for strings.Contains(frame.File, "eerror") {
		if !more {
			return nil
		}

		frame, more = cf.Next()
	}

	var frames []*StackFrame
	for {
		frames = append(frames, &StackFrame{
			PC:       frame.PC,
			Function: strings.TrimPrefix(frame.Function, "encr.dev/"),
			File:     strings.TrimPrefix(frame.File, projectSourcePath),
			Line:     frame.Line,
		})

		if !more {
			return frames
		}

		frame, more = cf.Next()
	}
}
