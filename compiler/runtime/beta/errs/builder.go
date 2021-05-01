package errs

import (
	"fmt"

	"encore.dev/internal/stack"
)

type Builder struct {
	code    ErrCode
	codeSet bool
	det     ErrDetails
	detSet  bool

	msg  string
	meta []interface{}
	err  error
}

func B() *Builder { return &Builder{} }

func (b *Builder) Code(c ErrCode) *Builder {
	b.code = c
	b.codeSet = true
	return b
}

func (b *Builder) Msg(msg string) *Builder {
	b.msg = msg
	return b
}

func (b *Builder) Msgf(format string, args ...interface{}) *Builder {
	b.msg = fmt.Sprintf(format, args...)
	return b
}

func (b *Builder) Meta(metaPairs ...interface{}) *Builder {
	b.meta = append(b.meta, metaPairs...)
	return b
}

func (b *Builder) Details(det ErrDetails) *Builder {
	b.det = det
	b.detSet = true
	return b
}

func (b *Builder) Cause(err error) *Builder {
	b.err = err
	if e, ok := err.(*Error); ok {
		if !b.codeSet {
			b.code = e.Code
		}
		if !b.detSet {
			b.det = e.Details
		}
	}
	return b
}

func (b *Builder) Err() error {
	code := b.code
	if code == OK {
		code = Unknown
	}

	msg := b.msg
	if msg == "" && b.err == nil {
		msg = "unknown error"
	}

	var errMeta Metadata
	var s stack.Stack
	if e, ok := b.err.(*Error); ok {
		errMeta = e.Meta
		s = e.stack
	} else {
		s = stack.Build(2)
	}

	return &Error{
		Code:       code,
		Message:    msg,
		Meta:       mergeMeta(errMeta, b.meta),
		Details:    b.det,
		underlying: b.err,
		stack:      s,
	}
	return nil
}
