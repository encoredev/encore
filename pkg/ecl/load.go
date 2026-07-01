package ecl

import (
	"fmt"
	"io/fs"
	"path"
)

// Load parses the given entrypoint files from fsys and recursively
// follows their imports, returning all files combined into a RuleSet.
//
// Import paths are resolved relative to the importing file's directory
// first, then relative to the root of fsys. A file imported multiple
// times is included once; import cycles are therefore harmless.
func Load(fsys fs.FS, entrypoints ...string) (*RuleSet, error) {
	type pending struct {
		path string
		imp  *Import     // nil for entrypoints
		src  *sourceFile // file containing the import
	}

	var diags ErrorList
	rs := &RuleSet{}
	visited := make(map[string]bool)
	var queue []pending
	for _, p := range entrypoints {
		queue = append(queue, pending{path: path.Clean(p)})
	}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if visited[item.path] {
			continue
		}
		visited[item.path] = true

		data, err := fs.ReadFile(fsys, item.path)
		if err != nil {
			d := &Diagnostic{Message: fmt.Sprintf("cannot read file %q: %v", item.path, err)}
			if item.imp != nil {
				d.Pos, d.End = item.imp.PathPos, item.imp.PathEnd
				d.src = item.src
				d.Message = fmt.Sprintf("cannot read imported file %q: %v", item.path, err)
			} else {
				d.Pos = Position{File: item.path}
			}
			diags = append(diags, d)
			continue
		}

		file, err := ParseFile(item.path, data)
		if err != nil {
			diags = append(diags, err.(ErrorList)...)
		}
		if file == nil {
			continue
		}
		rs.Files = append(rs.Files, file)

		dir := path.Dir(item.path)
		for _, imp := range file.Imports {
			target, found := resolveImport(fsys, dir, imp.Path)
			if !found {
				d := &Diagnostic{
					Pos: imp.PathPos, End: imp.PathEnd, src: file.src,
					Message: fmt.Sprintf("cannot find imported file %q", imp.Path),
				}
				rel := path.Clean(path.Join(dir, imp.Path))
				if rel != path.Clean(imp.Path) {
					d.Detail = []string{fmt.Sprintf("looked for %q and %q", rel, path.Clean(imp.Path))}
				}
				diags = append(diags, d)
				continue
			}
			queue = append(queue, pending{path: target, imp: imp, src: file.src})
		}
	}

	if len(diags) > 0 {
		diags.sort()
		return nil, diags
	}
	return rs, nil
}

// resolveImport resolves an import path relative to the importing file's
// directory, falling back to the filesystem root.
func resolveImport(fsys fs.FS, dir, impPath string) (string, bool) {
	candidates := []string{
		path.Clean(path.Join(dir, impPath)),
		path.Clean(impPath),
	}
	for _, c := range candidates {
		if info, err := fs.Stat(fsys, c); err == nil && !info.IsDir() {
			return c, true
		}
	}
	return "", false
}
