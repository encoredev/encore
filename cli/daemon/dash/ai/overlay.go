package ai

import (
	"bytes"
	"fmt"
	"go/token"
	"io/fs"
	"os"
	"strings"
	"time"

	"golang.org/x/exp/maps"

	"encr.dev/cli/daemon/apps"
	"encr.dev/pkg/errinsrc"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/idents"
	"encr.dev/pkg/paths"
	meta "encr.dev/proto/encore/parser/meta/v1"
	"encr.dev/v2/internals/perr"
)

// servicePaths is a helper struct to manage mapping between service names, pkg paths and filepaths
// It's created by parsing the metadata of the app
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
	name = idents.Convert(name, idents.SnakeCase)
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

// An overlay is a virtual file that is used to store the source code of an endpoint or types
// It automatically generates a header with pkg name and imports.
// It implements fs.FileInfo and fs.DirEntry interfaces
type overlay struct {
	path         paths.FS
	endpoint     *Endpoint
	service      *Service
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

// overlayFile is a wrapper which implements the fs.File interface
type overlayFile struct {
	*overlay
	*bytes.Reader
}

func (o *overlayFile) Close() error { return nil }

var (
	_ fs.FileInfo = (*overlay)(nil)
	_ fs.DirEntry = (*overlay)(nil)
)

func newOverlays(app *apps.Instance, overwrite bool, services ...Service) (*overlays, error) {
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

// overlays is a collection of virtual files that are used to store the source code of endpoints and types
// in memory. It's modelled as a filesystem and implements the fs.ReadFileFS, fs.ReadDirFS and fs.StatFS interfaces.
// It's used as an overlay for the Go and Encore parsers to include our in-progress code in the parsing process.
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

// validationErrors converts a perr.List into a slice of ValidationErrors
func (o *overlays) validationErrors(list *perr.List) []ValidationError {
	var rtn []ValidationError
	for i := 0; i < list.Len(); i++ {
		err := list.At(i)
		rtn = append(rtn, o.validationError(err)...)
	}
	return rtn
}

// validationError translates errinsrc.ErrInSrc into a ValidationError which is a simplified error
// used for displaying errors in the dashboard
func (o *overlays) validationError(err *errinsrc.ErrInSrc) []ValidationError {
	if err.Params.Locations == nil {
		return []ValidationError{{
			Message: err.Params.Summary,
		}}
	}
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

// add creates new overlays for an endpoint and its types.
// We create separate overlays for each endpoint and its types to allow for easier parsing and code generation.
func (o *overlays) add(s Service, e *Endpoint) error {
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
