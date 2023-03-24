package scan

import (
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"encr.dev/internal/paths"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
)

// Packages scans and parses the Go packages for all subdirectories in the root.
func Packages(quit chan struct{}, errs *perr.List, l *pkginfo.Loader, root paths.FS, basePkgPath paths.Pkg) <-chan *pkginfo.Package {
	// a worker accepts work in the form of package paths to parse
	// and sends the parsed results back on the results channel.
	// It calls wg.Done() when it's done.
	worker := func(wg *sync.WaitGroup, work <-chan paths.Pkg, results chan<- *pkginfo.Package) {
		defer wg.Done()
		for pkgPath := range work {
			if pkg, ok := l.LoadPkg(token.NoPos, pkgPath); ok {
				select {
				case results <- pkg:
				case <-quit:
					return // we're done
				}
			}
		}
	}

	// Enqueue all the directories to parse
	work := make(chan paths.Pkg, 100)
	go func() {
		defer close(work) // no more work when we're done
		err := walkGoPackages(root, basePkgPath, func(pkgPath paths.Pkg) {
			select {
			case work <- pkgPath:
			case <-quit:
				return // we're done
			}
		})
		if err != nil {
			errs.AddStd(err)
		}
	}()

	// Start the workers. One per GOMAXPROCS, but at least 4
	numWorkers := runtime.GOMAXPROCS(0)
	if numWorkers < 4 {
		numWorkers = 4
	}
	results := make(chan *pkginfo.Package, numWorkers)
	var activeWorkers sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		activeWorkers.Add(1)
		go worker(&activeWorkers, work, results)
	}

	// When all the workers are done, close the results channel
	go func() {
		activeWorkers.Wait()
		close(results)
	}()

	return results
}

type walkFunc func(pkgPath paths.Pkg)

// walkGoPackages recursively walks all subdirectories of root,
// calling walkFn for each directory that contains a go package
// (as indicated by the presence of any .go files).
func walkGoPackages(root paths.FS, basePkgPath paths.Pkg, walkFn walkFunc) error {
	return walkDir(root.ToIO(), basePkgPath, walkFn)
}

func walkDir(dir string, pkgPath paths.Pkg, walkFn walkFunc) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	// Iterate through the entries and keep track of any directories
	// we come across as well as whether there are any Go files.
	var subdirs []string
	foundGoFile := false
	for _, e := range entries {
		name := e.Name()
		if ignored(name) {
			continue
		} else if e.IsDir() {
			subdirs = append(subdirs, name)
		} else if !foundGoFile {
			// Only compute if we haven't already found a .go file
			foundGoFile = strings.HasSuffix(name, ".go")
		}
	}

	if foundGoFile {
		walkFn(pkgPath)
	}
	for _, d := range subdirs {
		subDir := filepath.Join(dir, d)
		subPkg := pkgPath.JoinSlash(paths.RelSlash(d))
		if err := walkDir(subDir, subPkg, walkFn); err != nil {
			return err
		}
	}
	return nil
}

// ignored returns true if a given directory should be ignored for parsing.
func ignored(dir string) bool {
	name := filepath.Base(filepath.Clean(dir))
	if strings.EqualFold(name, "node_modules") {
		return true
	}
	// Don't watch hidden folders like `.git` or `.idea`.
	if len(name) > 1 && name[0] == '.' {
		return true
	}
	return false
}
