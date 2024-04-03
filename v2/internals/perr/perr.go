// Package perr provides utilities for handling parse errors.
package perr

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/scanner"
	"go/token"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"

	"encr.dev/pkg/errinsrc"
	"encr.dev/pkg/errinsrc/srcerrors"
	"encr.dev/pkg/errlist"
	"encr.dev/pkg/errors"
	"encr.dev/pkg/paths"
	daemonpb "encr.dev/proto/encore/daemon"
)

// NewList constructs a new list.
//
// It takes a ctx to add an error on context cancellation
// since code often uses ctx cancellation to cause a bailout.
func NewList(ctx context.Context, fset *token.FileSet, fileReaders ...paths.FileReader) *List {
	return &List{ctx: ctx, fset: fset, fileReaders: fileReaders}
}

// List is a list of errors.
// The same instance is shared between different components.
type List struct {
	ctx            context.Context
	fset           *token.FileSet
	fileReaders    []paths.FileReader
	ignoreBailouts bool

	mu   sync.Mutex
	errs errinsrc.List
}

func (l *List) SetIgnoreBailouts(val bool) *List {
	l.ignoreBailouts = val
	return l
}

// AsError returns this list an error if there are
// errors in the list, otherwise it returns nil.
func (l *List) AsError() error {
	if l.Len() == 0 {
		return nil
	}

	return &ListAsErr{list: l}
}

// Add adds a templated error
func (l *List) Add(template errors.Template) {
	l.add(errinsrc.FromTemplate(template, l.fset, l.fileReaders...))
}

// Add adds an error at the given pos.
func (l *List) AddPos(pos token.Pos, msg string) {
	l.add(srcerrors.GenericError(l.fset.Position(pos), msg, l.fileReaders...))
}

// Addf is equivalent to l.Add(pos, fmt.Sprintf(format, args...))
func (l *List) Addf(pos token.Pos, format string, args ...any) {
	l.AddPos(pos, fmt.Sprintf(format, args...))
}

// AddStd adds an error from the stdlib packages that uses
// scanner.ErrorList or *scanner.Error under the hood.
func (l *List) AddStd(err error) {
	if err == nil {
		return
	}

	switch err := err.(type) {
	case *errinsrc.ErrInSrc:
		l.add(err)

	case errinsrc.ErrorList:
		for _, e := range err.ErrorList() {
			l.add(e)
		}

	case *scanner.Error:
		l.add(srcerrors.GenericGoParserError(err, l.fileReaders...))

	case scanner.ErrorList:
		for _, e := range err {
			l.add(srcerrors.GenericGoParserError(e, l.fileReaders...))
		}

	case packages.Error:
		l.add(srcerrors.GenericGoPackageError(err, l.fileReaders...))

	default:
		l.add(srcerrors.StandardLibraryError(err))
	}
}

func (l *List) AddStdNode(err error, node ast.Node) {
	if err == nil {
		return
	}

	add := func(err *errinsrc.ErrInSrc) {
		err.WithGoNode(l.fset, node, l.fileReaders...)
		l.add(err)
	}

	switch err := err.(type) {
	case *errinsrc.ErrInSrc:
		add(err)

	case errinsrc.ErrorList:
		for _, e := range err.ErrorList() {
			add(e)
		}

	case *scanner.Error:
		add(srcerrors.GenericGoParserError(err, l.fileReaders...))

	case scanner.ErrorList:
		for _, e := range err {
			add(srcerrors.GenericGoParserError(e, l.fileReaders...))
		}

	case packages.Error:
		add(srcerrors.GenericGoPackageError(err, l.fileReaders...))

	default:
		add(srcerrors.StandardLibraryError(err))
	}
}

func (l *List) Fatal(pos token.Pos, msg string) {
	l.AddPos(pos, msg)
	l.Bailout()
}

func (l *List) Fatalf(pos token.Pos, format string, args ...any) {
	l.Addf(pos, format, args...)
	l.Bailout()
}

func (l *List) Assert(template errors.Template) {
	l.Add(template)
	l.Bailout()
}

func (l *List) AssertStd(err error) {
	if err != nil {
		l.AddStd(err)
		l.Bailout()
	}
}

// Len returns the number of errors reported.
func (l *List) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := len(l.errs)

	if err := l.ctx.Err(); err != nil {
		n++
	}
	return n
}

// At returns the i'th error. i must be 0 <= i < l.Len().
func (l *List) At(i int) *errinsrc.ErrInSrc {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.errs[i]
}

func (l *List) FS() *token.FileSet {
	return l.fset
}

func (l *List) add(e *errinsrc.ErrInSrc) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errs = append(l.errs, e)
}

// FormatErrors formats the errors as a newline-separated string.
// If there are no errors it returns "no errors".
func (l *List) FormatErrors() string {
	if n := l.Len(); n == 0 {
		return "no errors"
	}

	var b strings.Builder
	for _, err := range l.errs {
		fmt.Fprintf(&b, "%s\n", err.Error())
	}
	if err := l.ctx.Err(); err != nil {
		fmt.Fprintf(&b, "%s\n", err.Error())
	}
	return b.String()
}

func (l *List) GoString() string {
	return "&perr.List{...}"
}

// MakeRelative rewrites the errors by making filenames within the
// app root relative to the relwd (which must be a relative path
// within the root).
func (l *List) MakeRelative(root, relwd string) {
	wdroot := filepath.Join(root, relwd)
	for _, e := range l.errs {
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
	if l != nil && len(l.errs) > 0 {
		// TODO: For now we use the older errlist format in JSON to preserve
		// backward compatibility with the daemon while it could receive
		// errors from v1 or v2.
		//
		// Once we remove v1, we can remove this shim
		errList := errlist.New(l.fset)
		errList.List = l.errs

		var err error
		bytes, err = json.Marshal(errList)
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

// BailoutOnErrors calls fn and bailouts if fn reports any errors.
func (l *List) BailoutOnErrors(fn func()) {
	n := l.Len()
	fn()
	if l.Len() > n {
		l.Bailout()
	}
}

func (l *List) Bailout() {
	if l.ignoreBailouts {
		return
	}
	panic(bailout{l})
}

type bailout struct{ l *List }

func (b bailout) GoString() string {
	return fmt.Sprintf("perr.bailout: %s", b.l.FormatErrors())
}

func (b bailout) String() string {
	return fmt.Sprintf("perr.bailout: %s", b.l.FormatErrors())
}

// CatchBailout catches a bailout panic and reports whether there was one.
// If true it also returns the error list that caused the bailout.
// Intended usage is:
//
//	  if l, ok := perr.CatchBailout(recover()); ok {
//		// handle bailout
//	  }
func CatchBailout(recovered any) (l *List, ok bool) {
	if recovered != nil {
		if b, ok := recovered.(bailout); ok {
			return b.l, true
		} else {
			panic(recovered)
		}
	}
	return nil, false
}

// CatchBailoutAndPanic is like CatchBailout but also catches other panics.
// In both types of panics it converts the result to an error.
//
// If there is no panic it returns the error provided in the first argument
// back to the caller. That way, it can usually be used like so
// (inside a deferred function):
//
//	err, _ = perr.CatchBailoutAndPanic(err, recover())
func CatchBailoutAndPanic(currentErr error, recovered any) (err error, caught bool) {
	if recovered == nil {
		return currentErr, false
	}

	if b, ok := recovered.(bailout); ok {
		err = b.l.AsError()
	} else {
		err = srcerrors.UnhandledPanic(recovered)
	}
	return err, true
}

// IsBailout reports whether a recovered value is a bailout panic.
// It reports the list that caused the bailout alongside.
func IsBailout(recovered any) (l *List, ok bool) {
	if b, ok := recovered.(bailout); ok {
		return b.l, true
	}
	return nil, false
}
