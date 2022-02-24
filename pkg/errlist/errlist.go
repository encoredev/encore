package errlist

import (
	"fmt"
	"go/scanner"
	"go/token"
	"io"
	"path/filepath"
	"strings"
)

type List struct {
	list scanner.ErrorList
	fset *token.FileSet
}

func New(fset *token.FileSet) *List {
	return &List{fset: fset}
}

// Add adds an error to the list.
//
// If too many errors have been added it panics
// with a Bailout value to abort processing.
// Use HandleBailout to conveniently handle this.
func (l *List) Add(pos token.Pos, msg string) {
	pp := l.fset.Position(pos)
	n := len(l.list)
	if n > 0 && l.list[n-1].Pos.Line == pp.Line {
		return // spurious
	} else if n > 10 {
		panic(Bailout{err: l})
	}
	addErrToList(&l.list, pp, msg)
}

// Addf is equivalent to Add(pos, fmt.Sprintf(format, args...))
func (l *List) Addf(pos token.Pos, format string, args ...interface{}) {
	l.Add(pos, fmt.Sprintf(format, args...))
}

// AddRaw adds a raw *scanner.Error to the list.
//
// If too many errors have been added it panics
// with a Bailout value to abort processing.
// Use HandleBailout to conveniently handle this.
func (l *List) AddRaw(err *scanner.Error) {
	n := len(l.list)
	if n > 0 && l.list[n-1].Pos.Line == err.Pos.Line {
		return // spurious
	} else if n > 10 {
		panic(Bailout{err: l})
	}
	addErrToList(&l.list, err.Pos, err.Msg)
}

// Merge merges another list into this one.
// The token.FileSet in use must be the same one as this one,
// or else it panics.
func (l *List) Merge(other *List) {
	if other.fset != l.fset {
		panic("errlist: cannot merge lists with different *token.FileSets")
	}
	l.list = append(l.list, other.list...)
}

// Err returns an error equivalent to this error list.
// If the list is empty, Err returns nil.
func (l *List) Err() error {
	if len(l.list) == 0 {
		return nil
	}
	return l
}

// Error implements the error interface.
func (l *List) Error() string {
	return l.list.Error()
}

// Sort sorts the error list.
func (l *List) Sort() {
	l.list.Sort()
}

// MakeRelative rewrites the errors by making filenames within the
// app root relative to the relwd (which must be a relative path
// within the root).
func (l *List) MakeRelative(root, relwd string) {
	wdroot := filepath.Join(root, relwd)
	for _, e := range l.list {
		fn := e.Pos.Filename
		if strings.HasPrefix(fn, root) {
			if rel, err := filepath.Rel(wdroot, fn); err == nil {
				e.Pos.Filename = rel
			}
		}
	}
}

// HandleBailout handles bailouts raised by (*List).Add and family
// when too many errors have been found.
func (l *List) HandleBailout(err *error) {
	if e := recover(); e != nil {
		if b, ok := e.(Bailout); ok {
			*err = b.err
		} else {
			panic(e)
		}
	}
}

func (l *List) Len() int {
	return len(l.list)
}

// Abort aborts early if there is an error in the list.
func (l *List) Abort() {
	panic(Bailout{err: l})
}

// Bailout is a sentinel type for panics that indicate
// to immediately stop processing because too many errors
// have been found. It can conveniently be handled by HandleBailout.
type Bailout struct{ err error }

// Print is a utility function that prints a list of errors to w,
// one error per line, if the err parameter is an ErrorList. Otherwise
// it prints the err string.
func Print(w io.Writer, err error) {
	if l, ok := err.(*List); ok {
		scanner.PrintError(w, l.list)
	} else if err != nil {
		fmt.Fprintf(w, "%s\n", err)
	}
}
