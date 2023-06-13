package schemautil

import (
	"go/ast"
	"reflect"
	"testing"

	"encr.dev/pkg/option"
)

func TestGetArgument(t *testing.T) {
	noName := &ast.Field{
		Names: nil,
		Type:  ast.NewIdent("noName"),
	}
	oneName := &ast.Field{
		Names: []*ast.Ident{ast.NewIdent("a")},
		Type:  ast.NewIdent("oneName"),
	}
	twoNames := &ast.Field{
		Names: []*ast.Ident{ast.NewIdent("b"), ast.NewIdent("c")},
		Type:  ast.NewIdent("twoNames"),
	}
	tests := []struct {
		name     string
		fields   []*ast.Field
		n        int
		wantF    *ast.Field
		wantName option.Option[string]
	}{
		{
			name:     "empty",
			fields:   []*ast.Field{},
			n:        0,
			wantF:    nil,
			wantName: option.None[string](),
		},
		{
			name:     "noName",
			fields:   []*ast.Field{noName},
			n:        0,
			wantF:    noName,
			wantName: option.None[string](),
		},
		{
			name:     "noName_overflow",
			fields:   []*ast.Field{noName},
			n:        1,
			wantF:    nil,
			wantName: option.None[string](),
		},
		{
			name:     "multi_noName",
			fields:   []*ast.Field{noName, noName},
			n:        1,
			wantF:    noName,
			wantName: option.None[string](),
		},
		{
			name:     "first",
			fields:   []*ast.Field{oneName, twoNames},
			n:        0,
			wantF:    oneName,
			wantName: option.Some("a"),
		},
		{
			name:     "second",
			fields:   []*ast.Field{oneName, twoNames},
			n:        1,
			wantF:    twoNames,
			wantName: option.Some("b"),
		},
		{
			name:     "third",
			fields:   []*ast.Field{oneName, twoNames},
			n:        2,
			wantF:    twoNames,
			wantName: option.Some("c"),
		},
		{
			name:     "fourth",
			fields:   []*ast.Field{oneName, twoNames, noName},
			n:        3,
			wantF:    noName,
			wantName: option.None[string](),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotF, gotName := GetArgument(&ast.FieldList{List: tt.fields}, tt.n)
			if !reflect.DeepEqual(gotF, tt.wantF) {
				t.Errorf("GetArgument() gotF = %v, want %v", gotF, tt.wantF)
			}
			if !reflect.DeepEqual(gotName, tt.wantName) {
				t.Errorf("GetArgument() gotName = %v, want %v", gotName, tt.wantName)
			}
		})
	}
}
