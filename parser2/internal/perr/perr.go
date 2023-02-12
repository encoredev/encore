// Package perr provides utilities for handling parse errors.
package perr

import (
	"context"
	"fmt"
	"go/scanner"
	"go/token"
	"strings"
	"sync"
)

// NewList constructs a new list.
//
// It takes a ctx to add an error on context cancellation
// since code often uses ctx cancellation to cause a bailout.
func NewList(ctx context.Context, fset *token.FileSet) *List {
	return &List{ctx: ctx, fset: fset}
}

// List is a list of errors.
// The same instance is shared between different components.
type List struct {
	ctx  context.Context
	fset *token.FileSet

	mu   sync.Mutex
	errs []*Error
}

// Add adds an error at the given pos.
func (l *List) Add(pos token.Pos, msg string) {
	l.add(&Error{
		Pos: l.fset.Position(pos),
		Msg: msg,
	})
}

// Addf is equivalent to l.Add(pos, fmt.Sprintf(format, args...))
func (l *List) Addf(pos token.Pos, format string, args ...any) {
	l.Add(pos, fmt.Sprintf(format, args...))
}

// AddPosition adds an error at the given token.Position.
func (l *List) AddPosition(pos token.Position, msg string) {
	l.add(&Error{
		Pos: pos,
		Msg: msg,
	})
}

// AddForFile adds an error for a given filename.
// If the error is an std error (scanner.ErrorList or *scanner.Error)
// that file information is used instead.
func (l *List) AddForFile(err error, filename string) {
	switch err.(type) {
	case nil:
		// do nothing
	case *scanner.Error, scanner.ErrorList:
		l.AddStd(err)
	default:
		l.AddPosition(token.Position{Filename: filename}, err.Error())
	}
}

// AddStd adds an error from the stdlib packages that uses
// scanner.ErrorList or *scanner.Error under the hood.
func (l *List) AddStd(err error) {
	if err == nil {
		return
	}

	if err, ok := err.(*scanner.Error); ok {
		l.add(&Error{
			Pos: err.Pos,
			Msg: err.Msg,
		})
		return
	}

	switch err := err.(type) {
	case *scanner.Error:
		l.add(&Error{Pos: err.Pos, Msg: err.Msg})
	case scanner.ErrorList:
		for _, e := range err {
			l.add(&Error{
				Pos: e.Pos,
				Msg: e.Msg,
			})
		}
	default:
		l.add(&Error{Msg: err.Error()})
	}
}

func (l *List) Fatalf(pos token.Pos, format string, args ...any) {
	l.Addf(pos, format, args...)
	l.Bailout()
}

func (l *List) Assert(err error, pos token.Pos) {
	l.AssertPosition(err, l.fset.Position(pos))
}

func (l *List) AssertStd(err error) {
	if err != nil {
		l.AddStd(err)
		l.Bailout()
	}
}

func (l *List) AssertPosition(err error, pos token.Position) {
	switch err := err.(type) {
	case nil:
		// do nothing
	case *scanner.Error:
		l.AddStd(err)
		l.Bailout()
	case scanner.ErrorList:
		if err.Len() > 0 {
			l.AddStd(err)
			l.Bailout()
		}
	default:
		l.AddPosition(pos, err.Error())
		l.Bailout()
	}
}

func (l *List) AssertFile(err error, filename string) {
	l.AssertPosition(err, token.Position{Filename: filename})
}

func (l *List) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := len(l.errs)

	if err := l.ctx.Err(); err != nil {
		n++
	}
	return n
}

func (l *List) add(e *Error) {
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
		fmt.Fprintf(&b, "%s\n", err)
	}
	if err := l.ctx.Err(); err != nil {
		fmt.Fprintf(&b, "%s\n", err)
	}
	return b.String()
}

// BailoutOnErrors calls fn and bailouts if fn reports any errors.
func (l *List) BailoutOnErrors(fn func()) {
	n := l.Len()
	fn()
	if l.Len() > n {
		l.Bailout()
	}
}

// An Error represents an error encountered during parsing.
// The position Pos, if valid, points to the beginning of
// the offending token, and the error condition is described
// by Msg.
type Error struct {
	Pos token.Position
	Msg string
}

// Error implements the error interface.
func (e Error) Error() string {
	if e.Pos.Filename != "" || e.Pos.IsValid() {
		return e.Pos.String() + ": " + e.Msg
	}
	return e.Msg
}

func (l *List) Bailout() {
	panic(bailout{l})
}

type bailout struct{ l *List }

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

// IsBailout reports whether a recovered value is a bailout panic.
// It reports the list that caused the bailout alongside.
func IsBailout(recovered any) (l *List, ok bool) {
	if b, ok := recovered.(bailout); ok {
		return b.l, true
	}
	return nil, false
}
