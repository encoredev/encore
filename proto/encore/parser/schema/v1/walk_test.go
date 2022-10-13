package v1

import "testing"

// TestWalk_RecursiveDataStructure tests that Walk gracefully handles
// recursive and mutually recursive data structures.
func TestWalk_RecursiveDataStructure(t *testing.T) {
	selfRecursive := &Decl{
		Id: 0,
		Type: &Type{
			Typ: &Type_Named{
				Named: &Named{
					Id: 0,
				},
			},
		},
	}

	mutualRecursiveOne := &Decl{
		Id: 1,
		Type: &Type{
			Typ: &Type_Struct{
				Struct: &Struct{
					Fields: []*Field{
						{
							Typ: &Type{Typ: &Type_Named{
								Named: &Named{Id: 1},
							}},
						},
					},
				},
			},
		},
	}
	mutualRecursiveTwo := &Decl{
		Id: 1,
		Type: &Type{
			Typ: &Type_Struct{
				Struct: &Struct{
					Fields: []*Field{
						{
							Typ: &Type{Typ: &Type_Named{
								Named: &Named{Id: 0},
							}},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name  string
		decls []*Decl
		node  any
	}{
		{
			name:  "self_recursive",
			decls: []*Decl{selfRecursive},
			node:  selfRecursive.Type,
		},
		{
			name:  "mutual_recursive",
			decls: []*Decl{mutualRecursiveOne, mutualRecursiveTwo},
			node:  mutualRecursiveOne.Type,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visitor := func(node any) error { return nil }
			if err := Walk(tt.decls, tt.node, visitor); err != nil {
				t.Fatal(err)
			}
		})
	}
}
