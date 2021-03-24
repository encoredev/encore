package runlog

import (
	"io"
	"os"
)

type Log interface {
	Stdout() io.Writer
	Stderr() io.Writer
}

type oslog struct{}

func (oslog) Stdout() io.Writer { return os.Stdout }
func (oslog) Stderr() io.Writer { return os.Stderr }

func OS() Log {
	return oslog{}
}
