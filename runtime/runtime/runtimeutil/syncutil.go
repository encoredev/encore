package runtimeutil

import (
	"sync"
	"sync/atomic"
)

/*
   This file is originally from go4.org/syncutil

   Copyright 2014 The Perkeep Authors
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at
        http://www.apache.org/licenses/LICENSE-2.0
   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// A Once will perform a successful action exactly once.
//
// Unlike a sync.Once, this Once's func returns an error
// and is re-armed on failure.
type Once struct {
	m    sync.Mutex
	done uint32
}

// Do calls the function f if and only if Do has not been invoked
// without error for this instance of Once.  In other words, given
// 	var once Once
// if once.Do(f) is called multiple times, only the first call will
// invoke f, even if f has a different value in each invocation unless
// f returns an error.  A new instance of Once is required for each
// function to execute.
//
// Do is intended for initialization that must be run exactly once.  Since f
// is niladic, it may be necessary to use a function literal to capture the
// arguments to a function to be invoked by Do:
// 	err := config.once.Do(func() error { return config.init(filename) })
func (o *Once) Do(f func() error) error {
	if atomic.LoadUint32(&o.done) == 1 {
		return nil
	}
	// Slow-path.
	o.m.Lock()
	defer o.m.Unlock()
	var err error
	if o.done == 0 {
		err = f()
		if err == nil {
			atomic.StoreUint32(&o.done, 1)
		}
	}
	return err
}

// A Group is like a sync.WaitGroup and coordinates doing
// multiple things at once. Its zero value is ready to use.
type Group struct {
	wg   sync.WaitGroup
	mu   sync.Mutex // guards errs
	errs []error
}

// Go runs fn in its own goroutine, but does not wait for it to complete.
// Call Err or Errs to wait for all the goroutines to complete.
func (g *Group) Go(fn func() error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		err := fn()
		if err != nil {
			g.mu.Lock()
			defer g.mu.Unlock()
			g.errs = append(g.errs, err)
		}
	}()
}

// Wait waits for all the previous calls to Go to complete.
func (g *Group) Wait() {
	g.wg.Wait()
}

// Err waits for all previous calls to Go to complete and returns the
// first non-nil error, or nil.
func (g *Group) Err() error {
	g.wg.Wait()
	if len(g.errs) > 0 {
		return g.errs[0]
	}
	return nil
}

// Errs waits for all previous calls to Go to complete and returns
// all non-nil errors.
func (g *Group) Errs() []error {
	g.wg.Wait()
	return g.errs
}
