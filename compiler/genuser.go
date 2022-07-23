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
	f := b.codegen.EncoreGen(svc)
	if f == nil {
		// Nothing to do
		return nil
	}

	var buf bytes.Buffer
	if err := f.Render(&buf); err != nil {
		return err
	}

	dst := filepath.Join(b.appRoot, svc.Root.RelPath, "encore.gen.go")
	return os.WriteFile(dst, buf.Bytes(), 0644)
}
