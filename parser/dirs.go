package parser

import (
	"fmt"
	"go/ast"
	"go/build"
	goparser "go/parser"
	"go/scanner"
	"go/token"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"encr.dev/parser/est"
)

// walkFunc is the callback called by walkDirs to process a directory.
// dir is the path to the current directory; relPath is the (slash-separated)
// relative path from the original root dir.
type walkFunc func(dir, relPath string, files []os.FileInfo) error

// walkDirs is like filepath.Walk but it calls walkFn once for each directory and not for individual files.
// It also reports both the full path and the path relative to the given root dir.
// It does not allow skipping directories in any way; any error returned from walkFn aborts the walk.
func walkDirs(root string, walkFn walkFunc) error {
	return walkDir(root, ".", walkFn)
}

// walkDir processes a single directory and recurses.
// dir is the current directory path, and rel is the relative path from the original root.
// rel is always in slash form, while dir uses the OS-native filepath separator.
func walkDir(dir, rel string, walkFn walkFunc) error {
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	// Split the files and dirs
	var dirs, files []os.FileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}

	if err := walkFn(dir, rel, files); err != nil {
		return err
	}

	for _, d := range dirs {
		dir2 := filepath.Join(dir, d.Name())
		rel2 := path.Join(rel, d.Name())
		if err := walkDir(dir2, rel2, walkFn); err != nil {
			return err
		}
	}
	return nil
}

// parseDir is like go/parser.ParseDir but it constructs *est.File objects instead.
func parseDir(buildContext build.Context, fset *token.FileSet, dir, relPath string, filter func(os.FileInfo) bool, mode goparser.Mode) (pkgs map[string]*ast.Package, files []*est.File, err error) {
	fd, err := os.Open(dir)
	if err != nil {
		return nil, nil, err
	}
	defer fd.Close()

	list, err := fd.Readdir(-1)
	if err != nil {
		return nil, nil, err
	}

	// Sort the slice so that we have a stable order to ensure deterministic metadata.
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })

	var errors scanner.ErrorList

	pkgs = make(map[string]*ast.Package)
	for _, d := range list {
		if strings.HasSuffix(d.Name(), ".go") && (filter == nil || filter(d)) {
			filename := filepath.Join(dir, d.Name())
			contents, err := os.ReadFile(filename)
			if err != nil {
				return nil, nil, err
			}

			// Check if this file should be part of the build
			matched, err := buildContext.MatchFile(dir, d.Name())
			if err != nil {
				errors.Add(token.Position{Filename: filename}, err.Error())
				continue
			}
			if !matched {
				continue
			}

			src, err := goparser.ParseFile(fset, filename, contents, mode)
			if err != nil || !src.Pos().IsValid() {
				// Parse error or invalid file
				if err == nil {
					err = fmt.Errorf("could not parse file %s", d.Name())
				}
				if el, ok := err.(scanner.ErrorList); ok {
					errors = append(errors, el...)
				} else {
					errors.Add(token.Position{Filename: filename}, err.Error())
				}
				continue
			}

			name := src.Name.Name
			pkg, found := pkgs[name]
			if !found {
				pkg = &ast.Package{
					Name:  name,
					Files: make(map[string]*ast.File),
				}
				pkgs[name] = pkg
			}
			pkg.Files[filename] = src
			tokFile := fset.File(src.Package)
			files = append(files, &est.File{
				Name:       d.Name(),
				AST:        src,
				Token:      tokFile,
				Contents:   contents,
				Path:       filename,
				References: make(map[ast.Node]*est.Node),
				Pkg:        nil, // will be set later
			})
		}
	}

	return pkgs, files, errors.Err()
}
