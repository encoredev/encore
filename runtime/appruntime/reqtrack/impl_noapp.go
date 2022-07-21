//go:build !encore_app

package reqtrack

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"sync"
)

func newImpl() reqTrackImpl {
	return &noappImpl{
		gmap: make(map[int64]*encoreG),
	}
}

type noappImpl struct {
	mu   sync.Mutex
	gmap map[int64]*encoreG
}

var _ reqTrackImpl = (*noappImpl)(nil)

func (i *noappImpl) get() *encoreG {
	id := goroutineID()

	i.mu.Lock()
	defer i.mu.Unlock()
	return i.gmap[id]
}

func (i *noappImpl) set(val *encoreG) {
	id := goroutineID()

	i.mu.Lock()
	defer i.mu.Unlock()
	i.gmap[id] = val
}

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

const stackBufSize = 16 << 20

var stackBuf = make(chan []byte, 8)

func getBuf() []byte {
	select {
	case b := <-stackBuf:
		return b[:stackBufSize]
	default:
		return make([]byte, stackBufSize)
	}
}

func putBuf(b []byte) {
	select {
	case stackBuf <- b:
	default:
	}
}

var goroutineSpace = []byte("goroutine ")

func goroutineID() int64 {
	b := getBuf()
	defer putBuf(b)
	b = b[:runtime.Stack(b, false)]
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
