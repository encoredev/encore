/*
This package contains code from https://github.com/mvdan/sh.

Copyright (c) 2016, Daniel Martí. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of the copyright holder nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

package lookpath

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

func checkStat(dir, file string, checkExec bool) (string, error) {
	if !filepath.IsAbs(file) {
		file = filepath.Join(dir, file)
	}
	info, err := os.Stat(file)
	if err != nil {
		return "", err
	}
	m := info.Mode()
	if m.IsDir() {
		return "", fmt.Errorf("is a directory")
	}
	if checkExec && runtime.GOOS != "windows" && m&0o111 == 0 {
		return "", fmt.Errorf("permission denied")
	}
	return file, nil
}

func winHasExt(file string) bool {
	i := strings.LastIndex(file, ".")
	if i < 0 {
		return false
	}
	return strings.LastIndexAny(file, `:\/`) < i
}

// findExecutable returns the path to an existing executable file.
func findExecutable(dir, file string, exts []string) (string, error) {
	if len(exts) == 0 {
		// non-windows
		return checkStat(dir, file, true)
	}
	if winHasExt(file) {
		if file, err := checkStat(dir, file, true); err == nil {
			return file, nil
		}
	}
	for _, e := range exts {
		f := file + e
		if f, err := checkStat(dir, f, true); err == nil {
			return f, nil
		}
	}
	return "", fmt.Errorf("not found")
}

// findFile returns the path to an existing file.
func findFile(dir, file string, _ []string) (string, error) {
	return checkStat(dir, file, false)
}

// InDir is similar to [os/exec.LookPath], with the difference that it uses the
// provided environment. env is used to fetch relevant environment variables
// such as PWD and PATH.
//
// If no error is returned, the returned path must be valid.
func InDir(cwd string, env []string, file string) (string, error) {
	if filepath.IsAbs(file) {
		return file, nil
	}

	upper := runtime.GOOS == "windows"
	envs := listEnvironWithUpper(upper, env...)
	return lookPathDir(cwd, envs, file, findExecutable)
}

// findAny defines a function to pass to lookPathDir.
type findAny = func(dir string, file string, exts []string) (string, error)

func lookPathDir(cwd string, env listEnviron, file string, find findAny) (string, error) {
	if find == nil {
		panic("no find function found")
	}

	pathList := filepath.SplitList(env.Get("PATH"))
	if len(pathList) == 0 {
		pathList = []string{""}
	}
	chars := `/`
	if runtime.GOOS == "windows" {
		chars = `:\/`
	}
	exts := pathExts(env)
	if strings.ContainsAny(file, chars) {
		return find(cwd, file, exts)
	}
	for _, elem := range pathList {
		var path string
		switch elem {
		case "", ".":
			// otherwise "foo" won't be "./foo"
			path = "." + string(filepath.Separator) + file
		default:
			path = filepath.Join(elem, file)
		}
		if f, err := find(cwd, path, exts); err == nil {
			return f, nil
		}
	}
	return "", fmt.Errorf("%q: executable file not found in $PATH", file)
}

func pathExts(env listEnviron) []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	pathext := env.Get("PATHEXT")
	if pathext == "" {
		return []string{".com", ".exe", ".bat", ".cmd"}
	}
	var exts []string
	for _, e := range strings.Split(strings.ToLower(pathext), `;`) {
		if e == "" {
			continue
		}
		if e[0] != '.' {
			e = "." + e
		}
		exts = append(exts, e)
	}
	return exts
}

// listEnvironWithUpper implements ListEnviron, but letting the tests specify
// whether to uppercase all names or not.
func listEnvironWithUpper(upper bool, pairs ...string) listEnviron {
	list := slices.Clone(pairs)
	if upper {
		// Uppercase before sorting, so that we can remove duplicates
		// without the need for linear search nor a map.
		for i, s := range list {
			if sep := strings.IndexByte(s, '='); sep > 0 {
				list[i] = strings.ToUpper(s[:sep]) + s[sep:]
			}
		}
	}

	slices.SortStableFunc(list, func(a, b string) int {
		isep := strings.IndexByte(a, '=')
		jsep := strings.IndexByte(b, '=')
		if isep < 0 {
			isep = 0
		} else {
			isep += 1
		}
		if jsep < 0 {
			jsep = 0
		} else {
			jsep += 1
		}
		return strings.Compare(a[:isep], b[:jsep])
	})

	last := ""
	for i := 0; i < len(list); {
		s := list[i]
		sep := strings.IndexByte(s, '=')
		if sep <= 0 {
			// invalid element; remove it
			list = append(list[:i], list[i+1:]...)
			continue
		}
		name := s[:sep]
		if last == name {
			// duplicate; the last one wins
			list = append(list[:i-1], list[i:]...)
			continue
		}
		last = name
		i++
	}
	return listEnviron(list)
}

// listEnviron is a sorted list of "name=value" strings.
type listEnviron []string

func (l listEnviron) Get(name string) string {
	eqpos := len(name)
	endpos := len(name) + 1
	i, ok := slices.BinarySearchFunc(l, name, func(l, name string) int {
		if len(l) < endpos {
			// Too short; see if we are before or after the name.
			return strings.Compare(l, name)
		}
		// Compare the name prefix, then the equal character.
		c := strings.Compare(l[:eqpos], name)
		eq := l[eqpos]
		if c == 0 {
			return cmp.Compare(eq, '=')
		}
		return c
	})
	if ok {
		return l[i][endpos:]
	}
	return ""
}
