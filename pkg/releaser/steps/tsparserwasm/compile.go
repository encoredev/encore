package tsparserwasm

import (
	"compress/gzip"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "encr.dev/pkg/releaser/bu"
	"encr.dev/pkg/releaser/steps/rustbuild"
	"github.com/cockroachdb/errors"
)

type CompileInput struct {
	Host HostInfo

	// The cargo 'target' directory to use for compiled outputs.
	CargoTargetDir FSPath

	// True if it should be built with `--release`.
	ReleaseBuild bool

	// Version string to embed in the binary.
	Version string
}

type CompileOutput struct {
	// Path to the pkg/ directory containing the wasm-pack output.
	PkgDir FSPath
}

// Compile runs wasm-pack to build the tsparser-wasm crate.
func Compile(ctx context.Context, in CompileInput) (*CompileOutput, error) {
	if _, err := exec.LookPath("wasm-pack"); err != nil {
		return nil, errors.New("wasm-pack not found in PATH; it must be installed on the build machine")
	}

	wasmDir := in.Host.RepoPath.Join("tsparser", "wasm")
	pkgDir := wasmDir.Join("pkg")

	// Acquire the cargo lock since wasm-pack runs cargo under the hood.
	rustbuild.CargoLock.Lock()
	defer rustbuild.CargoLock.Unlock()

	args := []string{"build", "--target", "web"}
	if in.ReleaseBuild {
		args = append(args, "--release")
	}

	cmd := exec.CommandContext(ctx, "wasm-pack", args...)
	cmd.Dir = wasmDir.ToIO()
	cmd.Env = append(os.Environ(), "ENCORE_VERSION="+in.Version)
	if in.CargoTargetDir != "" {
		cmd.Env = append(cmd.Env, "CARGO_TARGET_DIR="+in.CargoTargetDir.ToIO())
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrap(err, "failed to run wasm-pack build")
	}

	// Gzip all .wasm files in-place so they can be served
	// with Content-Encoding: gzip directly from S3/CloudFront.
	if err := gzipWasmFiles(pkgDir); err != nil {
		return nil, errors.Wrap(err, "failed to gzip wasm files")
	}

	return &CompileOutput{
		PkgDir: pkgDir,
	}, nil
}

func gzipWasmFiles(dir FSPath) error {
	entries, err := os.ReadDir(dir.ToIO())
	if err != nil {
		return errors.Wrap(err, "read pkg dir")
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".wasm") {
			continue
		}
		path := filepath.Join(dir.ToIO(), e.Name())
		if err := gzipFileInPlace(path); err != nil {
			return errors.Wrapf(err, "gzip %s", e.Name())
		}
	}
	return nil
}

func gzipFileInPlace(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := gzip.NewWriter(f)
	if _, err := w.Write(src); err != nil {
		return err
	}
	return w.Close()
}
