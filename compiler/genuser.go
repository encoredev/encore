package compiler

import (
	"bytes"
	"os"
	"path/filepath"

	"encr.dev/parser/est"
)

// GenUser generates user-facing application code.
func GenUser(appRoot string) error {
	b := &builder{
		cfg:     &Config{},
		appRoot: appRoot,
	}
	return b.GenUser()
}

func (b *builder) GenUser() (err error) {
	defer func() {
		if e := recover(); e != nil {
			if b, ok := e.(bailout); ok {
				err = b.err
			} else {
				panic(e)
			}
		}
	}()

	for _, fn := range []func() error{
		b.parseApp,
		b.genUser,
	} {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) genUser() error {
	for _, svc := range b.res.App.Services {
		if err := b.generateUserCode(svc); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) generateUserCode(svc *est.Service) (err error) {
	dst := filepath.Join(b.appRoot, svc.Root.RelPath, "encore.gen.go")
	f := b.codegen.EncoreGen(svc, false)
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
