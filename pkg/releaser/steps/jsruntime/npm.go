package jsruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"encr.dev/pkg/encorebuild/gentypedefs"
	. "encr.dev/pkg/releaser/bu"
	"github.com/cockroachdb/errors"
)

type PrepareNPMPackageInput struct {
	Version         string
	PackagePath     FSPath
	NapiTypeDefPath FSPath
}

// napiRelPath is the relative path from the package root to the napi directory.
var napiRelPath = filepath.Join("internal", "runtime", "napi")

type PrepareNPMPackageOutput struct {
	// Path to the built package, as a .tar.gz file.
	PackageTargz FSPath
}

func PrepareNPMPackage(ctx context.Context, in PrepareNPMPackageInput) (*PrepareNPMPackageOutput, error) {
	// Generate type definition files.
	{
		napiPath := in.PackagePath.Join(napiRelPath)
		napiPath.MkdirAll()

		cfg := gentypedefs.Config{
			ReleaseVersion: in.Version,
			TypeDefFile:    in.NapiTypeDefPath.ToIO(),
			DtsOutputFile:  napiPath.Join("napi.d.cts").ToIO(),
			CjsOutputFile:  napiPath.Join("napi.cjs").ToIO(),
		}
		if err := gentypedefs.Generate(cfg); err != nil {
			return nil, errors.Wrap(err, "generating napi type definitions")
		}
	}

	// Generate dist folder.
	{
		base := in.PackagePath
		distPath := base.Join("dist")
		if err := os.RemoveAll(distPath.ToIO()); err != nil {
			return nil, errors.Wrap(err, "removing dist folder")
		}
		distPath.MkdirAll()

		// Run 'npm install'.
		{
			cmd := exec.Command("npm", "install")
			cmd.Dir = base.ToIO()
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return nil, errors.Wrap(err, "running npm install")
			}
		}

		path := os.Getenv("PATH")
		path = path + string(filepath.ListSeparator) + base.Join("node_modules", ".bin").ToIO()
		env := append(os.Environ(), "PATH="+path)

		// Run 'npm run build'.
		{
			cmd := exec.Command("npm", "run", "build")
			cmd.Dir = base.ToIO()
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Env = env
			if err := cmd.Run(); err != nil {
				return nil, errors.Wrap(err, "running npm install")
			}
		}

		// Copy the napi directory over.
		{
			src := in.PackagePath.Join(napiRelPath)
			dst := distPath.Join(napiRelPath)
			cmd := exec.Command("cp", "-r", src.ToIO(), dst.ToIO())
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return nil, errors.Wrap(err, "copying napi directory")
			}
		}

		// Run 'tsc-esm-fix'.
		{
			cmd := exec.Command("./node_modules/.bin/tsc-esm-fix", "--target=dist")
			cmd.Dir = base.ToIO()
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Env = env
			if err := cmd.Run(); err != nil {
				return nil, errors.Wrap(err, "running tsc-esm-fix")
			}
		}
	}

	// Run 'npm version'.
	{
		npmVersion := strings.TrimPrefix(in.Version, "v")
		cmd := exec.Command("npm", "version", "--no-git-tag-version", "--no-commit-hooks", npmVersion)
		cmd.Dir = in.PackagePath.ToIO()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, errors.Wrap(err, "running npm version")
		}
	}

	// Run 'npm pack'.
	{
		var buf bytes.Buffer
		cmd := exec.Command("npm", "pack", "--pack-destination", ".", "--json")
		cmd.Dir = in.PackagePath.ToIO()
		cmd.Stdout = &buf
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, errors.Wrap(err, "running npm pack")
		}

		var packageDesc []struct {
			Name     string `json:"name"`
			Filename string `json:"filename"`
		}
		if err := json.Unmarshal(buf.Bytes(), &packageDesc); err != nil {
			return nil, errors.Wrap(err, "parsing npm pack output")
		} else if len(packageDesc) == 0 || packageDesc[0].Name != "encore.dev" {
			return nil, errors.New("unexpected npm pack output")
		}

		return &PrepareNPMPackageOutput{
			PackageTargz: in.PackagePath.Join(packageDesc[0].Filename),
		}, nil
	}
}
