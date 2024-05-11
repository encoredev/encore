package ai

import (
	"slices"
	"strings"

	"encr.dev/internal/clientgen"
	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
	"encr.dev/v2/internals/resourcepaths"
)

func toPathSegments(p *resourcepaths.Path, docs map[string]string) []PathSegment {
	rtn := make([]PathSegment, 0, len(p.Segments))
	for _, s := range p.Segments {
		switch s.Type {
		case resourcepaths.Literal:
			rtn = append(rtn, PathSegment{Type: SegmentTypeLiteral, Value: ptr(s.Value)})
		case resourcepaths.Param:
			rtn = append(rtn, PathSegment{
				Type:      SegmentTypeParam,
				Value:     ptr(s.Value),
				ValueType: ptr(SegmentValueType(strings.ToLower(s.ValueType.String()))),
				Doc:       docs[s.Value],
			})
		case resourcepaths.Wildcard:
			rtn = append(rtn, PathSegment{
				Type:      SegmentTypeWildcard,
				Value:     ptr(s.Value),
				ValueType: ptr(SegmentValueType(strings.ToLower(s.ValueType.String()))),
				Doc:       docs[s.Value],
			})
		case resourcepaths.Fallback:
			rtn = append(rtn, PathSegment{
				Type:      SegmentTypeFallback,
				Value:     ptr(s.Value),
				ValueType: ptr(SegmentValueType(strings.ToLower(s.ValueType.String()))),
				Doc:       docs[s.Value],
			})
		}
	}
	return rtn
}

func metaPathToPathSegments(metaPath *meta.Path) []PathSegment {
	var segments []PathSegment
	for _, seg := range metaPath.Segments {
		segments = append(segments, PathSegment{
			Type:      toSegmentType(seg.Type),
			Value:     ptr(seg.Value),
			ValueType: ptr(toSegmentValueType(seg.ValueType)),
		})
	}
	return segments
}

func toSegmentValueType(valueType meta.PathSegment_ParamType) SegmentValueType {
	switch valueType {
	case meta.PathSegment_UUID:
		return "string"
	default:
		return SegmentValueType(strings.ToLower(valueType.String()))
	}
}

func toSegmentType(segmentType meta.PathSegment_SegmentType) SegmentType {
	switch segmentType {
	case meta.PathSegment_LITERAL:
		return SegmentTypeLiteral
	case meta.PathSegment_PARAM:
		return SegmentTypeParam
	case meta.PathSegment_WILDCARD:
		return SegmentTypeWildcard
	case meta.PathSegment_FALLBACK:
		return SegmentTypeFallback
	default:
		panic("unknown segment type")
	}
}

func toVisibility(accessType meta.RPC_AccessType) VisibilityType {
	switch accessType {
	case meta.RPC_PUBLIC:
		return VisibilityTypePublic
	case meta.RPC_PRIVATE:
		return VisibilityTypePrivate
	case meta.RPC_AUTH:
		return ""
	default:
		panic("unknown access type")
	}
}

func renderTypesFromMetadata(md *meta.Data, svcs ...string) string {
	var types []*schema.Decl
	for _, metaSvc := range md.Svcs {
		if len(svcs) > 0 && !slices.Contains(svcs, metaSvc.Name) {
			continue
		}
		for _, rpc := range metaSvc.Rpcs {
			if rpc.RequestSchema != nil {
				types = append(types, md.Decls[rpc.RequestSchema.GetNamed().Id])
			}
			if rpc.ResponseSchema != nil {
				types = append(types, md.Decls[rpc.ResponseSchema.GetNamed().Id])
			}
		}
	}
	src, _ := clientgen.GenTypes(md, types...)
	return string(src)
}

func parseServicesFromMetadata(md *meta.Data, svcs ...string) []ServiceInput {
	services := []ServiceInput{}
	for _, metaSvc := range md.Svcs {
		if len(svcs) > 0 && !slices.Contains(svcs, metaSvc.Name) {
			continue
		}
		svc := ServiceInput{
			Name: metaSvc.Name,
		}
		for _, rpc := range metaSvc.Rpcs {
			ep := &Endpoint{
				Name:       rpc.Name,
				Method:     rpc.HttpMethods[0],
				Visibility: toVisibility(rpc.AccessType),
				Path:       metaPathToPathSegments(rpc.Path),
			}
			if rpc.RequestSchema != nil {
				decl := md.Decls[rpc.RequestSchema.GetNamed().Id]
				ep.RequestType = decl.Name
			}
			if rpc.ResponseSchema != nil {
				decl := md.Decls[rpc.ResponseSchema.GetNamed().Id]
				ep.ResponseType = decl.Name
			}
			svc.Endpoints = append(svc.Endpoints, ep)
		}
		services = append(services, svc)
	}
	return services
}
