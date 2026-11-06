package etrace

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
)

// The below code snippet is copied from go4.org/syncutil/syncdebug.
//
// Copyright 2013 The Perkeep Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

var goroutineSpace = []byte("goroutine ")

func goroutineID() int64 {
	// Only the first line is read, and runtime.Stack stops when the buffer is full.
	var buf [64]byte
	b := buf[:runtime.Stack(buf[:], false)]
	// Parse the 4707 out of "goroutine 4707 ["
	b = bytes.TrimPrefix(b, goroutineSpace)
	i := bytes.IndexByte(b, ' ')
	if i < 0 {
		panic(fmt.Sprintf("No space found in %q", b))
	}
	b = b[:i]
	n, err := strconv.ParseUint(string(b), 10, 64)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse goroutine ID out of %q: %v", b, err))
	}
	return int64(n)
}
