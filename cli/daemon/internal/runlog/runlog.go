package runlog

import (
	"io"
	"os"
)

type Log interface {
	Stdout(buffered bool) io.Writer
	Stderr(buffered bool) io.Writer
}

type oslog struct{}

func (oslog) Stdout(buffered bool) io.Writer { return os.Stdout }
func (oslog) Stderr(buffered bool) io.Writer { return os.Stderr }

func OS() Log {
	return oslog{}
}
