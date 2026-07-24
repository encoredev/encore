package ecl

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Diagnostic describes a single problem found while parsing, validating,
// or evaluating ECL rules.
type Diagnostic struct {
	Pos     Position // primary location of the problem
	End     Position // optional end of the offending range (same line as Pos)
	Message string   // one-line summary
	Detail  []string // optional additional lines providing context
	Hint    string   // optional remediation suggestion
	Related []RelatedInfo

	src *sourceFile // for snippet rendering; may be nil
}

// RelatedInfo points at a secondary source location that is relevant to a
// diagnostic, such as the other rule involved in a conflict.
type RelatedInfo struct {
	Pos     Position
	Message string
}

// Error returns the full multi-line rendering of the diagnostic,
// including a source snippet when available.
func (d *Diagnostic) Error() string { return d.render() }

// Summary returns a compact one-line form: "file:line:col: message".
func (d *Diagnostic) Summary() string {
	return d.Pos.String() + ": " + d.Message
}

func (d *Diagnostic) render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: error: %s\n", d.Pos, d.Message)
	d.renderSnippet(&b)
	for _, line := range d.Detail {
		fmt.Fprintf(&b, "  %s\n", line)
	}
	for _, r := range d.Related {
		fmt.Fprintf(&b, "  note: %s\n", r.Message)
	}
	if d.Hint != "" {
		fmt.Fprintf(&b, "  help: %s\n", d.Hint)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (d *Diagnostic) renderSnippet(b *strings.Builder) {
	line, ok := d.src.line(d.Pos.Line)
	if !ok {
		return
	}
	num := strconv.Itoa(d.Pos.Line)
	gutter := strings.Repeat(" ", len(num))
	fmt.Fprintf(b, " %s |\n", gutter)
	fmt.Fprintf(b, " %s | %s\n", num, line)

	// Build the caret line, mirroring tabs so the caret aligns with the
	// source line above regardless of tab rendering width.
	var lead strings.Builder
	col := 1
	for _, r := range line {
		if col >= d.Pos.Column {
			break
		}
		if r == '\t' {
			lead.WriteByte('\t')
		} else {
			lead.WriteByte(' ')
		}
		col++
	}
	width := 1
	if d.End.IsValid() && d.End.Line == d.Pos.Line && d.End.Column > d.Pos.Column {
		width = d.End.Column - d.Pos.Column
	}
	fmt.Fprintf(b, " %s | %s%s\n", gutter, lead.String(), strings.Repeat("^", width))
}

// ErrorList is a list of diagnostics. It implements error; all errors
// returned by this package are of this type.
type ErrorList []*Diagnostic

func (l ErrorList) Error() string {
	parts := make([]string, len(l))
	for i, d := range l {
		parts[i] = d.render()
	}
	return strings.Join(parts, "\n\n")
}

// Err returns the list as an error, or nil if the list is empty.
func (l ErrorList) Err() error {
	if len(l) == 0 {
		return nil
	}
	return l
}

func (l ErrorList) sort() {
	sort.SliceStable(l, func(i, j int) bool {
		a, b := l[i].Pos, l[j].Pos
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		return l[i].Message < l[j].Message
	})
}
