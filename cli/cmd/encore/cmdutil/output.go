package cmdutil

import (
	"errors"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"golang.org/x/exp/slices"
)

type Output struct {
	Value   string
	Allowed []string
}

func (o *Output) AddFlag(s *pflag.FlagSet) {
	s.VarP(o, "output", "o", o.Usage())
}

func (o *Output) String() string {
	return o.Value
}

func (o *Output) Type() string {
	return "output"
}

func (o *Output) Set(v string) error {
	if slices.Contains(o.Allowed, v) {
		o.Value = v
		return nil
	}

	var b strings.Builder
	b.WriteString("must be one of ")
	o.oneOf(&b)
	return errors.New(b.String())
}

func (o *Output) Usage() string {
	var b strings.Builder
	b.WriteString("Output format. One of (")
	o.oneOf(&b)
	b.WriteString(").")
	return b.String()
}

func (o *Output) oneOf(b *strings.Builder) {
	n := len(o.Allowed)
	for i, s := range o.Allowed {
		if i > 0 {
			switch {
			case n == 2:
				b.WriteString(" or ")
			case i == n-1:
				b.WriteString(", or ")
			default:
				b.WriteString(", ")
			}
		}

		b.WriteString(strconv.Quote(s))
	}
}
