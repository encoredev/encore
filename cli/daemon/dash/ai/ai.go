package ai

import (
	"fmt"
	"strings"
)

type VisibilityType string

const (
	VisibilityTypePublic  VisibilityType = "public"
	VisibilityTypePrivate VisibilityType = "private"
	VisibilityTypeAuth    VisibilityType = "auth"
)

type SegmentType string

const (
	SegmentTypeLiteral  SegmentType = "literal"
	SegmentTypeParam    SegmentType = "param"
	SegmentTypeWildcard SegmentType = "wildcard"
	SegmentTypeFallback SegmentType = "fallback"
)

type SegmentValueType string

const (
	SegmentValueTypeString SegmentValueType = "str"
	SegmentValueTypeInt    SegmentValueType = "int"
	SegmentValueTypeBool   SegmentValueType = "bool"
)

type BaseAIUpdateType struct {
	Type string `graphql:"__typename" json:"type"`
}

func (b BaseAIUpdateType) IsAIUpdateType() {}

type AIUpdateType interface {
	IsAIUpdateType()
}

type AIStreamUpdate = Result[AIUpdateType]

func ptr[T any](val T) *T {
	return &val
}

type Result[T any] struct {
	Value    T
	Finished *bool
	Error    *string
}

type PathSegment struct {
	Type      SegmentType       `json:"type,omitempty"`
	Value     *string           `json:"value,omitempty"`
	ValueType *SegmentValueType `json:"valueType,omitempty"`
}

func (p PathSegment) String() string {
	switch p.Type {
	case SegmentTypeLiteral:
		return *p.Value
	case SegmentTypeParam:
		return fmt.Sprintf("{%s:%s}", *p.Value, *p.ValueType)
	case SegmentTypeWildcard:
		return "*"
	case SegmentTypeFallback:
		return "**"
	default:
		panic(fmt.Sprintf("unknown path segment type: %s", p.Type))
	}
}

type EndpointInput struct {
	Name         string         `json:"name,omitempty"`
	Doc          string         `json:"doc,omitempty"`
	Method       string         `json:"method,omitempty"`
	Visibility   VisibilityType `json:"visibility,omitempty"`
	Path         []PathSegment  `json:"path,omitempty"`
	RequestType  string         `json:"requestType,omitempty"`
	ResponseType string         `json:"responseType,omitempty"`
	Errors       []ErrorInput   `json:"errors,omitempty"`
	Structs      string         `json:"structs,omitempty"`
}

type EndpointUpdate struct {
	BaseAIUpdateType
	Service      string         `json:"service,omitempty"`
	Name         string         `json:"name,omitempty"`
	Doc          string         `json:"doc,omitempty"`
	Method       string         `json:"method,omitempty"`
	Visibility   VisibilityType `json:"visibility,omitempty"`
	Path         []PathSegment  `json:"path,omitempty"`
	RequestType  string         `json:"requestType,omitempty"`
	ResponseType string         `json:"responseType,omitempty"`
	Errors       []string       `json:"errors,omitempty"`
}

type ServiceUpdate struct {
	BaseAIUpdateType
	Name string `json:"name,omitempty"`
	Doc  string `json:"doc,omitempty"`
}

type ServiceInput struct {
	Name      string          `json:"name,omitempty"`
	Doc       string          `json:"doc,omitempty"`
	Endpoints []EndpointInput `json:"endpoints,omitempty"`
}

type StructUpdate struct {
	BaseAIUpdateType
	Service  string `json:"service,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Name     string `json:"name,omitempty"`
	Doc      string `graphql:"mdoc: doc" json:"doc,omitempty"`
}

type StructInput struct {
	Name   string              `json:"name,omitempty"`
	Doc    string              `json:"doc,omitempty"`
	Fields []*StructFieldInput `json:"fields,omitempty"`
}

func (s *StructInput) Render() string {
	rtn := strings.Builder{}
	if s.Doc != "" {
		rtn.WriteString(fmt.Sprintf("// %s\n", s.Doc))
	}
	rtn.WriteString(fmt.Sprintf("type %s struct {\n", s.Name))
	for _, f := range s.Fields {
		rtn.WriteString("\n")
		if f.Doc != "" {
			rtn.WriteString(fmt.Sprintf("  // %s", f.Doc))
		}
		rtn.WriteString("\n")
		rtn.WriteString(fmt.Sprintf("  %s %s\n", f.Name, f.Type))
	}
	rtn.WriteString("\n}\n")
	return rtn.String()
}

type EndpointStructs struct {
	Type     string `json:"type,omitempty"`
	Service  string `json:"service,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Code     string `json:"code,omitempty"`
}

type StructFieldInput struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
	Doc  string `json:"doc,omitempty"`
}

type StructFieldUpdate struct {
	BaseAIUpdateType
	Service  string `json:"service,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Struct   string `json:"struct,omitempty"`
	Name     string `json:"name,omitempty"`
	Type     string `json:"type,omitempty"`
	Doc      string `graphql:"mdoc: doc" json:"doc,omitempty"`
}

type ErrorInput struct {
	Code string `json:"code,omitempty"`
	Doc  string `json:"doc,omitempty"`
}

func (e ErrorInput) String() string {
	return e.Code
}

type ErrorUpdate struct {
	BaseAIUpdateType
	Code     string `json:"code,omitempty"`
	Doc      string `json:"doc,omitempty"`
	Service  string `json:"service,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
}
