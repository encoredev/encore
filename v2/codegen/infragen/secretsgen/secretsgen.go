package secretsgen

import (
	"bytes"
	"fmt"
	"strconv"

	"encr.dev/v2/codegen"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/parser/infra/secrets"
)

func Gen(gen *codegen.Generator, pkg *pkginfo.Package, secrets []*secrets.Secrets) {
	addedImport := make(map[*pkginfo.File]bool)
	for _, secret := range secrets {
		file := secret.File
		rw := gen.Rewrite(file)

		if !addedImport[file] {
			// Add an import of the runtime package to be able to load secrets.
			//
			// We also import appinit to ensure the runtime is initialized before
			// we try to load secrets.
			// NOTE(andre): We should make the secrets package instead initialize itself.
			decl := file.AST().Decls[0]
			ln := gen.FS.Position(decl.Pos())
			rw.Insert(decl.Pos(), []byte(fmt.Sprintf("import __encore_secrets %s; import _ %s\n/*line :%d:%d*/",
				strconv.Quote("encore.dev/appruntime/secrets"), strconv.Quote("encore.dev/appruntime/app/appinit"),
				ln.Line, ln.Column)))
			addedImport[secret.File] = true
		}

		// Rewrite the value spec to load the secrets.
		spec := secret.Spec
		var buf bytes.Buffer
		buf.WriteString("{\n")
		for _, key := range secret.Keys {
			fmt.Fprintf(&buf, "\t%s: __encore_secrets.Load(%s),\n", key, strconv.Quote(key))
		}
		ep := gen.FS.Position(spec.End())
		fmt.Fprintf(&buf, "}/*line :%d:%d*/", ep.Line, ep.Column)
		rw.Insert(spec.Type.Pos(), []byte("= "))
		rw.Insert(spec.End(), buf.Bytes())

	}
}
