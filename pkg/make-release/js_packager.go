package main

import (
	"io/fs"
	osPkg "os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type JSPackager struct {
	log              zerolog.Logger
	WorkspaceRoot    string // Path to the yarn workspace root
	Version          string // the new version number
	DistFolder       string // the folder to output the compiled JS to
	compileCompleted chan struct{}
	compileFailed    atomic.Bool
}

func (j *JSPackager) PatchVersions() error {
	j.log.Info().Msg("Patching versions...")

	replaceVersionRegex, err := regexp.Compile(`"version": "([^-"]+)(?:-[^"]+)?",`)
	if err != nil {
		j.log.Err(err).Msg("failed to compile regex")
		return errors.Wrap(err, "compile regex")
	}

	// For all the folders in the packages dir
	packages, err := j.listPackages(filepath.Join(j.WorkspaceRoot, "packages"), "")
	if err != nil {
		return errors.Wrap(err, "list packages")
	}

	for _, pkg := range packages {
		// Look for a line containing `"version": ".*"`, and replace it
		// with `"version": "0.0.0"`.
		// (note we drop the "v" prefix for the package.json)
		newFile := replaceVersionRegex.ReplaceAll(pkg.PackageJsonData, []byte(`"version": "`+j.Version[1:]+`",`))

		// Write the file back to disk
		err = osPkg.WriteFile(pkg.PackageJsonPath, newFile, 0644)
		if err != nil {
			j.log.Err(err).Str("package", pkg.Name).Msg("failed to write package.json")
			return errors.Wrap(err, "write package.json")
		}
	}

	j.log.Info().Msg("Patching internal-runtime/conf/version.ts...")
	err = osPkg.WriteFile(
		filepath.Join(j.WorkspaceRoot, "packages", "@encore.dev", "internal-runtime", "conf", "version.ts"),
		[]byte(`// Code generated by /pkg/make-release. DO NOT EDIT.

/**
 * The current version of the runtime.
 */
export const Version = "`+j.Version+`";
`),
		0644,
	)
	if err != nil {
		j.log.Err(err).Msg("failed to patch version.ts")
		return errors.Wrap(err, "write patch version.ts")
	}

	return nil
}

type packageInfo struct {
	RootDir         string
	Name            string
	PackageJsonPath string
	PackageJsonData []byte
}

func (j *JSPackager) listPackages(dir string, basePkgName string) ([]packageInfo, error) {
	var pkgs []packageInfo

	// For all the folders in the packages dir
	entries, err := osPkg.ReadDir(dir)
	if err != nil {
		j.log.Err(err).Str("dir", dir).Msg("failed to read directory")
		return nil, errors.Wrap(err, "read directory")
	}

	for _, e := range entries {
		pkgDir := filepath.Join(dir, e.Name())
		switch {
		case !e.IsDir():
			continue
		case strings.HasPrefix(e.Name(), "@"):
			children, err := j.listPackages(pkgDir, e.Name())
			if err != nil {
				return nil, err
			}
			pkgs = append(pkgs, children...)
		default:
			pkgJsonPath := filepath.Join(pkgDir, "package.json")
			pkgJson, err := osPkg.ReadFile(pkgJsonPath)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			} else if err != nil {
				return nil, errors.Wrap(err, "read package.json")
			}

			pkgName := e.Name()
			if basePkgName != "" {
				pkgName = basePkgName + "/" + pkgName
			}

			pkgs = append(pkgs, packageInfo{
				RootDir:         pkgDir,
				Name:            pkgName,
				PackageJsonPath: pkgJsonPath,
				PackageJsonData: pkgJson,
			})
		}
	}

	return pkgs, nil
}

func (j *JSPackager) Package() (rtnErr error) {
	defer func() {
		if rtnErr != nil {
			j.compileFailed.Store(true)
		}
		close(j.compileCompleted)
	}()

	j.DistFolder = filepath.Join(j.WorkspaceRoot, "dist")

	// Remove the existing dist directory
	if err := osPkg.RemoveAll(j.DistFolder); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			j.log.Err(err).Msg("failed to remove dist directory")
			return errors.Wrap(err, "remove dist directory")
		}
	}

	// Patch the versions
	if err := j.PatchVersions(); err != nil {
		j.log.Err(err).Msg("failed to patch versions")
		return errors.Wrap(err, "patch versions")
	}

	// Ensure our node_modules is up to date and installed
	log.Info().Msg("Installing JS dependencies...")
	err := j.yarn("install")
	if err != nil {
		j.log.Err(err).Msg("yarn install failed")
		return errors.Wrap(err, "yarn install")
	}

	// Now clean up previous builds inside yarn
	log.Info().Msg("Cleaning up from previous builds...")
	err = j.yarn("clean")
	if err != nil {
		j.log.Err(err).Msg("yarn clean failed")
		return errors.Wrap(err, "yarn clean")
	}

	log.Info().Msg("Building JS...")
	err = j.yarn("build", "--filter=./packages/**/*", "--force") // only build our packages
	if err != nil {
		j.log.Err(err).Msg("yarn build failed")
		return errors.Wrap(err, "yarn build")
	}

	log.Info().Msg("Fixing dist folder for ESM")
	err = j.yarn("fix-dist")
	if err != nil {
		j.log.Err(err).Msg("yarn fix-dist failed")
		return errors.Wrap(err, "yarn fix-dist")
	}

	// Now it's all build we can start the publish/packing process
	log.Info().Msg("Publishing packages to npm...")
	npmTag := "latest"
	switch {
	case strings.Contains(j.Version, "-beta."):
		npmTag = "beta"
	case strings.Contains(j.Version, "-nightly."):
		npmTag = "nightly"
	}
	err = j.yarn("workspaces", "foreach", "--no-private", "-pt", "npm", "publish", "--tolerate-republish", "--access", "public", "--tag", npmTag)
	if err != nil {
		j.log.Err(err).Msg("yarn publish failed")
		return errors.Wrap(err, "yarn publish")
	}

	// Create our dist folder
	err = osPkg.MkdirAll(j.DistFolder, 0755)
	if err != nil {
		j.log.Err(err).Msg("failed to create dist directory")
		return errors.Wrap(err, "create dist directory")
	}

	// Package up our JS runtime
	log.Info().Msg("Packaging JS runtime packages...")
	err = j.yarn("workspaces", "foreach", "--no-private", "pack", "--out", filepath.Join(j.DistFolder, "%s.tgz"))
	if err != nil {
		j.log.Err(err).Msg("yarn pack failed")
		return errors.Wrap(err, "yarn pack")
	}

	// Now extract all the tarballs in our dist folder, deleting the tarballs
	// as we go.
	log.Info().Msg("Extracting JS runtime packages...")
	dirEntries, err := osPkg.ReadDir(j.DistFolder)
	if err != nil {
		j.log.Err(err).Msg("failed to read dist directory")
		return errors.Wrap(err, "read dist directory")
	}
	for _, entry := range dirEntries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) != ".tgz" {
			continue
		}

		name := entry.Name()
		name = strings.TrimSuffix(name, filepath.Ext(name))
		dirToExtractTo := filepath.Join(j.DistFolder, name)
		dirToExtractTo = strings.ReplaceAll(dirToExtractTo, "@encore.dev-", "@encore.dev/")

		// Create the target directory
		err = osPkg.MkdirAll(dirToExtractTo, 0755)
		if err != nil {
			j.log.Err(err).Str("tgz_file", entry.Name()).Str("target", dirToExtractTo).Msg("failed to create target directory")
			return errors.Wrap(err, "create target directory")
		}

		// Extract the tarball
		out, err := exec.Command("tar", "-xzf", filepath.Join(j.DistFolder, entry.Name()), "--strip-components", "1", "-C", dirToExtractTo).CombinedOutput()
		// nosemgrep
		if err != nil {
			j.log.Err(err).Str("tgz_file", entry.Name()).Str("target", dirToExtractTo).Str("output", string(out)).Msg("failed to extract tarball")
			return errors.Wrap(err, "extract tarball")
		}

		// Delete the tarball
		err = osPkg.Remove(filepath.Join(j.DistFolder, entry.Name()))
		if err != nil {
			j.log.Err(err).Str("tgz_file", entry.Name()).Str("target", dirToExtractTo).Msg("failed to delete tarball")
			return errors.Wrap(err, "delete tarball")
		}
	}

	log.Info().Msg("JS runtime packaged successfully")
	return nil
}

func (j *JSPackager) yarn(args ...string) error {
	cmd := exec.Command("yarn", args...)
	cmd.Env = osPkg.Environ()
	cmd.Dir = j.WorkspaceRoot
	cmd.Stdout = osPkg.Stdout
	cmd.Stderr = osPkg.Stderr

	j.log.Debug().Str("cmd", cmd.String()).Msg("running yarn command...")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "yarn command failed")
	}

	return nil
}
