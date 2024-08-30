package cmdutil

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type Oneof struct {
	Value       string
	Allowed     []string
	Flag        string // defaults to "output" if empty
	FlagShort   string // defaults to "o" if both Flag and FlagShort are empty
	Desc        string // usage desc
	TypeDesc    string // type description, defaults to the name of the flag
	NoOptDefVal string // default value when no option is provided
}

func (o *Oneof) AddFlag(cmd *cobra.Command) {
	name, short := o.FlagName()
	cmd.Flags().AddFlag(
		&pflag.Flag{
			Name:        name,
			NoOptDefVal: o.NoOptDefVal,
			Shorthand:   short,
			Usage:       o.Usage(),
			Value:       o,
			DefValue:    o.String(),
		})
	_ = cmd.RegisterFlagCompletionFunc(name, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return o.Allowed, cobra.ShellCompDirectiveNoFileComp
	})
}

func (o *Oneof) FlagName() (name, short string) {
	name, short = o.Flag, o.FlagShort
	if name == "" {
		name, short = "output", "o"
	}
	return name, short
}

func (o *Oneof) String() string {
	return o.Value
}

func (o *Oneof) Type() string {
	if o.TypeDesc != "" {
		return o.TypeDesc
	}
	name, _ := o.FlagName()
	return name
}

func (o *Oneof) Set(v string) error {
	if slices.Contains(o.Allowed, v) {
		o.Value = v
		return nil
	}

	var b strings.Builder
	b.WriteString("must be one of ")
	o.oneOf(&b)
	return errors.New(b.String())
}

func (o *Oneof) Usage() string {
	var b strings.Builder
	desc := o.Desc
	if desc == "" {
		desc = "Output format"
	}
	b.WriteString(desc + ". One of (")
	o.oneOf(&b)
	b.WriteString(").")
	return b.String()
}

// Alternatives lists the alternatives in the format "a|b|c".
func (o *Oneof) Alternatives() string {
	var b strings.Builder
	for i, s := range o.Allowed {
		if i > 0 {
			b.WriteByte('|')
		}
		b.WriteString(s)
	}
	return b.String()
}

func (o *Oneof) oneOf(b *strings.Builder) {
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
