package errlist

import (
	"encoding/json"
	"fmt"
	"go/scanner"
	"go/token"
	"io"
	"path/filepath"
	"strings"

	"encr.dev/pkg/errinsrc"
	"encr.dev/pkg/errinsrc/srcerrors"
	daemonpb "encr.dev/proto/encore/daemon"
)

// Verbose controls whether the error list prints all errors
// or just the what MaxErrorsToPrint is set to
var Verbose = false

// MaxErrorsToPrint is the maximum number of errors to print
// if Verbose is false
var MaxErrorsToPrint = 1

type List struct {
	List errinsrc.List `json:"list,omitempty"`
	fset *token.FileSet
}

var _ errinsrc.ErrorList = (*List)(nil)

func New(fset *token.FileSet) *List {
	return &List{fset: fset}
}

// Convert attempts to convert known error types into an error list
// if it can't it returns nil
func Convert(err error) *List {
	switch err := err.(type) {
	case *List:
		return err
	case *errinsrc.ErrInSrc:
		l := New(nil)
		l.List = []*errinsrc.ErrInSrc{err}
		return l
	case errinsrc.ErrorList:
		l := New(nil)
		l.List = err.ErrorList()
		return l
	default:
		return nil
	}
}

// Report is a function that allows you to report an error into
// this list, without having to check for nil.
//
// This function only supports error of types:
//   - *List
//   - *errinsrc.ErrInSrc
//   - *scanner.Error
//
// If too many errors have been reported it panics
// with a Bailout value to abort processing.
// Use HandleBailout to conveniently handle this.
func (l *List) Report(err error) {
	if err == nil {
		return
	}

	var errToAdd *errinsrc.ErrInSrc
	switch err := (err).(type) {
	case *errinsrc.ErrInSrc:
		// the base type we expect
		errToAdd = err

	case *scanner.Error:
		// errors directly from the Go parser
		errToAdd = srcerrors.GenericGoParserError(err)

	case scanner.ErrorList:
		for _, e := range err {
			l.Report(e)
		}
		return
	case *List:
		// If it's a different list, then merge it in
		if err != l {
			for _, e := range err.ErrorList() {
				l.Report(e)
			}
		}
		return
	case errinsrc.ErrorList:
		// either another errlist or a list from `srcerrors`
		for _, e := range err.ErrorList() {
			l.Report(e)
		}
		return
	default:
		panic(fmt.Sprintf("unsupported type %T being reported to errlist.List", err))
	}

	// Skip adding this error if it's on the same line as another error
	// we've already reported, since it's probably a spurious error caused
	// by the first one.
	for _, e := range l.List {
		if errToAdd.OnSameLine(e) {
			return
		}
	}

	l.List = append(l.List, errToAdd)
	if len(l.List) > 10 {
		l.Abort()
	}
}

// Add adds an error to the list.
//
// If too many errors have been added it panics
// with a Bailout value to abort processing.
// Use HandleBailout to conveniently handle this.
//
// Deprecated: use Report instead
func (l *List) Add(pos token.Pos, msg string) {
	pp := l.fset.Position(pos)
	l.Report(&scanner.Error{
		Pos: pp,
		Msg: msg,
	})
}

// Addf is equivalent to Add(pos, fmt.Sprintf(format, args...))
//
// Deprecated: use Report instead
func (l *List) Addf(pos token.Pos, format string, args ...interface{}) {
	l.Add(pos, fmt.Sprintf(format, args...))
}

// AddRaw adds a raw *scanner.Error to the list.
//
// If too many errors have been added it panics
// with a Bailout value to abort processing.
// Use HandleBailout to conveniently handle this.
//
// Deprecated: use Report instead
func (l *List) AddRaw(err *scanner.Error) {
	l.Report(err)
}

// Merge merges another list into this one.
// The token.FileSet in use must be the same one as this one,
// or else it panics.
//
// Deprecated: use Report instead
func (l *List) Merge(other *List) {
	if other.fset != l.fset {
		panic("errlist: cannot merge lists with different *token.FileSets")
	}
	l.List = append(l.List, other.List...)
}

// Err returns an error equivalent to this error list.
// If the list is empty, Err returns nil.
func (l *List) Err() error {
	if len(l.List) == 0 {
		return nil
	}
	return l
}

// Error implements the error interface.
func (l *List) Error() string {
	var b strings.Builder

	if Verbose {
		for _, err := range l.List {
			b.WriteString(err.Error())
		}
	} else {
		if len(l.List) == 0 {
			b.WriteString("no errors")
		} else {
			for i, err := range l.List {
				b.WriteString(err.Error())
				if i >= MaxErrorsToPrint-1 {
					break
				}
			}

			if len(l.List) > MaxErrorsToPrint {
				b.WriteString(fmt.Sprintf("And %d more errors", len(l.List)-MaxErrorsToPrint))
			}
		}
	}

	return b.String()
}

func (l *List) ErrorList() []*errinsrc.ErrInSrc {
	return l.List
}

// MakeRelative rewrites the errors by making filenames within the
// app root relative to the relwd (which must be a relative path
// within the root).
func (l *List) MakeRelative(root, relwd string) {
	if l == nil {
		return
	}

	wdroot := filepath.Join(root, relwd)
	for _, e := range l.List {
		for _, loc := range e.Params.Locations {
			if loc.File != nil {
				fn := loc.File.RelPath
				if strings.HasPrefix(fn, root) {
					if rel, err := filepath.Rel(wdroot, fn); err == nil {
						loc.File.RelPath = rel
					}
				}
			}
		}
	}
}

// HandleBailout handles bailouts raised by (*List).Add and family
// when too many errors have been found.
func (l *List) HandleBailout(err *error) {
	if e := recover(); e != nil {
		if b, ok := e.(errinsrc.Bailout); ok {
			*err = b.List
		} else {
			panic(e)
		}
	}
}

func (l *List) Len() int {
	return len(l.List)
}

// Abort aborts early if there is an error in the list.
func (l *List) Abort() {
	errinsrc.Panic(l)
}

// SendToStream sends a GRPC command with this
// full errlist
//
// If l is nil or empty, it sends a nil command
// allowing the client to know that there are no
// longer an error present
func (l *List) SendToStream(stream interface {
	Send(*daemonpb.CommandMessage) error
}) error {
	var bytes []byte
	if l != nil && len(l.List) > 0 {
		var err error
		bytes, err = json.Marshal(l)
		if err != nil {
			panic("unable to marshal error list")
		}
	}
	return stream.Send(
		&daemonpb.CommandMessage{
			Msg: &daemonpb.CommandMessage_Errors{
				Errors: &daemonpb.CommandDisplayErrors{
					Errinsrc: bytes,
				},
			},
		},
	)
}

// Print is a utility function that prints a list of errors to w,
// one error per line, if the err parameter is an errorList. Otherwise
// it prints the err string.
func Print(w io.Writer, err error) {
	if l, ok := err.(*List); ok {
		for _, e := range l.List {
			fmt.Fprintf(w, "%s\n", e)
		}
	} else if err != nil {
		fmt.Fprintf(w, "%s\n", err)
	}
}
