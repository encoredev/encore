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
	"encr.dev/pkg/fns"
)

// WriteToDisk writes the contents of the given filesystem to a new temporary directory,
// so that multiple services can be loaded from a single copy of the file tree.
//
// It returns the directory along with a function that removes it again, which the
// caller must call once every service has been loaded. If an error is returned the
// directory has already been removed.
func WriteToDisk(filesys fs.FS) (dir string, cleanup func(), err error) {
	tmpPath, err := os.MkdirTemp("", "encr-cfg-")
	if err != nil {
		return "", nil, eerror.Wrap(err, "config", "unable to create temporary directory", nil)
	}

	if err = writeFSToPath(filesys, tmpPath); err != nil {
		_ = os.RemoveAll(tmpPath)
		return "", nil, err
	}

	return tmpPath, func() { _ = os.RemoveAll(tmpPath) }, nil
}

// LoadFromDir takes the app-relative path to the service's root package and a directory
// written by WriteToDisk, and loads the full configuration needed for that service.
func LoadFromDir(dir string, serviceRelPath string, meta *Meta) (cue.Value, error) {
	// Find all config files for the service
	configFilesForService, err := allFilesUnder(os.DirFS(dir), serviceRelPath)
	if err != nil {
		return cue.Value{}, eerror.Wrap(err, "config", "unable to list all config files for service", map[string]any{"path": serviceRelPath})
	}

	// If there are no config files, return an empty value
	if len(configFilesForService) == 0 {
		return cue.Value{}, nil
	}

	// Tell CUE to load all the files
	loaderCfg := &load.Config{
		Dir:   dir,
		Tools: true,
		Tags:  meta.ToTags(),
	}
	pkgs := load.Instances(configFilesForService, loaderCfg)
	for _, pkg := range pkgs {
		if pkg.Err != nil {
			return cue.Value{}, srcerrors.UnableToLoadCUEInstances(pkg.Err, dir)
		}

		// Non CUE files may be orphaned (JSON/YAML), so need to be parsed into the CUE AST and added to the package.
		if err := addOrphanedFiles(pkg); err != nil {
			return cue.Value{}, srcerrors.UnableToAddOrphanedCUEFiles(err, dir)
		}
	}

	// Build the CUE values
	ctx := cuecontext.New()
	values, err := ctx.BuildInstances(pkgs)
	if err != nil {
		return cue.Value{}, srcerrors.UnableToLoadCUEInstances(err, dir)
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
		return cue.Value{}, srcerrors.CUEEvaluationFailed(err, dir)
	}

	return rtnValue, nil
}

// allFilesUnder returns all files under the given path in the given filesystem.
func allFilesUnder(filesys fs.FS, path string) ([]string, error) {
	var files []string

	// Check if the path exists
	// and if it doesn't that means there zero CUE files for this service
	// this can happen when a Config struct has zero fields
	if _, err := fs.Stat(filesys, path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

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
func writeFSToPath(fsys fs.FS, targetPath string) error {
	// Copy the files into the temporary directory
	err := fs.WalkDir(fsys, ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return eerror.Wrap(err, "config", "unable to walk VFS", nil)
		}

		// Convert the io/fs slash-based path to a filepath.
		osPath := filepath.FromSlash(path)

		if !info.IsDir() {
			// Open the source file from our filesystem
			srcFile, err := fsys.Open(path)
			if err != nil {
				return eerror.Wrap(err, "config", "unable to open src file", nil)
			}
			defer fns.CloseIgnore(srcFile)

			dstFile, err := os.OpenFile(filepath.Join(targetPath, osPath), os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return eerror.Wrap(err, "config", "unable to open dst file", nil)
			}

			_, err = io.Copy(dstFile, srcFile)
			if err2 := dstFile.Close(); err == nil {
				err = err2
			}
			if err != nil {
				return eerror.Wrap(err, "config", "unable to copy file", nil)
			}
		} else {
			if err := os.Mkdir(filepath.Join(targetPath, osPath), 0755); err != nil && !errors.Is(err, os.ErrExist) {
				return eerror.Wrap(err, "config", "unable to make dir", nil)
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
// The majority of the code in the function is taken directly from the CUE source code as the code is currently only accessible
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
