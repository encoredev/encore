package ai

import (
	"bytes"
	"fmt"
	"go/token"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/exp/maps"

	"encr.dev/cli/daemon/apps"
	"encr.dev/pkg/errinsrc"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/paths"
	meta "encr.dev/proto/encore/parser/meta/v1"
	"encr.dev/v2/internals/perr"
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

type EndpointInput struct {
	ID             string         `json:"id,omitempty"`
	Name           string         `json:"name"`
	Doc            string         `json:"doc"`
	Method         string         `json:"method"`
	Visibility     VisibilityType `json:"visibility"`
	Path           []PathSegment  `json:"path"`
	RequestType    string         `json:"requestType,omitempty"`
	ResponseType   string         `json:"responseType,omitempty"`
	Errors         []*ErrorInput  `json:"errors,omitempty"`
	Types          []*TypeInput   `json:"types,omitempty"`
	Language       string         `json:"language,omitempty"`
	TypeSource     string         `json:"typeSource,omitempty"`
	EndpointSource string         `json:"endpointSource,omitempty"`
}

func (s *EndpointInput) GraphQL() *EndpointInput {
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

func (e *EndpointInput) Render() string {
	buf := strings.Builder{}
	if e.Doc != "" {
		buf.WriteString(comment(strings.TrimSpace(e.Doc) + "\n"))
	}
	buf.WriteString(renderDocList(PathDocPrefix, e.Path))
	buf.WriteString(renderDocList(ErrDocPrefix, e.Errors))
	pathStr, pathParams := formatPath(e.Path)
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

type ServiceInput struct {
	ID        string           `json:"id,omitempty"`
	Name      string           `json:"name,omitempty"`
	Doc       string           `json:"doc,omitempty"`
	Endpoints []*EndpointInput `json:"endpoints,omitempty"`
}

func (s ServiceInput) GraphQL() ServiceInput {
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
	rtn.WriteString("}")
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

func (e ErrorInput) DocItem() (string, string) {
	return e.Code, e.Doc
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

type PathParamUpdate struct {
	BaseAIUpdateType
	Service  string `json:"service,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Param    string `json:"param,omitempty"`
	Doc      string `json:"doc,omitempty"`
}

func formatPath(segs []PathSegment) (docPath string, goParams []string) {
	var params []string
	return "/" + path.Join(fns.Map(segs, func(s PathSegment) string {
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

type servicePaths struct {
	relPaths map[string]paths.RelSlash
	root     paths.FS
	module   paths.Mod
}

func (s *servicePaths) IsNew(svc string) bool {
	_, ok := s.relPaths[svc]
	return !ok
}

func (s *servicePaths) Add(svc string, path paths.RelSlash) *servicePaths {
	s.relPaths[svc] = path
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

func (s *servicePaths) FileName(svc, name string) (paths.FS, error) {
	relPath, err := s.RelFileName(svc, name)
	if err != nil {
		return "", err
	}
	return s.root.JoinSlash(relPath), nil
}

func (s *servicePaths) RelFileName(svc, name string) (paths.RelSlash, error) {
	pkgPath := s.FullPath(svc)
	name = strings.ToLower(name)
	fileName := name + ".go"
	var i int
	for {
		fspath := pkgPath.Join(fileName)
		if _, err := os.Stat(fspath.ToIO()); os.IsNotExist(err) {
			return s.RelPath(svc).Join(fileName), nil
		} else if err != nil {
			return "", err
		}
		i++
		fileName = fmt.Sprintf("%s_%d.go", name, i)
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
	path         paths.FS
	endpoint     *EndpointInput
	service      *ServiceInput
	codeType     CodeType
	content      []byte
	headerOffset token.Position
}

func (o *overlay) Type() fs.FileMode {
	return o.Mode()
}

func (o *overlay) Info() (fs.FileInfo, error) {
	return o, nil
}

func (o *overlay) Name() string {
	return o.path.Base()
}

func (o *overlay) Size() int64 {
	return int64(len(o.content))
}

func (o *overlay) Mode() fs.FileMode {
	return fs.ModePerm
}

func (o *overlay) ModTime() time.Time {
	return time.Now()
}

func (o *overlay) IsDir() bool {
	return false
}

func (o *overlay) Sys() any {
	//TODO implement me
	panic("implement me")
}

func (o *overlay) Stat() (fs.FileInfo, error) {
	return o, nil
}

func (o *overlay) File() fs.File {
	return &overlayFile{o, bytes.NewReader(o.content)}
}

type overlayFile struct {
	*overlay
	*bytes.Reader
}

func (o *overlayFile) Close() error { return nil }

var (
	_ fs.FileInfo = (*overlay)(nil)
	_ fs.DirEntry = (*overlay)(nil)
)

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

func (o *overlays) Stat(name string) (fs.FileInfo, error) {
	f, ok := o.list[paths.FS(name)]
	if !ok {
		// else return the filesystem file
		return os.Stat(name)
	}
	return f, nil
}

func (o *overlays) ReadDir(name string) ([]fs.DirEntry, error) {
	entries := map[string]fs.DirEntry{}
	osFiles, err := os.ReadDir(name)
	for _, f := range osFiles {
		entries[f.Name()] = f
	}
	dir := paths.FS(name)
	for _, info := range o.list {
		if dir == info.path.Dir() {
			entries[info.path.Base()] = info
		}
	}
	if len(entries) == 0 && err != nil {
		return nil, err
	}
	return maps.Values(entries), nil
}

func (o *overlays) PkgOverlay() map[string][]byte {
	files := map[string][]byte{}
	for f, info := range o.list {
		files[f.ToIO()] = info.content
	}
	return files
}

func (o *overlays) ReadFile(name string) ([]byte, error) {
	f, ok := o.list[paths.FS(name)]
	if !ok {
		// else return the filesystem file
		return os.ReadFile(name)
	}
	return f.content, nil
}

func (o *overlays) Open(name string) (fs.File, error) {
	f, ok := o.list[paths.FS(name)]
	if !ok {
		// else return the filesystem file
		return os.Open(name)
	}
	return f.File(), nil
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

func (o *overlays) endpoint(p paths.FS) *EndpointInput {
	ov, ok := o.get(p)
	if !ok {
		return nil
	}
	return ov.endpoint
}

func (o *overlays) validationErrors(list *perr.List) []ValidationError {
	var rtn []ValidationError
	for i := 0; i < list.Len(); i++ {
		err := list.At(i)
		if err.Params.Locations == nil {
			rtn = append(rtn, ValidationError{
				Message: err.Params.Summary,
			})
		}
		rtn = append(rtn, o.validationError(err)...)
	}
	return rtn
}

func (o *overlays) validationError(err *errinsrc.ErrInSrc) []ValidationError {
	var rtn []ValidationError
	for _, loc := range err.Params.Locations {
		o, ok := o.get(paths.FS(loc.File.FullPath))
		if !ok {
			rtn = append(rtn, ValidationError{
				Message: err.Params.Summary,
			})
			continue
		}
		rtn = append(rtn, ValidationError{
			Service:  o.service.ID,
			Endpoint: o.endpoint.ID,
			CodeType: o.codeType,
			Message:  err.Params.Summary,
			Start: &Pos{
				Line:   loc.Start.Line - o.headerOffset.Line,
				Column: loc.Start.Col - o.headerOffset.Column,
			},
			End: &Pos{
				Line:   loc.End.Line - o.headerOffset.Line,
				Column: loc.End.Col - o.headerOffset.Column,
			},
		})
	}
	return rtn
}

func (o *overlays) add(s ServiceInput, e *EndpointInput) error {
	p, err := o.paths.FileName(s.Name, e.Name+"_func")
	if err != nil {
		return err
	}
	offset, content := toSrcFile(p, s.Name, e.EndpointSource)
	e.EndpointSource = string(content[offset.Offset:])
	o.list[p] = &overlay{
		path:         p,
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
		path:         p,
		endpoint:     e,
		service:      &s,
		codeType:     CodeTypeTypes,
		content:      content,
		headerOffset: offset,
	}
	return nil
}

var (
	_ fs.ReadFileFS = (*overlays)(nil)
	_ fs.ReadDirFS  = (*overlays)(nil)
	_ fs.StatFS     = (*overlays)(nil)
)

type SyncResult struct {
	Services []ServiceInput    `json:"services"`
	Errors   []ValidationError `json:"errors"`
}
