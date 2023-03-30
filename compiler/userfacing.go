package compiler

import (
	"bytes"
	"os"
	"path/filepath"

	"encr.dev/parser"
	"encr.dev/parser/est"
	"encr.dev/pkg/errinsrc/srcerrors"
)

// GenUserFacing generates user-facing application code.
func GenUserFacing(appRoot string, res *parser.Result) error {
	b := &builder{
		cfg: &Config{
			Parse: res,
		},
		appRoot: appRoot,
		res:     res,
	}
	return b.GenUserFacing()
}

func (b *builder) GenUserFacing() (err error) {
	defer func() {
		if e := recover(); e != nil {
			if b, ok := e.(bailout); ok {
				err = b.err
			} else {
				err = srcerrors.UnhandledPanic(e)
			}
		}
	}()

	for _, fn := range []func() error{
		b.parseApp,
		b.genUserFacing,
	} {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) genUserFacing() error {
	for _, svc := range b.res.App.Services {
		if err := b.generateUserFacingGoCode(svc); err != nil {
			return err
		}

		if err := b.generateUserFacingCueCode(svc); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) generateUserFacingGoCode(svc *est.Service) (err error) {
	dst := filepath.Join(b.appRoot, svc.Root.RelPath, "encore.gen.go")
	f := b.codegen.UserFacing(svc, false)
	if f == nil {
		// No need for any generated code. Try to remove the existing file
		// if it's there as it's no longer needed.
		_ = os.Remove(dst)
		return nil
	}

	var buf bytes.Buffer
	if err := f.Render(&buf); err != nil {
		return err
	}

	return os.WriteFile(dst, buf.Bytes(), 0644)
}

func (b *builder) generateUserFacingCueCode(svc *est.Service) (err error) {
	dst := filepath.Join(b.appRoot, svc.Root.RelPath, "encore.gen.cue")
	f, err := b.cuegen.UserFacing(svc)
	if err != nil {
		return err
	}

	if f == nil || len(f) == 0 {
		// No need for any generated code. Try to remove the existing file
		// if it's there as it's no longer needed.
		_ = os.Remove(dst)
		return nil
	}

	return os.WriteFile(dst, f, 0644)
}
