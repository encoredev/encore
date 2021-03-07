package codegen

import (
	"fmt"
	"sort"

	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

func getNamedTypes(md *meta.Data) *typeRegistry {
	r := &typeRegistry{
		md:         md,
		namespaces: make(map[string][]*schema.Decl),
		seenDecls:  make(map[uint32]bool),
		declRefs:   make(map[uint32]map[uint32]bool),
	}
	for _, svc := range md.Svcs {
		for _, rpc := range svc.Rpcs {
			if rpc.AccessType != meta.RPC_PRIVATE {
				r.VisitDecl(rpc.RequestSchema)
				r.VisitDecl(rpc.ResponseSchema)
			}
		}
	}
	return r
}

// typeRegistry computes the visible set of type declarations
// and how to group them into namespaces.
type typeRegistry struct {
	md         *meta.Data
	namespaces map[string][]*schema.Decl
	seenDecls  map[uint32]bool
	declRefs   map[uint32]map[uint32]bool // tracks which decls reference which other decls
	currDecl   *schema.Decl               // may be nil
}

type namedType struct {
	pkg  string
	name string
}

func (v *typeRegistry) Decls(name string) []*schema.Decl {
	return v.namespaces[name]
}

func (v *typeRegistry) Namespaces() []string {
	nss := make([]string, 0, len(v.namespaces))
	for ns := range v.namespaces {
		nss = append(nss, ns)
	}
	sort.Strings(nss)
	return nss
}

func (v *typeRegistry) Visit(typ *schema.Type) {
	if typ == nil {
		return
	}
	switch t := typ.Typ.(type) {
	case *schema.Type_Named:
		v.visitNamed(t.Named)
	case *schema.Type_List:
		v.Visit(t.List.Elem)
	case *schema.Type_Map:
		v.Visit(t.Map.Key)
		v.Visit(t.Map.Value)
	case *schema.Type_Struct:
		for _, f := range t.Struct.Fields {
			v.Visit(f.Typ)
		}
	case *schema.Type_Builtin:
		// do nothing
	default:
		panic(fmt.Sprintf("unhandled type: %T", typ))
	}
}

func (v *typeRegistry) VisitDecl(decl *schema.Decl) {
	if decl == nil {
		return
	}

	if !v.seenDecls[decl.Id] {
		v.seenDecls[decl.Id] = true
		ns := decl.Loc.PkgName
		v.namespaces[ns] = append(v.namespaces[ns], decl)

		// Set currDecl when processing this and then reset it
		prev := v.currDecl
		v.currDecl = decl
		v.Visit(decl.Type)
		v.currDecl = prev
	}
}

func (v *typeRegistry) visitNamed(n *schema.Named) {
	to := n.Id
	curr := v.currDecl
	if curr != nil {
		from := curr.Id
		if _, ok := v.declRefs[from]; !ok {
			v.declRefs[from] = make(map[uint32]bool)
		}
		v.declRefs[from][to] = true
	}

	decl := v.md.Decls[to]
	v.VisitDecl(decl)

	// Add transitive refs
	if curr != nil {
		from := curr.Id
		for to2 := range v.declRefs[to] {
			v.declRefs[from][to2] = true
		}
	}
}

func (v *typeRegistry) IsRecursiveRef(from, to uint32) bool {
	return v.declRefs[from][to] && v.declRefs[to][from]
}
