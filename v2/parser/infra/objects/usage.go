package objects

import (
	"encr.dev/pkg/option"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/parser/resource/usage"
)

type MethodUsage struct {
	usage.Base
	Perm Perm
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

func ResolveBucketUsage(data usage.ResolveData, topic *Bucket) usage.Usage {
	switch expr := data.Expr.(type) {
	case *usage.MethodCall:
		var perm Perm
		switch expr.Method {
		case "Upload":
			perm = WriteObject
		default:
			return nil
		}
		return &MethodUsage{
			Base: usage.Base{
				File: expr.File,
				Bind: expr.Bind,
				Expr: expr,
			},
			Perm: perm,
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
		if schemautil.IsNamed(typ, "encore.dev/storage/objects", "Uploader") {
			return &RefUsage{
				Base: usage.Base{
					File: expr.File,
					Bind: expr.Bind,
					Expr: expr,
				},
				Perms: []Perm{WriteObject},
			}, true
		}
		return nil, false
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
