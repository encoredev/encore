package cueutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
	"cuelang.org/go/encoding/json"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"encr.dev/compiler/internal/cueutil/internal/yaml"
	"encr.dev/pkg/eerror"
)

// LoadFromFS takes a given filesystem object and the app-relative path to the service's root package
// and loads the full configuration needed for that service.
func LoadFromFS(filesys fs.FS, serviceRelPath string) (cue.Value, error) {
	// Write the FS to the file system
	tmpPath, err := writeFSToTmpPath(filesys)
	if err != nil {
		return cue.Value{}, err
	}
	defer func() { _ = os.RemoveAll(tmpPath) }()

	// Find all config files for the service
	configFilesForService, err := allFilesUnder(filesys, serviceRelPath)
	if err != nil {
		return cue.Value{}, eerror.Wrap(err, "config", "unable to list all config files for service", map[string]any{"path": serviceRelPath})
	}

	// Tell CUE to load all the files
	loaderCfg := &load.Config{
		Dir:   tmpPath,
		Tools: true,
	}
	pkgs := load.Instances(configFilesForService, loaderCfg)
	for _, pkg := range pkgs {
		if pkg.Err != nil {
			return cue.Value{}, toDescriptiveError(pkg.Err)
		}

		// Non CUE files may be orphaned (JSON/YAML), so need to be parsed into the CUE AST and added to the package.
		if err := addOrphanedFiles(pkg); err != nil {
			return cue.Value{}, toDescriptiveError(err)
		}
	}

	// Build the CUE values
	ctx := cuecontext.New()
	values, err := ctx.BuildInstances(pkgs)
	if err != nil {
		return cue.Value{}, toDescriptiveError(err)
	}
	if len(values) == 0 {
		return cue.Value{}, eerror.New("config", "no values generated from config", nil)
	}

	// Unify all returned values into a single value
	rtnValue := values[0]
	for _, value := range values {
		if value.Err() != nil {
			return cue.Value{}, toDescriptiveError(value.Err())
		}
		rtnValue = rtnValue.Unify(value)
	}

	// Check the unified value for errors
	if rtnValue.Err() != nil {
		return cue.Value{}, toDescriptiveError(err)
	}

	// Validate the unified value is concrete
	if err := rtnValue.Validate(cue.Concrete(true)); err != nil {
		return cue.Value{}, toDescriptiveError(err)
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

// toDescriptiveError takes a CUE error list and expands it to show each error (unless there is more than 10)
func toDescriptiveError(err error) error {
	if err == nil {
		return nil
	}

	errs := cueerrors.Errors(err)
	if len(errs) == 1 {
		return errs[0]
	}

	var str strings.Builder
	str.WriteString("the following errors where detected:\n")
	for i, err2 := range errs {
		str.WriteString(fmt.Sprintf("\t- %s\n", err2))

		if i >= 10 {
			str.WriteString("\t- ... (too many errors to show)")
			break
		}
	}

	return errors.New(str.String())
}

// writeFSToTmpPath writes the contents of the given filesystem to a temporary directory on the local filesystem.
func writeFSToTmpPath(filesys fs.FS) (string, error) {
	// Work out of a temporary directory
	tmpPath, err := os.MkdirTemp("", "encr-cfg-")
	if err != nil {
		return "", eerror.Wrap(err, "config", "unable to create temporary directory", nil)
	}

	// Copy the files into the temporary directory
	err = fs.WalkDir(filesys, ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			// Open the source file from our filesystem
			srcFile, err := filesys.Open(path)
			if err != nil {
				return err
			}

			dstFile, err := os.OpenFile(filepath.Join(tmpPath, path), os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}

			_, err = io.Copy(dstFile, srcFile)
			if err != nil {
				return err
			}
		} else {
			if err := os.Mkdir(filepath.Join(tmpPath, path), 0755); err != nil && !errors.Is(err, os.ErrExist) {
				return err
			}
		}

		return nil
	})
	if err != nil {
		_ = os.RemoveAll(tmpPath) // clear the temporary directory incase of error
		return "", eerror.Wrap(err, "config", "unable to write config files to temporary directory", map[string]any{"path": tmpPath})
	}

	return tmpPath, nil
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
		defer func() { _ = rc.Close() }()

		t := unicode.BOMOverride(unicode.UTF8.NewDecoder())
		r := transform.NewReader(rc, t)

		switch f.Encoding {
		case build.CUE:
			file, err = parser.ParseFile(f.Filename, r, parser.ParseComments)
			if err != nil {
				return err
			}
		case build.JSON, build.JSONL:
			file, err = json.NewDecoder(nil, f.Filename, r).Extract()
		case build.YAML:
			dec, err := yaml.NewDecoder(f.Filename, r)
			if err != nil {
				return err
			}
			file, err = dec.Decode()
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
		return ioutil.NopCloser(strings.NewReader(s)), nil
	case []byte:
		return ioutil.NopCloser(bytes.NewReader(s)), nil
	case *bytes.Buffer:
		// is io.Reader, but it needs to be readable repeatedly
		if s != nil {
			return ioutil.NopCloser(bytes.NewReader(s.Bytes())), nil
		}
	default:
		return nil, fmt.Errorf("invalid source type %T", f.Source)
	}
	return os.Open(f.Filename)
}
