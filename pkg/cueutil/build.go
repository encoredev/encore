package cueutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"encr.dev/pkg/eerror"
	"encr.dev/pkg/errinsrc/srcerrors"
)

// LoadFromFS takes a given filesystem object and the app-relative path to the service's root package
// and loads the full configuration needed for that service.
func LoadFromFS(filesys fs.FS, serviceRelPath string, meta *Meta) (cue.Value, error) {
	// Work out of a temporary directory
	tmpPath, err := os.MkdirTemp("", "encr-cfg-")
	if err != nil {
		return cue.Value{}, eerror.Wrap(err, "config", "unable to create temporary directory", nil)
	}
	defer func() { _ = os.RemoveAll(tmpPath) }()

	// Write the FS to the file system
	err = writeFSToPath(filesys, tmpPath)
	if err != nil {
		return cue.Value{}, err
	}

	// Find all config files for the service
	configFilesForService, err := allFilesUnder(filesys, serviceRelPath)
	if err != nil {
		return cue.Value{}, eerror.Wrap(err, "config", "unable to list all config files for service", map[string]any{"path": serviceRelPath})
	}

	// Tell CUE to load all the files
	loaderCfg := &load.Config{
		Dir:   tmpPath,
		Tools: true,
		Tags:  meta.ToTags(),
	}
	pkgs := load.Instances(configFilesForService, loaderCfg)
	for _, pkg := range pkgs {
		if pkg.Err != nil {
			return cue.Value{}, srcerrors.UnableToLoadCUEInstances(pkg.Err, tmpPath)
		}

		// Non CUE files may be orphaned (JSON/YAML), so need to be parsed into the CUE AST and added to the package.
		if err := addOrphanedFiles(pkg); err != nil {
			return cue.Value{}, srcerrors.UnableToAddOrphanedCUEFiles(err, tmpPath)
		}
	}

	// Build the CUE values
	ctx := cuecontext.New()
	values, err := ctx.BuildInstances(pkgs)
	if err != nil {
		return cue.Value{}, srcerrors.UnableToLoadCUEInstances(err, tmpPath)
	}
	if len(values) == 0 {
		return cue.Value{}, eerror.New("config", "no values generated from config", nil)
	}

	// Unify all returned values into a single value
	// Note; to get all errors in the CUE files, we want to wait until
	// the validate output to check for errors
	rtnValue := values[0]
	for _, value := range values {
		rtnValue = rtnValue.Unify(value)
	}

	// Validate the unified value is concrete
	if err := rtnValue.Validate(cue.Concrete(true)); err != nil {
		return cue.Value{}, srcerrors.CUEEvaluationFailed(err, tmpPath)
	}

	return rtnValue, nil
}

// allFilesUnder returns all files under the given path in the given filesystem.
func allFilesUnder(filesys fs.FS, path string) ([]string, error) {
	var files []string
	err := fs.WalkDir(filesys, path, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

// writeFSToPath writes the contents of the given filesystem to a temporary directory on the local filesystem.
func writeFSToPath(filesys fs.FS, targetPath string) error {
	// Copy the files into the temporary directory
	err := fs.WalkDir(filesys, ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			// Open the source file from our filesystem
			srcFile, err := filesys.Open(path)
			if err != nil {
				return err
			}

			dstFile, err := os.OpenFile(filepath.Join(targetPath, path), os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}

			_, err = io.Copy(dstFile, srcFile)
			if err != nil {
				return err
			}
		} else {
			if err := os.Mkdir(filepath.Join(targetPath, path), 0755); err != nil && !errors.Is(err, os.ErrExist) {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return eerror.Wrap(err, "config", "unable to write config files to temporary directory", map[string]any{"path": targetPath})
	}
	return nil
}

// addOrphanedFiles adds any orphaned files outside the package to the build instance. This could be CUE, YAML or JSON files
//
// The majority of the code in ths function is taken directly from the CUE source code as the code is currently only acessible
// from internal paths - they are planning to move this out of the internal path so non-CUE code can directly call it
// as library functions. (Src: cue/internal/encoding/encoding.go : NewDecoder())
func addOrphanedFiles(i *build.Instance) (err error) {
	for _, f := range i.OrphanedFiles {
		var file ast.Node

		rc, err := reader(f)
		if err != nil {
			return err
		}
		//goland:noinspection GoDeferInLoop
		defer func() { _ = rc.Close() }()

		t := unicode.BOMOverride(unicode.UTF8.NewDecoder())
		r := transform.NewReader(rc, t)

		switch f.Encoding {
		case build.CUE:
			file, err = parser.ParseFile(f.Filename, r, parser.ParseComments)
			if err != nil {
				return err
			}
		default:
			return errors.New(fmt.Sprintf("unsupported encoding: %s", f.Encoding))
		}

		if err != nil {
			return err
		}
		if err := i.AddSyntax(toFile(file)); err != nil {
			return err
		}
	}

	return nil
}

// toFile converts an ast.Node to a *ast.File. (from the CUE source code)
func toFile(n ast.Node) *ast.File {
	switch x := n.(type) {
	case nil:
		return nil
	case *ast.StructLit:
		return &ast.File{Decls: x.Elts}
	case ast.Expr:
		ast.SetRelPos(x, token.NoSpace)
		return &ast.File{Decls: []ast.Decl{&ast.EmbedDecl{Expr: x}}}
	case *ast.File:
		return x
	default:
		panic(fmt.Sprintf("Unsupported node type %T", x))
	}
}

// reader returns a reader for the given file. (from the CUE source code)
func reader(f *build.File) (io.ReadCloser, error) {
	switch s := f.Source.(type) {
	case nil:
		// Use the file name.
	case string:
		return io.NopCloser(strings.NewReader(s)), nil
	case []byte:
		return io.NopCloser(bytes.NewReader(s)), nil
	case *bytes.Buffer:
		// is io.Reader, but it needs to be readable repeatedly
		if s != nil {
			return io.NopCloser(bytes.NewReader(s.Bytes())), nil
		}
	default:
		return nil, fmt.Errorf("invalid source type %T", f.Source)
	}
	return os.Open(f.Filename)
}
