package dockerbuild

import (
	"archive/tar"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog/log"
)

type tarCopier struct {
	fileTimes *time.Time
	tw        *tar.Writer
	seenDirs  map[ImagePath]bool
}

func newTarCopier(tw *tar.Writer, opts ...tarCopyOption) *tarCopier {
	tc := &tarCopier{
		tw:       tw,
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
}

func (tc *tarCopier) CopyData(spec *ImageSpec) error {
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

func (tc *tarCopier) CopyDir(desc *dirCopyDesc) error {
	err := filepath.WalkDir(string(desc.SrcPath), func(pathStr string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		path := HostPath(pathStr)

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
		dstPath := desc.DstPath.Join(string(relPath.ToImage()))

		// If this is a symlink, compute the link target relative to DstPath.
		var link ImagePath
		if d.Type()&fs.ModeSymlink != 0 {
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
		err = tc.CopyFile(dstPath, path, fi, link)
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
		relFromSrcPath, err = desc.SrcPath.Rel(absTarget)
		if err != nil {
			return "", err
		}

		// If the relative path is local to the SrcPath, allow it.
		if filepath.IsLocal(relFromSrcPath.String()) {
			return desc.DstPath.JoinImage(relFromSrcPath.ToImage()), nil
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

	for dstPath != "." && dstPath != "/" {
		if !tc.seenDirs[dstPath] {
			modTime := time.Time{}
			if tc.fileTimes != nil {
				modTime = *tc.fileTimes
			}
			header := &tar.Header{
				Typeflag: tar.TypeDir,
				ModTime:  modTime,
				Name:     (dstPath + "/").String(), // from [archive/tar.FileInfoHeader]
				Mode:     int64(mode.Perm()),
			}
			if err := tc.tw.WriteHeader(header); err != nil {
				return errors.Wrap(err, "write tar header")
			}
			tc.seenDirs[dstPath] = true
		}

		dstPath = dstPath.Dir()
	}

	return nil
}

func (tc *tarCopier) CopyFile(dstPath ImagePath, srcPath HostPath, fi fs.FileInfo, linkTarget ImagePath) (err error) {
	header, err := tar.FileInfoHeader(fi, linkTarget.String())
	if err != nil {
		return err
	}
	if tc.fileTimes != nil {
		t := *tc.fileTimes
		header.ModTime = t
		header.AccessTime = t
		header.ChangeTime = t
	}

	header.Name = filepath.ToSlash(dstPath.String())
	if err := tc.tw.WriteHeader(header); err != nil {
		return errors.Wrap(err, "write tar header")
	}

	if fi.IsDir() {
		tc.seenDirs[dstPath] = true
		return nil
	}

	// If this is not a symlink, write the file.
	if (fi.Mode() & fs.ModeSymlink) != fs.ModeSymlink {
		// Write the file
		f, err := os.Open(srcPath.String())
		if err != nil {
			return errors.Wrap(err, "open file")
		}
		defer func() {
			if closeErr := f.Close(); err == nil {
				err = errors.Wrap(closeErr, "close file")
			}
		}()

		if _, err = io.Copy(tc.tw, f); err != nil {
			return errors.Wrap(err, "copy file")
		}
	}

	return nil
}

func (tc *tarCopier) WriteFile(dstPath string, mode fs.FileMode, data []byte) (err error) {
	header := &tar.Header{
		Name:     dstPath,
		Typeflag: tar.TypeReg,
		Mode:     int64(mode.Perm()),
		Size:     int64(len(data)),
	}
	if tc.fileTimes != nil {
		t := *tc.fileTimes
		header.ModTime = t
		header.AccessTime = t
		header.ChangeTime = t
	}

	header.Name = filepath.ToSlash(dstPath)
	if err := tc.tw.WriteHeader(header); err != nil {
		return errors.Wrap(err, "write tar header")
	}

	_, err = tc.tw.Write(data)
	return errors.Wrap(err, "copy file")
}
