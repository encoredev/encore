package schemautil

import (
	"go/ast"

	"encr.dev/pkg/option"
)

// GetArgument gets the n'th argument from the field list.
// It reports the name of the n'th argument if it has one.
func GetArgument(fields *ast.FieldList, n int) (f *ast.Field, name option.Option[string]) {
	idx := 0
	for _, f := range fields.List {
		num := len(f.Names)
		if num == 0 {
			num = 1
		}
		for i := 0; i < num; i++ {
			if idx == n {
				var name option.Option[string]
				if hasName := i < len(f.Names); hasName {
					name = option.Some(f.Names[i].Name)
				}
				return f, name
			}
			idx++
		}
	}
	return nil, option.None[string]()
}
