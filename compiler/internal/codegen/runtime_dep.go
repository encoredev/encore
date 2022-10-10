package codegen

import (
	. "github.com/dave/jennifer/jen"

	"encr.dev/parser/est"
)

func (b *Builder) ForceRuntimeDependency(pkg *est.Package) (f *File, err error) {
	defer b.errors.HandleBailout(&err)
	f = NewFilePathName(pkg.ImportPath, pkg.Name)
	f.Anon("encore.dev/appruntime/app/appinit")
	return f, b.errors.Err()
}
