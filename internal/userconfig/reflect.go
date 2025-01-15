package userconfig

import (
	_ "embed"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"reflect"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/fatih/structtag"
)

func keyForField(f *reflect.StructField) (string, error) {
	tags, err := structtag.Parse(string(f.Tag))
	if err != nil {
		return "", err
	}
	tag, err := tags.Get("koanf")
	if err != nil {
		return "", err
	}
	key := tag.Name
	if key == "" {
		return "", errors.New("empty key")
	}
	return key, nil
}

type keyDesc struct {
	Doc       string
	Type      Type
	FieldName string // field name in the Config struct
}

func newKeyDesc(f *reflect.StructField) (key string, desc keyDesc, err error) {
	tags, err := structtag.Parse(string(f.Tag))
	if err != nil {
		return "", keyDesc{}, err
	}
	tag, err := tags.Get("koanf")
	if err != nil {
		return "", keyDesc{}, errors.Wrap(err, "failed to get koanf tag")
	}
	key = tag.Name
	if key == "" {
		return "", keyDesc{}, errors.New("empty key")
	}

	kind, ok := kindFromReflect(f.Type.Kind())
	if !ok {
		return "", keyDesc{}, errors.Errorf("unsupported type %v", f.Type)
	}

	ty := Type{Kind: kind}

	// Do we have a default?
	if def, _ := tags.Get("default"); def != nil {
		val, err := kind.parseValue(def.Name)
		if err != nil {
			return "", keyDesc{}, errors.Wrap(err, "parse default value")
		}
		ty.Default = &val
	}

	// Do we have a oneof?
	if tag := f.Tag.Get("oneof"); tag != "" {
		var oneof []any
		for _, part := range strings.Split(tag, ",") {
			val, err := kind.parseValue(part)
			if err != nil {
				return "", keyDesc{}, errors.Wrap(err, "parse oneof value")
			}
			oneof = append(oneof, val)
		}
		ty.Oneof = oneof
	}

	desc = keyDesc{
		Doc:       docComments[f.Name],
		Type:      ty,
		FieldName: f.Name,
	}
	return key, desc, nil
}

var descs = (func() map[string]keyDesc {
	var cfg Config
	t := reflect.TypeOf(cfg)
	descs := make(map[string]keyDesc, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		key, desc, err := newKeyDesc(&f)
		if err != nil {
			panic(fmt.Sprintf("invalid userconfig definition for field %s: %v", f.Name, err))
		}
		if _, ok := descs[key]; ok {
			panic(fmt.Sprintf("duplicate key %s in userconfig.Config", key))
		}
		descs[key] = desc
	}

	return descs
})()

func kindFromReflect(kind reflect.Kind) (Kind, bool) {
	switch kind {
	case reflect.String:
		return String, true
	case reflect.Bool:
		return Bool, true
	case reflect.Int:
		return Int, true
	case reflect.Uint:
		return Uint, true
	default:
		return 0, false
	}
}

//go:embed config.go
var configGo string

// doc comments, keyed by field name.
var docComments = (func() map[string]string {
	// Parse config.go as a Go file to extract the doc comments.
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "config.go", configGo, parser.ParseComments)
	if err != nil {
		panic(fmt.Sprintf("userconfig/config.go is invalid: %v", err))
	}

	// Compute package documentation with examples.
	p, err := doc.NewFromFiles(fset, []*ast.File{f}, "encr.dev/internal/userconfig")
	if err != nil {
		panic(fmt.Sprintf("userconfig/config.go is invalid: %v", err))
	}

	for _, typ := range p.Types {
		if typ.Name == "Config" {
			comments := make(map[string]string)

			// Extract comments for each field.
			structType := typ.Decl.Specs[0].(*ast.TypeSpec).Type.(*ast.StructType)
			for _, f := range structType.Fields.List {
				if f.Doc == nil {
					continue
				}
				if len(f.Names) == 0 {
					panic("field has no name")
				}
				text := f.Doc.Text()
				for _, name := range f.Names {
					comments[name.Name] = text
				}
			}
			return comments
		}
	}

	panic("Config type not found in userconfig/config.go")
})()
