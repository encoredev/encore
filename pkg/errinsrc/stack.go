package errinsrc

import (
	"fmt"
	"runtime"
	"strings"
)

// StackFrame represents a single frame in a Stack trace
type StackFrame struct {
	ProgramCounter uintptr `json:"pc"`
	File           string  `json:"file"`
	Package        string  `json:"pkg"`
	Function       string  `json:"fun"`
	Line           int     `json:"line"`
}

const maxFramesOnPrettyPrint = 5

func GetStack() []*StackFrame {
	ret := make([]uintptr, 100)

	index := runtime.Callers(1, ret)
	if index == 0 {
		return nil
	}

	cf := runtime.CallersFrames(ret[:index])
	frame, more := cf.Next()

	// Skip over the "errinsrc" or "errlist" package files or any subpackages
	// which are the top frames (as these would only be related to the creation of the error)
	for strings.Contains(frame.File, "errinsrc") ||
		strings.Contains(frame.File, "errlist") ||
		strings.Contains(frame.File, "perr") ||
		strings.HasSuffix(frame.File, "errs.go") {
		if !more {
			return nil
		}

		frame, more = cf.Next()
	}

	var frames []*StackFrame
	for {
		// Skip the frame if it's Go Runtime or internal testing code related code
		if !strings.HasPrefix(frame.Function, "runtime.") && !strings.HasPrefix(frame.Function, "testing.") {
			// Seperate the package name and function name
			pkgAndFunc := frame.Function
			if idx := strings.LastIndex(pkgAndFunc, "/"); idx >= 0 {
				pkgAndFunc = pkgAndFunc[idx+1:]
			}
			pkgName, funcName, _ := strings.Cut(pkgAndFunc, ".")

			// Record the frame
			frames = append(frames, &StackFrame{
				ProgramCounter: frame.PC,
				Package:        pkgName,
				Function:       funcName,
				File:           frame.File,
				Line:           frame.Line,
			})
		}

		if !more {
			return frames
		}

		frame, more = cf.Next()
	}
}

func prettyPrintStack(stack []*StackFrame, b *strings.Builder) string {
	b.WriteString(aurora.Gray(grayLevelOnLineNumbers, fmt.Sprintf("%c%c%c", set.LeftTop, set.HorizontalBar, set.LeftBracket)).String())
	b.WriteString(aurora.Cyan("Stack Trace").String())
	b.WriteString(aurora.Gray(grayLevelOnLineNumbers, fmt.Sprintf("%c", set.RightBracket)).String())

	longestFunc := 0
	for i, frame := range stack {
		name := fmt.Sprintf("%s.%s", frame.Package, frame.Function)
		if length := len(name); length > longestFunc {
			longestFunc = length
		}

		if i >= maxFramesOnPrettyPrint {
			break
		}
	}

	for i, frame := range stack {
		vertical := set.LeftCross
		if i == len(stack)-1 {
			vertical = set.LeftBottom
		}

		b.WriteString(
			fmt.Sprintf(
				"\n%s %s.%s%s %s:%d",
				aurora.Gray(grayLevelOnLineNumbers, fmt.Sprintf("%c%c%c", vertical, set.HorizontalBar, set.RightArrow)),
				aurora.Gray(18, frame.Package),
				aurora.Magenta(frame.Function),
				strings.Repeat(" ", longestFunc-len(frame.Function)-len(frame.Package)-1),
				frame.File,
				frame.Line,
			),
		)

		if i >= maxFramesOnPrettyPrint && i != len(stack)-1 {
			b.WriteString("\n")
			b.WriteString(aurora.Gray(grayLevelOnLineNumbers, fmt.Sprintf("%c%c%c", set.LeftBottom, set.HorizontalBar, set.LeftBracket)).String())
			b.WriteString(aurora.Yellow("... remaining frames omitted ...").Italic().String())
			b.WriteString(aurora.Gray(grayLevelOnLineNumbers, fmt.Sprintf("%c", set.RightBracket)).String())

			break
		}
	}
	b.WriteString("\n\n")

	return b.String()
}
