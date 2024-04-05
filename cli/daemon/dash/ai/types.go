package ai

import (
	"fmt"
	"path"
	"strings"

	"encr.dev/pkg/fns"
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

const SegmentValueTypeString SegmentValueType = "string"

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

type PathSegments []PathSegment

func (p PathSegments) Render() (docPath string, goParams []string) {
	var params []string
	return "/" + path.Join(fns.Map(p, func(s PathSegment) string {
		switch s.Type {
		case SegmentTypeLiteral:
			return *s.Value
		case SegmentTypeParam:
			params = append(params, fmt.Sprintf("%s %s", *s.Value, *s.ValueType))
			return fmt.Sprintf(":%s", *s.Value)
		case SegmentTypeWildcard:
			params = append(params, fmt.Sprintf("%s %s", *s.Value, SegmentValueTypeString))
			return fmt.Sprintf("*%s", *s.Value)
		case SegmentTypeFallback:
			params = append(params, fmt.Sprintf("%s %s", *s.Value, SegmentValueTypeString))
			return fmt.Sprintf("!%s", *s.Value)
		default:
			panic(fmt.Sprintf("unknown path segment type: %s", s.Type))
		}
	})...), params
}

type PathSegment struct {
	Type      SegmentType       `json:"type,omitempty"`
	Value     *string           `json:"value,omitempty"`
	ValueType *SegmentValueType `json:"valueType,omitempty"`
	Doc       string            `graphql:"-" json:"doc,omitempty"`
}

func (p PathSegment) DocItem() (string, string) {
	return *p.Value, p.Doc
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
		return "!"
	default:
		panic(fmt.Sprintf("unknown path segment type: %s", p.Type))
	}
}

type AISessionID string

type SessionUpdate struct {
	BaseAIUpdateType
	Id AISessionID
}

type TitleUpdate struct {
	BaseAIUpdateType
	Title string
}

type Endpoint struct {
	ID             string         `json:"id,omitempty"`
	Name           string         `json:"name"`
	Doc            string         `json:"doc"`
	Method         string         `json:"method"`
	Visibility     VisibilityType `json:"visibility"`
	Path           PathSegments   `json:"path"`
	RequestType    string         `json:"requestType,omitempty"`
	ResponseType   string         `json:"responseType,omitempty"`
	Errors         []*Error       `json:"errors,omitempty"`
	Types          []*Type        `json:"types,omitempty"`
	Language       string         `json:"language,omitempty"`
	TypeSource     string         `json:"typeSource,omitempty"`
	EndpointSource string         `json:"endpointSource,omitempty"`
}

func (s *Endpoint) GraphQL() *Endpoint {
	s.ID = ""
	s.EndpointSource = ""
	s.Types = nil
	s.Language = ""
	for _, p := range s.Path {
		p.Doc = ""
	}
	return s
}

const (
	PathDocPrefix = "Path Parameters"
	ErrDocPrefix  = "Errors"
)

func indentItem(header, comment string) string {
	buf := strings.Builder{}
	buf.WriteString(header)
	for i, line := range strings.Split(strings.TrimSpace(comment), "\n") {
		indent := ""
		if i > 0 {
			indent = strings.Repeat(" ", len(header))
		}
		buf.WriteString(fmt.Sprintf("%s%s\n", indent, line))
	}
	return buf.String()
}

func renderDocList[T interface{ DocItem() (string, string) }](header string, items []T) string {
	maxLen := 0
	items = fns.Filter(items, func(p T) bool {
		key, val := p.DocItem()
		if val == "" {
			return false
		}
		maxLen = max(maxLen, len(key))
		return true
	})
	buf := strings.Builder{}
	for i, item := range items {
		if i == 0 {
			buf.WriteString(header)
			buf.WriteString(":\n")
		}
		key, value := item.DocItem()
		spacing := strings.Repeat(" ", maxLen-len(key))
		itemHeader := fmt.Sprintf(" - %s: %s", key, spacing)
		buf.WriteString(indentItem(itemHeader, value))
	}
	return comment(buf.String())
}

func comment(txt string) string {
	if txt == "" {
		return ""
	}
	buf := strings.Builder{}
	for _, line := range strings.Split(txt, "\n") {
		buf.WriteString(fmt.Sprintf("// %s\n", line))
	}
	return buf.String()
}

func (e *Endpoint) Render() string {
	buf := strings.Builder{}
	if e.Doc != "" {
		buf.WriteString(comment(strings.TrimSpace(e.Doc) + "\n"))
	}
	buf.WriteString(renderDocList(PathDocPrefix, e.Path))
	buf.WriteString(renderDocList(ErrDocPrefix, e.Errors))
	pathStr, pathParams := e.Path.Render()
	params := []string{"ctx context.Context"}
	params = append(params, pathParams...)
	if e.RequestType != "" {
		params = append(params, "req *"+e.RequestType)
	}
	var rtnParams []string
	if e.ResponseType != "" {
		rtnParams = append(rtnParams, "*"+e.ResponseType)
	}
	rtnParams = append(rtnParams, "error")
	buf.WriteString(fmt.Sprintf("//encore:api %s method=%s path=%s\n", e.Visibility, e.Method, pathStr))
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

type Service struct {
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Doc       string      `json:"doc,omitempty"`
	Endpoints []*Endpoint `json:"endpoints,omitempty"`
}

func (s Service) GraphQL() Service {
	s.ID = ""
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

type Type struct {
	Name   string       `json:"name,omitempty"`
	Doc    string       `json:"doc,omitempty"`
	Fields []*TypeField `json:"fields,omitempty"`
}

func (s *Type) Render() string {
	rtn := strings.Builder{}
	if s.Doc != "" {
		for _, line := range strings.Split(strings.TrimSpace(s.Doc), "\n") {
			rtn.WriteString(fmt.Sprintf("// %s\n", line))
		}
	}
	rtn.WriteString(fmt.Sprintf("type %s struct {\n", s.Name))
	for i, f := range s.Fields {
		if i > 0 {
			rtn.WriteString("\n")
		}
		if f.Doc != "" {
			for _, line := range strings.Split(strings.TrimSpace(f.Doc), "\n") {
				rtn.WriteString(fmt.Sprintf("  // %s\n", line))
			}
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
	rtn.WriteString("}")
	return rtn.String()
}

type LocalEndpointUpdate struct {
	Type     string    `json:"type,omitempty"`
	Service  string    `json:"service,omitempty"`
	Endpoint *Endpoint `json:"endpoint,omitempty"`
}

type TypeField struct {
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

type Error struct {
	Code string `json:"code,omitempty"`
	Doc  string `json:"doc,omitempty"`
}

func (e Error) DocItem() (string, string) {
	return e.Code, e.Doc
}

func (e Error) String() string {
	return e.Code
}

type ErrorUpdate struct {
	BaseAIUpdateType
	Code     string `json:"code,omitempty"`
	Doc      string `json:"doc,omitempty"`
	Service  string `json:"service,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
}

type PathParamUpdate struct {
	BaseAIUpdateType
	Service  string `json:"service,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Param    string `json:"param,omitempty"`
	Doc      string `json:"doc,omitempty"`
}

type SyncResult struct {
	Services []Service         `json:"services"`
	Errors   []ValidationError `json:"errors"`
}
