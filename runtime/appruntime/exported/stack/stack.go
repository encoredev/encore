// Package stack collects stack traces.
package stack

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
)

type Stack struct {
	Frames []uintptr
	Off    uintptr
}

func Build(skip int) Stack {
	pcs := make([]uintptr, 101)
	idx, off := encoreCallers(skip+1, pcs)
	pcs = pcs[:idx]
	if idx == 0 {
		return Stack{}
	}

	// Go through our PCs and see if we reached a stop PC.
	stopMu.RLock()
	for i, pc := range pcs {
		if stopPCs[pc] {
			stopMu.RUnlock()
			return Stack{Frames: pcs[:i], Off: off}
		}
	}
	stopMu.RUnlock()

	for i, p := range pcs {
		fn := runtime.FuncForPC(p)
		// Is this a new stop point?
		if fn != nil && strings.Contains(fn.Name(), "__encore_") {
			stopMu.Lock()
			stopPCs[p] = true
			stopMu.Unlock()
			return Stack{Frames: pcs[:i], Off: off}
		}
	}
	return Stack{Frames: pcs, Off: off}
}

func Print(s Stack) {
	var b bytes.Buffer
	cf := runtime.CallersFrames(s.Frames)
	i := 0
	for {
		f, more := cf.Next()
		pc := s.Frames[i] - s.Off
		fmt.Fprintf(&b, "%d: %s:%d: %s\n", pc, f.File, f.Line, f.Function)
		if !more {
			break
		}
		i++
	}
	if s := b.Bytes(); len(s) > 0 {
		os.Stdout.Write(s)
	}
}

type FormattedFrame struct {
	File string
	Line int
	Func string
}

func Format(s Stack) []FormattedFrame {
	var frames []FormattedFrame
	cf := runtime.CallersFrames(s.Frames)
	i := 0
	for {
		f, more := cf.Next()
		frames = append(frames, FormattedFrame{
			File: f.File,
			Line: f.Line,
			Func: f.Function,
		})
		if !more {
			break
		}
		i++
	}
	return frames
}

var (
	stopMu  sync.RWMutex
	stopPCs = make(map[uintptr]bool)
)
