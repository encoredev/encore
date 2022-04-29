// This package is a simplified copy of cmd/go/internal/vcs/vcs.go from the Go project:
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vcs

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Status is the current state of a local repository.
type Status struct {
	Revision    string    // Optional.
	CommitTime  time.Time // Optional.
	Uncommitted bool      // Required.
}

func gitStatus(rootDir string) (Status, error) {
	out, err := vcsGit.runOutputVerboseOnly(rootDir, "status --porcelain")
	if err != nil {
		return Status{}, err
	}
	uncommitted := len(out) > 0

	// "git status" works for empty repositories, but "git show" does not.
	// Assume there are no commits in the repo when "git show" fails with
	// uncommitted files and skip tagging revision / committime.
	var rev string
	var commitTime time.Time
	out, err = vcsGit.runOutputVerboseOnly(rootDir, "-c log.showsignature=false show -s --format=%H:%ct")
	if err != nil && !uncommitted {
		return Status{}, err
	} else if err == nil {
		rev, commitTime, err = parseRevTime(out)
		if err != nil {
			return Status{}, err
		}
	}

	return Status{
		Revision:    rev,
		CommitTime:  commitTime,
		Uncommitted: uncommitted,
	}, nil
}

// parseRevTime parses commit details in "revision:seconds" format.
func parseRevTime(out []byte) (string, time.Time, error) {
	buf := string(bytes.TrimSpace(out))

	i := strings.IndexByte(buf, ':')
	if i < 1 {
		return "", time.Time{}, errors.New("unrecognized VCS tool output")
	}
	rev := buf[:i]

	secs, err := strconv.ParseInt(string(buf[i+1:]), 10, 64)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("unrecognized VCS tool output: %v", err)
	}

	return rev, time.Unix(secs, 0), nil
}

func runGit(dir string, cmdline string) ([]byte, error) {
	args := strings.Fields(cmdline)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return out, err
}
