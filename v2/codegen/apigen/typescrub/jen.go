package typescrub

import (
	"fmt"

	"encore.dev/appruntime/exported/scrub"

	. "github.com/dave/jennifer/jen"
)

func HeadersToJen(headers []string) Code {
	if len(headers) == 0 {
		return Nil()
	}
	return Map(String()).Bool().ValuesFunc(func(g *Group) {
		for _, h := range headers {
			g.Add(Lit(h).Op(":").Lit(true))
		}
	})
}

func PathsToJen(paths []scrub.Path) Code {
	if len(paths) == 0 {
		return Nil()
	}
	return Index().Qual("encore.dev/appruntime/exported/scrub", "Path").ValuesFunc(func(g *Group) {
		for _, p := range paths {
			g.Add(scrubPathToJen(p))
		}
	})
}

func scrubPathToJen(path scrub.Path) Code {
	return Index().Qual("encore.dev/appruntime/exported/scrub", "PathEntry").ValuesFunc(func(g *Group) {
		for _, pe := range path {
			kind := ""
			switch pe.Kind {
			case scrub.ObjectField:
				kind = "ObjectField"
			case scrub.MapKey:
				kind = "MapKey"
			case scrub.MapValue:
				kind = "MapValue"
			default:
				panic(fmt.Sprintf("unknown PathEntry.Kind: %v", pe.Kind))
			}
			g.Add(
				Values(
					Id("Kind").Op(":").Qual("encore.dev/appruntime/exported/scrub", kind),
					Id("FieldName").Op(":").Lit(pe.FieldName),
					Id("CaseSensitive").Op(":").Lit(pe.CaseSensitive),
				),
			)
		}
	})
}
