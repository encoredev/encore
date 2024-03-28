package ai

import (
	"fmt"
	"go/token"
	"os"
	"path"
	"strings"

	"golang.org/x/exp/maps"

	"encr.dev/cli/daemon/apps"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/paths"
	meta "encr.dev/proto/encore/parser/meta/v1"
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
	Errors         []*ErrorInput  `json:"errors,omitempty"`
	Types          []*TypeInput   `json:"types,omitempty"`
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
		for _, line := range strings.Split(strings.TrimSpace(e.Doc), "\n") {
			buf.WriteString(fmt.Sprintf("// %s\n", line))
		}
	}
	for i, err := range e.Errors {
		if i == 0 {
			buf.WriteString("//\n// Errors:\n")
		}
		errHeader := fmt.Sprintf(" - %s: ", err.Code)
		buf.WriteString("//" + errHeader)
		for i, line := range strings.Split(strings.TrimSpace(err.Doc), "\n") {
			indent := ""
			if i > 0 {
				indent = "//" + strings.Repeat(" ", len(errHeader))
			}
			buf.WriteString(fmt.Sprintf("%s%s\n", indent, line))
		}
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
	if buf.Len() > 0 {
		buf.WriteString("//\n")
	}
	buf.WriteString(fmt.Sprintf("//encore:api %s method=%s path=%s\n", e.Visibility, e.Method, path))
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
	rtn.WriteString("}\n")
	return rtn.String()
}

type LocalEndpointUpdate struct {
	Type     string         `json:"type,omitempty"`
	Service  string         `json:"service,omitempty"`
	Endpoint *EndpointInput `json:"endpoint,omitempty"`
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

func formatPath(segs []PathSegment) (string, []string) {
	var params []string
	return "/" + path.Join(fns.Map(segs, func(s PathSegment) string {
		switch s.Type {
		case SegmentTypeLiteral:
			return *s.Value
		case SegmentTypeParam:
			params = append(params, fmt.Sprintf("%s %s", *s.Value, valueTypeToGoType(s.ValueType)))
			return fmt.Sprintf(":%s", *s.Value)
		case SegmentTypeWildcard:
			return "*"
		case SegmentTypeFallback:
			return "!fallback"
		default:
			panic(fmt.Sprintf("unknown path segment type: %s", s.Type))
		}
	})...), params
}

type servicePaths struct {
	relPaths map[string]paths.RelSlash
	root     paths.FS
	module   paths.Mod
}

func (s *servicePaths) Add(key string, path paths.RelSlash) *servicePaths {
	s.relPaths[key] = path
	return s
}

func (s *servicePaths) PkgPath(svc string) paths.Pkg {
	rel := s.RelPath(svc)
	return s.module.Pkg(rel)
}

func (s *servicePaths) FullPath(svc string) paths.FS {
	rel := s.RelPath(svc)
	return s.root.JoinSlash(rel)
}

func (s *servicePaths) RelPath(svc string) paths.RelSlash {
	pkgName, ok := s.relPaths[svc]
	if !ok {
		pkgName = paths.RelSlash(strings.ToLower(svc))
	}
	return pkgName
}

func (s *servicePaths) FileName(svc, endpoint string) (paths.FS, error) {
	relPath, err := s.RelFileName(svc, endpoint)
	if err != nil {
		return "", err
	}
	return s.root.JoinSlash(relPath), nil
}

func (s *servicePaths) RelFileName(svc, endpoint string) (paths.RelSlash, error) {
	pkgPath := s.FullPath(svc)
	endpoint = strings.ToLower(endpoint)
	fileName := endpoint + ".go"
	var i int
	for {
		fspath := pkgPath.Join(fileName)
		if _, err := os.Stat(fspath.ToIO()); os.IsNotExist(err) {
			return s.RelPath(svc).Join(fileName), nil
		} else if err != nil {
			return "", err
		}
		i++
		fileName = fmt.Sprintf("%s_%d.go", endpoint, i)
	}
}

func newServicePaths(app *apps.Instance) (*servicePaths, error) {
	md, err := app.CachedMetadata()
	if err != nil {
		return nil, err
	}
	pkgRelPath := fns.ToMap(md.Pkgs, func(p *meta.Package) string { return p.RelPath })
	svcPaths := &servicePaths{
		relPaths: map[string]paths.RelSlash{},
		root:     paths.FS(app.Root()),
		module:   paths.Mod(md.ModulePath),
	}
	for _, svc := range md.Svcs {
		if pkgRelPath[svc.RelPath] != nil {
			svcPaths.Add(svc.Name, paths.RelSlash(pkgRelPath[svc.RelPath].RelPath))
		}
	}
	return svcPaths, nil
}

type overlay struct {
	endpoint     *EndpointInput
	service      *ServiceInput
	codeType     CodeType
	content      []byte
	headerOffset token.Position
}

func newOverlays(app *apps.Instance, overwrite bool, services ...ServiceInput) (*overlays, error) {
	svcPaths, err := newServicePaths(app)
	if err != nil {
		return nil, err
	}
	o := &overlays{
		list:  map[paths.FS]*overlay{},
		paths: svcPaths,
	}
	for _, s := range services {
		for _, e := range s.Endpoints {
			if overwrite {
				e.TypeSource = ""
				e.EndpointSource = ""
			}
			if err := o.add(s, e); err != nil {
				return nil, err
			}
		}
	}
	return o, nil
}

type overlays struct {
	list  map[paths.FS]*overlay
	paths *servicePaths
}

func (o *overlays) pkgPaths() []paths.Pkg {
	pkgs := map[paths.Pkg]struct{}{}
	for _, info := range o.list {
		pkgs[o.paths.PkgPath(info.service.Name)] = struct{}{}
	}
	return maps.Keys(pkgs)
}

func (o *overlays) get(p paths.FS) (*overlay, bool) {
	rtn, ok := o.list[p]
	return rtn, ok
}

func (o *overlays) add(s ServiceInput, e *EndpointInput) error {
	p, err := o.paths.FileName(s.Name, e.Name+"_func")
	if err != nil {
		return err
	}
	offset, content := toSrcFile(p, s.Name, e.EndpointSource)
	e.EndpointSource = string(content[offset.Offset:])
	o.list[p] = &overlay{
		endpoint:     e,
		service:      &s,
		codeType:     CodeTypeEndpoint,
		content:      content,
		headerOffset: offset,
	}
	p, err = o.paths.FileName(s.Name, e.Name+"_types")
	if err != nil {
		return err
	}
	offset, content = toSrcFile(p, s.Name, e.TypeSource)
	e.TypeSource = string(content[offset.Offset:])
	o.list[p] = &overlay{
		endpoint:     e,
		service:      &s,
		codeType:     CodeTypeTypes,
		content:      content,
		headerOffset: offset,
	}
	return nil
}

func (p *overlays) files() map[paths.FS][]byte {
	files := map[paths.FS][]byte{}
	for f, info := range p.list {
		files[f] = info.content
	}
	return files
}

type SyncResult struct {
	Services []ServiceInput    `json:"services"`
	Errors   []ValidationError `json:"errors"`
}
