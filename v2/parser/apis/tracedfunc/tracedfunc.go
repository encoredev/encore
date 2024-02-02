package tracedfunc

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/parser/apis/internal/directive"
	"encr.dev/v2/parser/resource"
)

type TracedFunc struct {
	AST  *ast.FuncDecl // the AST node that this declaration represents
	File *pkginfo.File // file it's declared in
	Name string        // the name of the span created when this function is traced
	Type TraceType     // the type of the span created when this function is traced
}

type TraceType uint8

const (
	Internal TraceType = iota
	RequestHandler
	Call
	Producer
	Consumer
)

func (tf *TracedFunc) Kind() resource.Kind { return resource.TracedFunc }
func (tf *TracedFunc) Pos() token.Pos      { return tf.AST.Pos() }
func (tf *TracedFunc) End() token.Pos      { return tf.AST.End() }
func (tf *TracedFunc) SortKey() string {
	return tf.File.Pkg.ImportPath.String() + "." + tf.AST.Name.Name
}

type ParseData struct {
	Errs   *perr.List
	Schema *schema.Parser

	File *pkginfo.File
	Func *ast.FuncDecl
	Dir  *directive.Directive
	Doc  string
}

// Parse parses the middleware in the provided declaration.
// It may return nil on errors.
func Parse(d ParseData) *TracedFunc {
	// can't trace a non Go function
	if d.Func.Body == nil {
		d.Errs.Add(errNonGoFunc.AtGoNode(d.Func))
		return nil
	}

	tf := &TracedFunc{
		AST:  d.Func,
		File: d.File,
		Name: fmt.Sprintf("%s.%s", d.File.Pkg.Name, d.Func.Name.Name),
		Type: Internal,
	}

	_ = directive.Validate(d.Errs, d.Dir, directive.ValidateSpec{
		AllowedOptions: []string{},
		AllowedFields:  []string{"name", "type"},
		ValidateOption: nil,
		ValidateField: func(errs *perr.List, f directive.Field) (ok bool) {
			switch f.Key {
			case "name":
				name := strings.TrimSpace(f.Value)
				if name != "" {
					tf.Name = name
				}
			case "type":
				name := strings.ToLower(strings.TrimSpace(f.Value))
				switch name {
				case "internal":
					tf.Type = Internal
				case "request_handler", "api_handler", "handler":
					// alaises
					tf.Type = RequestHandler
				case "call", "api_call":
					tf.Type = Call
				case "producer", "publisher":
					tf.Type = Producer
				case "consumer", "subscriber":
					tf.Type = Consumer
				default:
					errs.Add(errInvalidTracedFunc.AtGoNode(f))
				}
			}
			return true
		},
		ValidateTag: nil,
	})

	return tf
}
