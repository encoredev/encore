package daemon

import (
	"fmt"
	"time"

	jsoniter "github.com/json-iterator/go"

	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

// genSchema generates a JSON payload to match the schema.
func genSchema(meta *meta.Data, decl *schema.Type) []byte {
	if decl == nil {
		return nil
	}
	r := &schemaRenderer{
		Stream:    jsoniter.NewStream(jsoniter.ConfigDefault, nil, 256),
		meta:      meta,
		seenDecls: make(map[uint32]*schema.Decl),
	}
	return r.Render(decl)
}

type schemaRenderer struct {
	*jsoniter.Stream
	meta      *meta.Data
	seenDecls map[uint32]*schema.Decl
}

func (r *schemaRenderer) Render(d *schema.Type) []byte {
	r.renderType(d)
	return r.Buffer()
}

func (r *schemaRenderer) renderType(typ *schema.Type) {
	switch typ := typ.Typ.(type) {
	case *schema.Type_Struct:
		r.renderStruct(typ.Struct)
	case *schema.Type_Map:
		r.renderMap(typ.Map)
	case *schema.Type_List:
		r.renderList(typ.List)
	case *schema.Type_Builtin:
		r.renderBuiltin(typ.Builtin)
	case *schema.Type_Named:
		r.renderNamed(typ.Named)
	case *schema.Type_Pointer:
		r.renderType(typ.Pointer.Base)
	case *schema.Type_Config:
		// Config is invisible here
		r.renderType(typ.Config.Elem)
	default:
		panic(fmt.Sprintf("unknown schema type %T", typ))
	}
}

func (r *schemaRenderer) renderStruct(s *schema.Struct) {
	r.WriteObjectStart()
	written := false
	for _, f := range s.Fields {
		n := f.JsonName
		if n == "-" {
			continue
		} else if n == "" {
			n = f.Name
		}

		if written {
			r.WriteMore()
		}
		r.WriteObjectField(n)
		r.renderType(f.Typ)
		written = true
	}
	r.WriteObjectEnd()
}

func (r *schemaRenderer) renderMap(m *schema.Map) {
	r.WriteObjectStart()
	r.renderType(m.Key)
	r.WriteRaw(": ")
	r.renderType(m.Value)
	r.WriteObjectEnd()
}

func (r *schemaRenderer) renderList(l *schema.List) {
	r.WriteArrayStart()
	r.renderType(l.Elem)
	r.WriteArrayEnd()
}

func (r *schemaRenderer) renderBuiltin(b schema.Builtin) {
	switch b {
	case schema.Builtin_ANY:
		r.WriteString("<any data>")
	case schema.Builtin_BOOL:
		r.WriteBool(true)
	case schema.Builtin_INT, schema.Builtin_INT8, schema.Builtin_INT16, schema.Builtin_INT32, schema.Builtin_INT64,
		schema.Builtin_UINT, schema.Builtin_UINT8, schema.Builtin_UINT16, schema.Builtin_UINT32, schema.Builtin_UINT64:
		r.WriteInt(1)
	case schema.Builtin_FLOAT32, schema.Builtin_FLOAT64:
		r.WriteRaw("2.3")
	case schema.Builtin_STRING:
		r.WriteString("hello")
	case schema.Builtin_BYTES:
		r.WriteString("YmFzZTY0Cg==") // "base64"
	case schema.Builtin_TIME:
		s, _ := time.Now().MarshalText()
		r.WriteString(string(s))
	case schema.Builtin_UUID:
		r.WriteString("7d42f515-3517-4e76-be13-30880443546f")
	case schema.Builtin_JSON:
		r.WriteObjectStart()
		r.WriteObjectField("some json data")
		r.WriteBool(true)
		r.WriteObjectEnd()
	case schema.Builtin_USER_ID:
		r.WriteString("userID")
	default:
		r.WriteString("<unknown>")
	}
}

func (r *schemaRenderer) renderNamed(n *schema.Named) {
	if _, ok := r.seenDecls[n.Id]; ok {
		// Already seen this name before
		r.WriteNil()
		return
	}

	// Avoid infinite recursion
	decl := r.meta.Decls[n.Id]
	r.seenDecls[n.Id] = decl
	r.renderType(decl.Type)
	delete(r.seenDecls, n.Id)
}
