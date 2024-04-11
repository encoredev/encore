package ai

import (
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

type PathSegments []PathSegment

type PathSegment struct {
	Type      SegmentType       `json:"type,omitempty"`
	Value     *string           `json:"value,omitempty"`
	ValueType *SegmentValueType `json:"valueType,omitempty"`
	Doc       string            `graphql:"-" json:"doc,omitempty"`
}

func (p PathSegment) DocItem() (string, string) {
	return *p.Value, p.Doc
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

func (s *Endpoint) Auth() bool {
	return s.Visibility == VisibilityTypeAuth
}

// GraphQL scrubs data that is not needed for the graphql client
func (s *Endpoint) GraphQL() *Endpoint {
	s.ID = ""
	s.EndpointSource = ""
	s.TypeSource = ""
	s.Types = nil
	s.Language = ""
	for i, _ := range s.Path {
		s.Path[i].Doc = ""
	}
	return s
}

type Type struct {
	Name   string       `json:"name,omitempty"`
	Doc    string       `json:"doc,omitempty"`
	Fields []*TypeField `json:"fields,omitempty"`
}

type Service struct {
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Doc       string      `json:"doc,omitempty"`
	Endpoints []*Endpoint `json:"endpoints,omitempty"`
}

func (s Service) GetName() string {
	return s.Name
}

func (s Service) GetEndpoints() []*Endpoint {
	return s.Endpoints
}

// ServiceInput is the graphql input type for our queries
// the graphQL client we use requires the type name to match the
// graphql type
type ServiceInput Service

// GraphQL scrubs data that is not needed for the graphql client
func (s Service) GraphQL() ServiceInput {
	s.ID = ""
	for _, e := range s.Endpoints {
		e.GraphQL()
	}
	return ServiceInput(s)
}

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

type TypeUpdate struct {
	BaseAIUpdateType
	Service  string `json:"service,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Name     string `json:"name,omitempty"`
	Doc      string `graphql:"mdoc: doc" json:"doc,omitempty"`
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

// ValidationError is a simplified ErrInSrc to return to the dashboard
type ValidationError struct {
	Service  string   `json:"service"`
	Endpoint string   `json:"endpoint"`
	CodeType CodeType `json:"codeType"`
	Message  string   `json:"message"`
	Start    *Pos     `json:"start,omitempty"`
	End      *Pos     `json:"end,omitempty"`
}

type CodeType string

const (
	CodeTypeEndpoint CodeType = "endpoint"
	CodeTypeTypes    CodeType = "types"
)

type Pos struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}
