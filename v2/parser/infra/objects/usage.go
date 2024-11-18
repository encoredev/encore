package objects

import (
	"encr.dev/pkg/option"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/parser/resource/usage"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

type MethodUsage struct {
	usage.Base
	Method string
	Perm   Perm
}

type RefUsage struct {
	usage.Base
	Perms []Perm
}

func (u *RefUsage) HasPerm(perm Perm) bool {
	for _, p := range u.Perms {
		if p == perm {
			return true
		}
	}
	return false
}

type Perm string

const (
	ListObjects          Perm = "list-objects"
	ReadObjectContents   Perm = "read-object-contents"
	WriteObject          Perm = "write-object"
	UpdateObjectMetadata Perm = "update-object-metadata"
	GetObjectMetadata    Perm = "get-object-metadata"
	DeleteObject         Perm = "delete-object"
)

func (p Perm) ToMeta() (meta.BucketUsage_Operation, bool) {
	switch p {
	case ListObjects:
		return meta.BucketUsage_LIST_OBJECTS, true
	case ReadObjectContents:
		return meta.BucketUsage_READ_OBJECT_CONTENTS, true
	case WriteObject:
		return meta.BucketUsage_WRITE_OBJECT, true
	case UpdateObjectMetadata:
		return meta.BucketUsage_UPDATE_OBJECT_METADATA, true
	case GetObjectMetadata:
		return meta.BucketUsage_GET_OBJECT_METADATA, true
	case DeleteObject:
		return meta.BucketUsage_DELETE_OBJECT, true
	default:
		return meta.BucketUsage_UNKNOWN, false
	}
}

func ResolveBucketUsage(data usage.ResolveData, topic *Bucket) usage.Usage {
	switch expr := data.Expr.(type) {
	case *usage.MethodCall:
		var perm Perm
		switch expr.Method {
		case "Upload":
			perm = WriteObject
		case "Download":
			perm = ReadObjectContents
		case "List":
			perm = ListObjects
		case "Remove":
			perm = DeleteObject
		default:
			return nil
		}

		return &MethodUsage{
			Base: usage.Base{
				File: expr.File,
				Bind: expr.Bind,
				Expr: expr,
			},
			Method: expr.Method,
			Perm:   perm,
		}

	case *usage.FuncArg:
		switch {
		case option.Contains(expr.PkgFunc, pkginfo.Q("encore.dev/storage/objects", "BucketRef")):
			return parseBucketRef(data.Errs, expr)
		}
	}

	data.Errs.Add(errInvalidBucketUsage.AtGoNode(data.Expr))
	return nil
}

func parseBucketRef(errs *perr.List, expr *usage.FuncArg) usage.Usage {
	if len(expr.TypeArgs) < 1 {
		errs.Add(errBucketRefNoTypeArgs.AtGoNode(expr.Call))
		return nil
	}

	checkUsage := func(typ schema.Type) (usage.Usage, bool) {
		var perms []Perm
		switch {
		case isNamed(typ, "Uploader"):
			perms = []Perm{WriteObject}
		case isNamed(typ, "Downloader"):
			perms = []Perm{ReadObjectContents}
		case isNamed(typ, "Lister"):
			perms = []Perm{ListObjects}
		case isNamed(typ, "Remover"):
			perms = []Perm{DeleteObject}
		case isNamed(typ, "Attrser"):
			perms = []Perm{GetObjectMetadata}
		case isNamed(typ, "ReadWriter"):
			perms = []Perm{WriteObject, ReadObjectContents, ListObjects, DeleteObject, GetObjectMetadata, UpdateObjectMetadata}
		default:
			return nil, false
		}

		return &RefUsage{
			Base: usage.Base{
				File: expr.File,
				Bind: expr.Bind,
				Expr: expr,
			},
			Perms: perms,
		}, true
	}

	// Do we have a simple usage directly as the type argument?
	if u, ok := checkUsage(expr.TypeArgs[0]); ok {
		return u
	}

	// Determine if we have a custom ref type,
	// either in the form "type Foo = pubsub.Publisher[Msg]"
	// or in the form "type Foo interface { pubsub.Publisher[Msg] }"
	if named, ok := expr.TypeArgs[0].(schema.NamedType); ok {
		underlying := named.Decl().Type
		if u, ok := checkUsage(underlying); ok {
			return u
		}

		// Otherwise make sure the interface only embeds the one supported type we have (pubsub.Publisher).
		// We'll need to extend this in the future to support multiple permissions.
		if iface, ok := underlying.(schema.InterfaceType); ok {
			if len(iface.EmbeddedIfaces) == 1 && len(iface.Methods) == 0 && len(iface.TypeLists) == 0 {
				if u, ok := checkUsage(iface.EmbeddedIfaces[0]); ok {
					return u
				}
			}
		}
	}

	errs.Add(errBucketRefInvalidPerms.AtGoNode(expr.Call))
	return nil
}

func isNamed(typ schema.Type, name string) bool {
	return schemautil.IsNamed(typ, "encore.dev/storage/objects", name)
}
