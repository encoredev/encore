package ai

import (
	"fmt"

	meta "encr.dev/proto/encore/parser/meta/v1"
	"encr.dev/v2/internals/resourcepaths"
)

func toPathSegments(p *resourcepaths.Path) []PathSegment {
	rtn := make([]PathSegment, 0, len(p.Segments))
	for _, s := range p.Segments {
		switch s.Type {
		case resourcepaths.Literal:
			rtn = append(rtn, PathSegment{Type: SegmentTypeLiteral, Value: ptr(s.Value)})
		case resourcepaths.Param:
			rtn = append(rtn, PathSegment{Type: SegmentTypeParam, Value: ptr(s.Value), ValueType: ptr(SegmentValueType(s.ValueType.String()))})
		case resourcepaths.Wildcard:
			rtn = append(rtn, PathSegment{Type: SegmentTypeWildcard})
		case resourcepaths.Fallback:
			rtn = append(rtn, PathSegment{Type: SegmentTypeFallback})
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
	case meta.PathSegment_STRING, meta.PathSegment_UUID:
		return SegmentValueTypeString
	case meta.PathSegment_BOOL:
		return SegmentValueTypeBool
	case meta.PathSegment_INT, meta.PathSegment_INT8, meta.PathSegment_INT16, meta.PathSegment_INT32, meta.PathSegment_INT64,
		meta.PathSegment_UINT, meta.PathSegment_UINT8, meta.PathSegment_UINT16, meta.PathSegment_UINT32, meta.PathSegment_UINT64:
		return SegmentValueTypeInt
	default:
		panic("unknown segment value type")
	}
}

func valueTypeToGoType(t *SegmentValueType) string {
	switch *t {
	case SegmentValueTypeString:
		return "string"
	case SegmentValueTypeInt:
		return "int"
	case SegmentValueTypeBool:
		return "bool"
	default:
		panic(fmt.Sprintf("unknown segment value type: %s", *t))
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

func accessTypeToVisibility(accessType meta.RPC_AccessType) VisibilityType {
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
