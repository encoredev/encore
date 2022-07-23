package runtimeutil

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

import (
	"errors"
	"testing"
)

func TestOnce(t *testing.T) {
	timesRan := 0
	f := func() error {
		timesRan++
		return nil
	}

	once := Once{}
	grp := Group{}

	for i := 0; i < 10; i++ {
		grp.Go(func() error { return once.Do(f) })
	}

	if grp.Err() != nil {
		t.Errorf("Expected no errors, got %v", grp.Err())
	}

	if timesRan != 1 {
		t.Errorf("Expected to run one time, ran %d", timesRan)
	}
}

// TestOnceErroring verifies we retry on every error, but stop after
// the first success.
func TestOnceErroring(t *testing.T) {
	timesRan := 0
	f := func() error {
		timesRan++
		if timesRan < 3 {
			return errors.New("retry")
		}
		return nil
	}

	once := Once{}
	grp := Group{}

	for i := 0; i < 10; i++ {
		grp.Go(func() error { return once.Do(f) })
	}

	if len(grp.Errs()) != 2 {
		t.Errorf("Expected two errors, got %d", len(grp.Errs()))
	}

	if timesRan != 3 {
		t.Errorf("Expected to run two times, ran %d", timesRan)
	}
}
