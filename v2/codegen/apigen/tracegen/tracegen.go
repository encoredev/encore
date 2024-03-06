package tracegen

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/codegen/internal/rewrite"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/parser/apis/tracedfunc"
	"encr.dev/v2/parser/resource"
)

func Gen(gen *codegen.Generator, appDesc *app.Desc) {
	for _, fn := range collectTracedFunctions(appDesc) {
		rewriteTracedFunc(gen, fn)
	}
}

func rewriteTracedFunc(gen *codegen.Generator, tf *tracedfunc.TracedFunc) {
	rw := gen.Rewrite(tf.File)

	importName := getOrAddTracingImport(gen, rw, tf.File)

	insertPos := tf.AST.Body.Pos() + 1
	ln := gen.FS.Position(insertPos)

	withTypeAttribute := ""
	switch tf.Type {
	case tracedfunc.Internal:
		// no-op this is default
	case tracedfunc.RequestHandler:
		withTypeAttribute = "AsRequestHandler"
	case tracedfunc.Call:
		withTypeAttribute = "AsCall"
	case tracedfunc.Producer:
		withTypeAttribute = "AsProducer"
	case tracedfunc.Consumer:
		withTypeAttribute = "AsConsumer"
	default:
		gen.Errs.Fatal(tf.Pos(), fmt.Sprintf("unknown trace type %q", tf.Type))
	}
	if withTypeAttribute != "" {
		withTypeAttribute = fmt.Sprintf("\n\t\t%s.%s(),", importName, withTypeAttribute)
	}

	// Create if required the "tracing.WithAttributes" for the initial "StartSpan" call
	params := paramData(tf.AST)
	withAttributesOnCall := ""
	if len(params) > 0 {
		withAttributesOnCall = fmt.Sprintf("\n\t\t%s.WithAttributes(%s),", importName, strings.Join(params, ", "))
	}

	// Create if required the "span.WithAttributes" for the final "Finish" call
	// and capture the name of the last error return variable
	errorVariableName, returnVars := returnData(rw, tf.AST)
	withAttributesOnReturn := ""
	if len(returnVars) > 0 {
		withAttributesOnReturn = fmt.Sprintf(".\n\t\t\tWithAttributes(%s)", strings.Join(returnVars, ", "))
	}

	// Insert the tracing code
	rw.Insert(insertPos, []byte(fmt.Sprintf(
		"\n\t__auto_generated_span := %s.StartSpan(\n\t\t%q,%s%s\n\t);"+
			"\n\tdefer func() {\n\t\t__auto_generated_span%s.\n\t\t\tFinish(%s)\n\t}();/*line :%d:%d*/",
		importName, tf.Name, withTypeAttribute, withAttributesOnCall,
		withAttributesOnReturn, errorVariableName,
		ln.Line, ln.Column,
	)))
}

// paramData returns a list of all the parameters to the function as the idents
// with one quoted and one not quoted.
func paramData(fn *ast.FuncDecl) []string {
	var params []string
	for _, param := range fn.Type.Params.List {
		if ignoreVariableBasedOnType(param.Type) {
			continue
		}

		for _, name := range param.Names {
			params = append(params, strconv.Quote(name.Name), name.Name)
		}
	}
	return params

}

// returnData returns the name of the last error return variable and a list of all the other return variables.
// with a name to display as the variable name, followed by the actual variable name.
//
// If returned it will rewrite the function to have named return variables.
//
// i.e.
//
// func foo() (string, error) ==> func foo() (__encore_named_rtn_var_0 string, __encore_named_rtn_var_1 error)
// func foo() error => func foo() (__encore_named_rtn_var_0 error)
func returnData(rw *rewrite.Rewriter, fn *ast.FuncDecl) (string, []string) {
	if fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		// There are no return variables
		return "nil", nil
	}
	rtnVars := fn.Type.Results.List

	if fn.Type.Results.Opening == token.NoPos {
		// we need to add () around the return list
		rw.Insert(fn.Type.Results.Pos(), []byte("("))
		rw.Insert(fn.Type.Results.End(), []byte(")"))
	}

	lastErrorType := "nil"
	var attributesToCapture []string
	lastErrorIdx := -1
	for i, res := range rtnVars {
		ident := ""
		displayName := ""
		if len(res.Names) == 0 {
			// If the variable is unnamed, we need to name it so we can capture it
			ident = fmt.Sprintf("__encore_named_rtn_var_%d", i)
			displayName = fmt.Sprintf("return %d", i+1)

			rw.Insert(res.Type.Pos(), []byte(ident+" "))
			res.Names = []*ast.Ident{ast.NewIdent(ident)}
		} else {
			// Otherwise we can use the name of the variable
			ident = res.Names[0].Name
			displayName = ident
		}

		if ignoreVariableBasedOnType(res.Type) {
			continue
		}

		switch t := res.Type.(type) {
		case *ast.Ident:
			if t.Name == "error" {
				lastErrorType = ident
				lastErrorIdx = i
			}
		}

		attributesToCapture = append(attributesToCapture, strconv.Quote(displayName), ident)
	}

	// Now remove the last error return variable from the attributes to capture
	if lastErrorIdx != -1 {
		attributesToCapture = append(attributesToCapture[:lastErrorIdx*2], attributesToCapture[lastErrorIdx*2+2:]...)
	}

	// no error return variable
	return lastErrorType, attributesToCapture
}

func ignoreVariableBasedOnType(t ast.Expr) bool {
	// Skip over context.Context parameters
	switch param := t.(type) {
	case *ast.SelectorExpr:
		if ident, ok := param.X.(*ast.Ident); ok && ident.Name == "context" {
			// ignore everything from the context package
			return true
		}
	}

	return false
}

func getOrAddTracingImport(gen *codegen.Generator, rw *rewrite.Rewriter, file *pkginfo.File) string {
	tracingImport, found := file.Imports["encore.dev/tracing"]

	// If the import is not found, add it.
	if !found {
		name := "__encore_tracing_api"
		tracingImport = &ast.ImportSpec{Name: ast.NewIdent(name)}
		file.Imports["encore.dev/tracing"] = tracingImport
		insertTracingImport(gen, rw, file.AST(), name)
		return name
	}

	importSpec := tracingImport.(*ast.ImportSpec)
	if importSpec.Name == nil {
		return "tracing"
	}

	return importSpec.Name.Name
}

func insertTracingImport(gen *codegen.Generator, rw *rewrite.Rewriter, file *ast.File, import_name string) {
	insertPos := firstASTNode(file)
	ln := gen.FS.Position(insertPos)

	rw.Insert(
		insertPos,
		[]byte(fmt.Sprintf("\nimport %s %s;/*line :%d:%d*/",
			import_name,
			strconv.Quote("encore.dev/tracing"),
			ln.Line, ln.Column,
		)),
	)
}

func firstASTNode(file *ast.File) token.Pos {
	if len(file.Decls) > 0 {
		return file.Decls[0].Pos()
	}

	if len(file.Comments) > 0 {
		return file.Comments[0].Pos()
	}

	return token.NoPos
}

// collectTracedFunctions collects all traced functions in the app.
func collectTracedFunctions(appDesc *app.Desc) (tfs []*tracedfunc.TracedFunc) {
	for _, res := range appDesc.Parse.Resources() {
		if res.Kind() == resource.TracedFunc {
			tf := res.(*tracedfunc.TracedFunc)
			tfs = append(tfs, tf)
		}
	}

	return tfs
}
