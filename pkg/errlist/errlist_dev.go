//go:build dev_build
// +build dev_build

package errlist

import (
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var projectSourcePath = getProjectSrcPath()

// addErrToList adds a parse error to the list, but also captures
// the position within our parse that the error originated.
func addErrToList(list *scanner.ErrorList, position token.Position, msg string) {
	list.Add(position, msg+getStack())
}

// getRepoPath returns the path to this repo on the local system.
func getProjectSrcPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}

	internalPackagePath := filepath.Dir(file)
	parserPackagePath := filepath.Dir(internalPackagePath)
	encoreProjectPath := filepath.Dir(parserPackagePath)

	return fmt.Sprintf("%s%c", encoreProjectPath, os.PathSeparator)
}

// getStack returns a human readable stack trace.
func getStack() string {
	ret := make([]uintptr, 100)

	index := runtime.Callers(1, ret)
	if index == 0 {
		return ""
	}

	cf := runtime.CallersFrames(ret[:index])
	frame, more := cf.Next()

	// Skip this package
	for strings.Contains(frame.File, "errlist") {
		if !more {
			return ""
		}

		frame, more = cf.Next()
	}

	var stack strings.Builder
	for {
		stack.WriteString("\n\tin ")
		stack.WriteString(strings.TrimPrefix(frame.Function, "encr.dev/"))

		stack.WriteString(" at ")
		stack.WriteString(strings.TrimPrefix(frame.File, projectSourcePath))
		stack.WriteRune(':')
		stack.WriteString(strconv.FormatInt(int64(frame.Line), 10))

		if !more {
			return stack.String()
		}

		frame, more = cf.Next()
	}
}
