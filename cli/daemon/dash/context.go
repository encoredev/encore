package dash

import (
	"bufio"
	"io"
)

// sourceContext returns N lines of context around line.
func sourceContext(r io.Reader, line, N int) (lines []string, start int, err error) {
	s := bufio.NewScanner(r)
	start = line - N
	if start < 1 {
		start = 1
	}

	// Skip forward to start
	for i := 1; i < start; i++ {
		s.Scan()
	}

	lines = make([]string, 0, 2*N+1)
	for i := start; i <= line+N && s.Scan(); i++ {
		lines = append(lines, s.Text())
	}
	if err := s.Err(); err != nil {
		return nil, 0, err
	}
	return lines, start, nil
}
