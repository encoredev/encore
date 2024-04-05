package ai

import (
	"strings"

	meta "encr.dev/proto/encore/parser/meta/v1"
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

func parseServicesFromMetadata(md *meta.Data) []Service {
	var services []Service
	for _, metaSvc := range md.Svcs {
		svc := Service{
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
				ep.RequestType = md.Decls[rpc.RequestSchema.GetNamed().Id].Name
			}
			if rpc.ResponseSchema != nil {
				ep.ResponseType = md.Decls[rpc.ResponseSchema.GetNamed().Id].Name
			}
			svc.Endpoints = append(svc.Endpoints, ep)
		}
		services = append(services, svc)
	}
	return services
}
