package dash

import (
	"strconv"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestSourceContext(t *testing.T) {
	tests := []struct {
		line, n int
		start   int
		lines   []string
	}{
		{1, 2, 1, []string{"1", "2", "3"}},
		{2, 2, 1, []string{"1", "2", "3", "4"}},
		{3, 2, 1, []string{"1", "2", "3", "4", "5"}},
		{4, 2, 2, []string{"2", "3", "4", "5", "6"}},
		{15, 2, 13, []string{"13", "14", "15"}},
		{20, 2, 18, []string{}},
	}
	c := qt.New(t)

	for i, test := range tests {
		c.Run(strconv.Itoa(i), func(c *qt.C) {
			lines, start, err := sourceContext(strings.NewReader(data), test.line, test.n)
			c.Assert(err, qt.IsNil)
			c.Assert(start, qt.Equals, test.start)
			c.Assert(lines, qt.DeepEquals, test.lines)
		})
	}
}

var data = strings.ReplaceAll("1 2 3 4 5 6 7 8 9 10 11 12 13 14 15", " ", "\n")
