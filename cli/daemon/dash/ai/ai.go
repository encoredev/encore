package ai

import (
	"fmt"
	"strings"

	"encr.dev/v2/parser/apis/api/apienc"
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
	Name           string         `json:"name,omitempty"`
	Doc            string         `json:"doc,omitempty"`
	Method         string         `json:"method,omitempty"`
	Visibility     VisibilityType `json:"visibility,omitempty"`
	Path           []PathSegment  `json:"path,omitempty"`
	RequestType    string         `json:"requestType,omitempty"`
	ResponseType   string         `json:"responseType,omitempty"`
	Errors         []ErrorInput   `json:"errors,omitempty"`
	Types          []TypeInput    `json:"types,omitempty"`
	Language       string         `json:"language,omitempty"`
	TypeSource     string         `json:"typeSource,omitempty"`
	EndpointSource string         `json:"endpointSource,omitempty"`
}

func (s *EndpointInput) GraphQL() *EndpointInput {
	s.EndpointSource = ""
	s.Types = nil
	s.Language = ""
	return s
}

func (e *EndpointInput) Render() string {
	buf := strings.Builder{}
	if e.Doc != "" {
		for _, line := range strings.Split(e.Doc, "\n") {
			buf.WriteString(fmt.Sprintf("// %s\n", line))
		}
	}
	for i, err := range e.Errors {
		if i == 0 {
			buf.WriteString("// Errors:\n")
		}
		buf.WriteString(fmt.Sprintf("//  %s: %s\n", err.Code, err.Doc))
	}
	params := []string{"ctx context.Context"}
	path, pathParams := formatPath(e.Path)
	params = append(params, pathParams...)
	if e.RequestType != "" {
		params = append(params, "req *"+e.RequestType)
	}
	var rtnParams []string
	if e.ResponseType != "" {
		rtnParams = append(rtnParams, "*"+e.ResponseType)
	}
	rtnParams = append(rtnParams, "error")
	buf.WriteString(fmt.Sprintf("// encore:api %s method=%s path=%s\n", e.Visibility, e.Method, path))
	paramsStr := strings.Join(params, ", ")
	rtnParamsStr := strings.Join(rtnParams, ", ")
	if len(rtnParams) > 1 {
		rtnParamsStr = fmt.Sprintf("(%s)", rtnParamsStr)
	}
	buf.WriteString(fmt.Sprintf("func %s(%s) %s", e.Name, paramsStr, rtnParamsStr))
	return buf.String()
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
	Name      string           `json:"name,omitempty"`
	Doc       string           `json:"doc,omitempty"`
	Endpoints []*EndpointInput `json:"endpoints,omitempty"`
}

func (s ServiceInput) GraphQL() ServiceInput {
	for _, e := range s.Endpoints {
		e.GraphQL()
	}
	return s
}

type TypeUpdate struct {
	BaseAIUpdateType
	Service  string `json:"service,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Name     string `json:"name,omitempty"`
	Doc      string `graphql:"mdoc: doc" json:"doc,omitempty"`
}

type TypeInput struct {
	Name   string            `json:"name,omitempty"`
	Doc    string            `json:"doc,omitempty"`
	Fields []*TypeFieldInput `json:"fields,omitempty"`
}

func (s *TypeInput) Render() string {
	rtn := strings.Builder{}
	if s.Doc != "" {
		rtn.WriteString(fmt.Sprintf("// %s\n", strings.TrimSpace(s.Doc)))
	}
	rtn.WriteString(fmt.Sprintf("type %s struct {\n", s.Name))
	for i, f := range s.Fields {
		if i > 0 {
			rtn.WriteString("\n")
		}
		if f.Doc != "" {
			rtn.WriteString(fmt.Sprintf("  // %s", f.Doc))
		}
		tags := ""
		switch f.Location {
		case apienc.Body:
			tags = fmt.Sprintf(" `json:\"%s\"`", f.WireName)
		case apienc.Query:
			tags = fmt.Sprintf(" `query:\"%s\"`", f.WireName)
		case apienc.Header:
			tags = fmt.Sprintf(" `header:\"%s\"`", f.WireName)
		}
		rtn.WriteString(fmt.Sprintf("  %s %s%s\n", f.Name, f.Type, tags))
	}
	rtn.WriteString("}\n")
	return rtn.String()
}

type EndpointStructs struct {
	Type     string       `json:"type,omitempty"`
	Service  string       `json:"service,omitempty"`
	Endpoint string       `json:"endpoint,omitempty"`
	Code     string       `json:"code,omitempty"`
	Types    []*TypeInput `json:"types,omitempty"`
}

type TypeFieldInput struct {
	Name     string         `json:"name,omitempty"`
	WireName string         `json:"wireName,omitempty"`
	Type     string         `json:"type,omitempty"`
	Location apienc.WireLoc `json:"location,omitempty"`
	Doc      string         `json:"doc,omitempty"`
}

type TypeFieldUpdate struct {
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
