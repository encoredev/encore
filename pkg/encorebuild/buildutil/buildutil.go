package buildutil

import (
	"fmt"
	"os/exec"
	"sync"

	"github.com/cockroachdb/errors"
)

type Bailout struct {
	Err error
}

func (b Bailout) Error() string {
    return b.Err.Error()
}

func Bail(err error) {
	panic(Bailout{err})
}

func Bailf(format string, args ...any) {
	Bail(fmt.Errorf(format, args...))
}

func Must[T any](val T, err error) T {
	if err != nil {
		Bail(err)
	}
	return val
}

func Check(err error) {
	if err != nil {
		Bail(err)
	}
}

func TarGzip(srcDirectory string, tarFile string) error {
	// Create the tar.gz file from the src directory
	cmd := exec.Command("tar", "-czf", tarFile, "-C", srcDirectory, ".")
	// nosemgrep
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrapf(err, "failed to create tar.gz: %s", out)
	}
	return nil
}

// RunParallel runs the given functions in parallel, bailing with the first error
func RunParallel(functions ...func()) {
	var wg sync.WaitGroup
	wg.Add(len(functions))
	var firstErr error
	var mu sync.Mutex

	for _, f := range functions {
		f := f
		go func() {
			defer wg.Done()

			defer func() {
				if err := recover(); err != nil {
					if b, ok := err.(Bailout); ok {
						mu.Lock()
						defer mu.Unlock()
						if firstErr == nil {
							firstErr = b.Err
						}
					} else {
						panic(err)
					}
				}
			}()

			f()
		}()
	}

	wg.Wait()

	Check(firstErr)
}
