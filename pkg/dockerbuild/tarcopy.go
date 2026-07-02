package dockerbuild

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/rs/zerolog/log"

	"encr.dev/pkg/option"
	"encr.dev/pkg/tarstream"
	"encr.dev/pkg/xos"
	"encr.dev/v2/compiler/build"
)

// layerKind identifies the image layer a file is placed in. The image
// contents are split into layers by how frequently they are expected to
// change, so that unchanged layers keep identical digests across builds
// and can be skipped when pushing and pulling images.
type layerKind int

const (
	certsLayer   layerKind = iota // CA certificates
	runtimeLayer                  // Encore runtime and supervisor binaries
	depsLayer                     // third-party dependencies (node_modules)
	appLayer                      // application source code and compiled artifacts
	configLayer                   // build info, app metadata, and runtime configuration
)

// layerOrder lists the layers from bottom to top,
// ordered by how frequently their contents are expected to change.
var layerOrder = []layerKind{certsLayer, runtimeLayer, depsLayer, appLayer, configLayer}

func (k layerKind) String() string {
	switch k {
	case certsLayer:
		return "ca-certificates"
	case runtimeLayer:
		return "encore-runtime"
	case depsLayer:
		return "dependencies"
	case appLayer:
		return "application"
	case configLayer:
		return "configuration"
	default:
		return fmt.Sprintf("layer-%d", int(k))
	}
}

// layeredTarCopier groups tar copiers by the image layer they produce.
type layeredTarCopier struct {
	opts   []tarCopyOption
	layers map[layerKind]*tarCopier
}

func newLayeredTarCopier(opts ...tarCopyOption) *layeredTarCopier {
	return &layeredTarCopier{
		opts:   opts,
		layers: make(map[layerKind]*tarCopier),
	}
}

// Layer returns the tar copier for the given layer, creating it if necessary.
func (lc *layeredTarCopier) Layer(kind layerKind) *tarCopier {
	tc, ok := lc.layers[kind]
	if !ok {
		tc = newTarCopier(lc.opts...)
		lc.layers[kind] = tc
	}
	return tc
}

// imageLayer is a single layer to include in the image.
type imageLayer struct {
	kind   layerKind
	opener tarball.Opener
}

// Layers returns an opener for each non-empty layer, ordered bottom to top.
func (lc *layeredTarCopier) Layers() []imageLayer {
	var layers []imageLayer
	for _, kind := range layerOrder {
		tc := lc.layers[kind]
		if tc == nil || len(tc.entries) == 0 {
			continue
		}
		layers = append(layers, imageLayer{kind: kind, opener: tc.Opener()})
	}
	return layers
}

type tarCopier struct {
	fileTimes *time.Time
	entries   []*tarEntry
	seenDirs  map[ImagePath]bool
}

func newTarCopier(opts ...tarCopyOption) *tarCopier {
	tc := &tarCopier{
		seenDirs: make(map[ImagePath]bool),
	}
	for _, opt := range opts {
		opt(tc)
	}
	return tc
}

type tarCopyOption func(*tarCopier)

func setFileTimes(t time.Time) tarCopyOption {
	return func(tc *tarCopier) {
		tc.fileTimes = &t
	}
}

// dirCopyDesc describes how to copy a directory to the tar.
type dirCopyDesc struct {
	Spec    *ImageSpec
	SrcPath HostPath
	DstPath ImagePath

	// Src paths to exclude.
	ExcludeSrcPaths map[HostPath]bool

	// Src paths to include.
	IncludeSrcPaths []HostPath

	// DepsCopier, if set, receives all entries within node_modules directories
	// so that third-party dependencies are placed in their own layer.
	DepsCopier *tarCopier
}

// CopyData copies the spec's CopyData entries, routing Encore runtime
// components to the runtime layer and everything else to the app layer.
func (lc *layeredTarCopier) CopyData(spec *ImageSpec) error {
	// Sort the paths by the destination path so that the tar file is deterministic.
	type pathPair struct {
		Src  HostPath
		Dest ImagePath
	}

	var paths []pathPair
	for dest, src := range spec.CopyData {
		paths = append(paths, pathPair{Src: src, Dest: dest})
	}
	sort.Slice(paths, func(i, j int) bool {
		return paths[i].Dest < paths[j].Dest
	})

	for _, p := range paths {
		tc := lc.Layer(appLayer)
		if isRuntimePath(p.Dest) {
			tc = lc.Layer(runtimeLayer)
		}

		fi, err := os.Stat(string(p.Src))
		if err != nil {
			return errors.Wrap(err, "stat source file")
		}
		if err := tc.MkdirAll(p.Dest.Dir(), 0755); err != nil {
			return errors.Wrap(err, "create dirs")
		}
		if fi.IsDir() {
			err = tc.CopyDir(&dirCopyDesc{
				Spec:            spec,
				SrcPath:         p.Src,
				DstPath:         p.Dest,
				ExcludeSrcPaths: nil,
				IncludeSrcPaths: []HostPath{p.Src},
			})
		} else {
			err = tc.CopyFile(p.Dest, p.Src, fi, "")
		}
		if err != nil {
			return errors.Wrap(err, "copy path")
		}
	}

	return nil
}

// isRuntimePath reports whether the image path contains Encore runtime components.
func isRuntimePath(p ImagePath) bool {
	return p == runtimesBaseDir || strings.HasPrefix(string(p), string(runtimesBaseDir)+"/")
}

// nodeModulesPath reports whether relPath is within a node_modules tree, and
// whether it is the root node_modules directory of that tree.
func nodeModulesPath(relPath HostPath) (within, isRoot bool) {
	components := strings.Split(string(relPath.ToUnix()), "/")
	for i, c := range components {
		if c == "node_modules" {
			return true, i == len(components)-1
		}
	}
	return false, false
}

// volatileDepMetadata are package-manager bookkeeping files written directly
// inside a node_modules directory whose contents change on every install even
// when the installed dependencies are identical (e.g. pnpm records a prunedAt
// timestamp in .modules.yaml). They are not read at runtime, and including them
// gives the dependency layer a new digest each build. That defeats layer
// caching, so the whole node_modules layer is re-pushed and re-pulled on every
// deploy. See https://github.com/pnpm/pnpm/issues/9474.
var volatileDepMetadata = map[string]bool{
	".modules.yaml":                 true, // pnpm: volatile prunedAt + machine-specific storeDir
	".pnpm-workspace-state.json":    true, // pnpm (older): lastValidatedTimestamp
	".pnpm-workspace-state-v1.json": true, // pnpm (current): lastValidatedTimestamp
	".package-lock.json":            true, // npm hidden lockfile (defensive)
	".yarn-state.yml":               true, // yarn node-modules linker state (defensive)
}

// isVolatileDepMetadata reports whether relPath is a volatileDepMetadata file
// located directly inside a node_modules directory (so a coincidentally-named
// file shipped deep inside a package is not skipped).
func isVolatileDepMetadata(relPath HostPath) bool {
	components := strings.Split(string(relPath.ToUnix()), "/")
	n := len(components)
	if n < 2 {
		return false
	}
	return components[n-2] == "node_modules" && volatileDepMetadata[components[n-1]]
}

// shouldInclude returns true if the path should be included in the tar.
func shouldInclude(desc *dirCopyDesc, path HostPath) bool {
	for _, include := range desc.IncludeSrcPaths {
		if string(path) == string(include) {
			return true
		}

		if strings.HasPrefix(string(path), string(include)) {
			return true
		}

		if strings.HasPrefix(string(include), string(path)) {
			return true
		}
	}

	return false
}

func (tc *tarCopier) CopyDir(desc *dirCopyDesc) error {
	err := filepath.WalkDir(string(desc.SrcPath), func(pathStr string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		path := HostPath(pathStr)

		// Should we keep this path?
		if !shouldInclude(desc, path) {
			if d.IsDir() {
				return filepath.SkipDir
			} else {
				return nil
			}
		}

		// Should we skip this path?
		if desc.ExcludeSrcPaths[path] {
			if d.IsDir() {
				return filepath.SkipDir
			} else {
				return nil
			}
		}

		relPath, err := desc.SrcPath.Rel(path)
		if err != nil {
			return errors.WithStack(err)
		}

		// Skip volatile package-manager bookkeeping files so the dependency
		// layer digest stays stable across installs. See volatileDepMetadata
		// and https://github.com/pnpm/pnpm/issues/9474.
		if !d.IsDir() && isVolatileDepMetadata(relPath) {
			return nil
		}

		dstPath := desc.DstPath.Join(string(relPath.ToImage()))

		// Route node_modules trees to the dependency layer's copier, if configured.
		dst := tc
		if desc.DepsCopier != nil {
			if within, isRoot := nodeModulesPath(relPath); within {
				dst = desc.DepsCopier
				if isRoot {
					// Entering a node_modules tree: make sure its parent
					// directories exist in the dependency layer.
					if err := dst.MkdirAll(dstPath.Dir(), 0755); err != nil {
						return errors.Wrap(err, "create deps dirs")
					}
				}
			}
		}

		// If this is a symlink, compute the link target relative to DstPath.
		var link ImagePath

		isSymlink := d.Type()&fs.ModeSymlink != 0
		if !isSymlink && runtime.GOOS == "windows" {
			// Check if the file is a junction point on Windows.
			if isJunction, _ := xos.IsWindowsJunctionPoint(pathStr); isJunction {
				return errors.Newf("%q is a windows junction point and cannot be copied to a docker image. Use symlinks instead.", pathStr)
			}

		}

		if isSymlink {
			target, err := os.Readlink(string(path))
			if err != nil {
				return errors.WithStack(err)
			}

			link, err = tc.rewriteSymlink(desc, path, HostPath(target))
			if err != nil {
				return errors.WithStack(err)
			} else if link == "" {
				// Drop the symlink
				return nil
			}
		}

		fi, err := d.Info()
		if err != nil {
			return errors.WithStack(err)
		}
		err = dst.CopyFile(dstPath, path, fi, link)
		return errors.Wrap(err, "add file")
	})

	return errors.WithStack(err)
}

// rewriteSymlink rewrites the symlink to the target filesystem.
func (tc *tarCopier) rewriteSymlink(desc *dirCopyDesc, path HostPath, linkTarget HostPath) (newTarget ImagePath, err error) {
	var (
		absTarget      HostPath
		relFromSrcPath HostPath
	)

	if linkTarget.IsAbs() {
		// It's a link to an absolute destination.
		// Determine its relative path, and see if that lives within the desc.SrcPath.
		absTarget = linkTarget
		// On Windows, we can only make a relative link if the source and target are on the same volume.
		if runtime.GOOS != "windows" || filepath.VolumeName(desc.SrcPath.String()) == filepath.VolumeName(absTarget.String()) {
			relFromSrcPath, err = desc.SrcPath.Rel(absTarget)
			if err != nil {
				return "", err
			}

			// If the relative path is local to the SrcPath, allow it.
			if filepath.IsLocal(relFromSrcPath.String()) {
				return desc.DstPath.JoinImage(relFromSrcPath.ToImage()), nil
			}
		}
	} else {
		// We have a relative link target. Determine its absolute destination.
		// Use path.Dir() since the symlink is relative to its directory, not relative to itself.
		absTarget = path.Dir().JoinHost(linkTarget)

		// Determine its relative path, and see if that lives within the desc.SrcPath.
		relFromSrcPath, err = desc.SrcPath.Rel(absTarget)
		if err != nil {
			return "", err
		}

		// If the relative path is local to the SrcPath, allow it.
		if filepath.IsLocal(relFromSrcPath.String()) {
			return desc.DstPath.JoinImage(relFromSrcPath.ToImage()), nil
		}
	}

	// Otherwise, determine if the absTarget is within some other path being copied.
	absTargetStr := absTarget.String()
	for dst, src := range desc.Spec.CopyData {
		srcStr := src.String()
		stcStrSep := srcStr + string(filepath.Separator)
		if absTargetStr == srcStr {
			return dst, nil
		} else if suffix, found := strings.CutPrefix(absTargetStr, stcStrSep); found {
			// It lives within the target. Compute the new target path.
			return dst.Join(suffix), nil
		}
	}

	log.Debug().
		Str("target", linkTarget.String()).
		Str("rel_target", relFromSrcPath.String()).
		Str("abs_target", absTarget.String()).
		Msg("dropping escaping symlink")

	return "", nil
}

func (tc *tarCopier) MkdirAll(dstPath ImagePath, mode fs.FileMode) (err error) {
	dstPath = ImagePath(filepath.ToSlash(dstPath.String()))
	dstPath = dstPath.Clean()

	var dirs []ImagePath
	for dstPath != "." && dstPath != "/" {
		if !tc.seenDirs[dstPath] {
			dirs = append(dirs, dstPath)
			tc.seenDirs[dstPath] = true
		}
		dstPath = dstPath.Dir()
	}

	// Sort by name so parent directories appear before children.
	slices.Sort(dirs)

	for _, d := range dirs {
		modTime := time.Time{}
		if tc.fileTimes != nil {
			modTime = *tc.fileTimes
		}
		header := &tar.Header{
			Typeflag: tar.TypeDir,
			ModTime:  modTime,
			Name:     tarHeaderName(d, true),
			Mode:     int64(mode.Perm()),
		}
		tc.entries = append(tc.entries, &tarEntry{
			header: header,
		})
	}

	return nil
}

func (tc *tarCopier) CopyFile(dstPath ImagePath, srcPath HostPath, fi fs.FileInfo, linkTarget ImagePath) (err error) {
	header, err := tar.FileInfoHeader(fi, linkTarget.String())
	if err != nil {
		return err
	}
	if tc.fileTimes != nil {
		header.ModTime = *tc.fileTimes
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}
	}

	// Normalize ownership so the layer digest is reproducible across machines;
	// FileInfoHeader populates these from the host file.
	header.Uid = 0
	header.Gid = 0
	header.Uname = ""
	header.Gname = ""

	// HACK: make the linux binary executable when cross compiling from windows as the unix permissions gets lost.
	if runtime.GOOS == "windows" && fi.Name() == build.BinaryName {
		header.Mode = 0755
	}

	header.Name = tarHeaderName(dstPath, fi.IsDir())
	entry := &tarEntry{header: header}
	tc.entries = append(tc.entries, entry)

	if fi.IsDir() {
		tc.seenDirs[dstPath] = true
		return nil
	}

	// If this is not a symlink, write the file.
	if (fi.Mode() & fs.ModeSymlink) != fs.ModeSymlink {
		entry.hostPath = option.Some(srcPath)
	}

	return nil
}

func (tc *tarCopier) WriteFile(dstPath ImagePath, mode fs.FileMode, data []byte) (err error) {
	header := &tar.Header{
		Name:     tarHeaderName(dstPath, false),
		Typeflag: tar.TypeReg,
		Mode:     int64(mode.Perm()),
		Size:     int64(len(data)),
	}
	if tc.fileTimes != nil {
		header.ModTime = *tc.fileTimes
	}

	tc.entries = append(tc.entries, &tarEntry{
		header: header,
		data:   option.Some(data),
	})
	return nil
}

type tarEntry struct {
	header *tar.Header

	data     option.Option[[]byte]
	hostPath option.Option[HostPath]
}

func (tc *tarCopier) Opener() tarball.Opener {
	errThunk := func(err error) tarball.Opener {
		return func() (io.ReadCloser, error) {
			return nil, err
		}
	}

	var dvecs []tarstream.Datavec
	for _, e := range tc.entries {

		// create buffer to write tar header to
		buf := new(bytes.Buffer)
		tw := tar.NewWriter(buf)

		// write tar header to buffer
		if err := tw.WriteHeader(e.header); err != nil {
			return errThunk(errors.Wrap(err, fmt.Sprintf("writing header %v", e)))
		}

		memv := tarstream.MemVec{
			Data: buf.Bytes(),
		}

		// add the tar header mem buffer to the tarvec
		dvecs = append(dvecs, memv)

		var dataEntry tarstream.Datavec
		if hostPath, ok := e.hostPath.Get(); ok {
			fi := e.header.FileInfo()
			dataEntry = &tarstream.PathVec{
				Path: hostPath.String(),
				Info: fi,
			}
		} else if data, ok := e.data.Get(); ok {
			dataEntry = tarstream.MemVec{Data: data}
		}

		if dataEntry != nil {
			// add the file path info to the tarvec
			dvecs = append(dvecs, dataEntry)

			// tar requires file entries to be padded out to 512 bytes.
			if !e.header.FileInfo().IsDir() {
				if size := dataEntry.GetSize(); size%512 != 0 {
					padv := tarstream.PadVec{
						Size: 512 - (size % 512),
					}
					dvecs = append(dvecs, padv)
				}
			}
		}
	}

	tv := tarstream.NewTarVec(dvecs)
	return func() (io.ReadCloser, error) {
		tv2 := tv.Clone()
		return tv2, nil
	}
}

func tarHeaderName(p ImagePath, isDir bool) string {
	name := strings.TrimPrefix(filepath.ToSlash(p.String()), "/")
	if isDir {
		name += "/"
	}
	return name
}
