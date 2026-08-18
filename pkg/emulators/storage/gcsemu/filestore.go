package gcsemu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	cloudstorage "cloud.google.com/go/storage"
	"google.golang.org/api/storage/v1"
)

const (
	metaExtention = ".emumeta"

	// folderMarker is the file a folder placeholder object is stored as.
	//
	// Cloud Storage represents an empty folder as a zero-byte object whose name ends
	// in "/", but a filename cannot end in a path separator. The object "a/" is
	// therefore held as the file "a/<folderMarker>", which Walk reports back under the
	// object's real name, so that the rest of the emulator can treat it as the plain
	// object it is in Cloud Storage. An object genuinely named "a/<folderMarker>" is
	// indistinguishable from the placeholder, the same collision metaExtention has.
	folderMarker = ".emufolder"
)

type filestore struct {
	gcsDir string
}

var _ Store = (*filestore)(nil)

// NewFileStore returns a new Store that writes to the given directory.
func NewFileStore(gcsDir string) *filestore {
	return &filestore{gcsDir: gcsDir}
}

type composeObj struct {
	filename string
	conds    cloudstorage.Conditions
}

func (fs *filestore) CreateBucket(bucket string) error {
	bucketDir, err := fs.filename(bucket, "")
	if err != nil {
		return err
	}
	return os.MkdirAll(bucketDir, 0777)
}

func (fs *filestore) GetBucketMeta(baseUrl HttpBaseUrl, bucket string) (*storage.Bucket, error) {
	f, err := fs.filename(bucket, "")
	if err != nil {
		return nil, err
	}
	fInfo, err := os.Stat(f)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stating %s: %w", f, err)
	}

	obj := BucketMeta(baseUrl, bucket)
	obj.Updated = fInfo.ModTime().UTC().Format(time.RFC3339Nano)
	return obj, nil
}

func (fs *filestore) Get(baseUrl HttpBaseUrl, bucket string, filename string) (*storage.Object, []byte, error) {
	obj, err := fs.GetMeta(baseUrl, bucket, filename)
	if err != nil {
		return nil, nil, err
	}
	if obj == nil {
		return nil, nil, nil
	}

	f, err := fs.filename(bucket, filename)
	if err != nil {
		return nil, nil, err
	}
	contents, err := os.ReadFile(f)
	if err != nil {
		return nil, nil, fmt.Errorf("reading  %s: %w", f, err)
	}
	return obj, contents, nil
}

func (fs *filestore) GetMeta(baseUrl HttpBaseUrl, bucket string, filename string) (*storage.Object, error) {
	f, err := fs.filename(bucket, filename)
	if err != nil {
		return nil, err
	}
	fInfo, err := os.Stat(f)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stating  %s: %w", f, err)
	}

	return fs.ReadMeta(baseUrl, bucket, filename, fInfo)
}

func (fs *filestore) Add(bucket string, filename string, contents []byte, meta *storage.Object) error {
	f, err := fs.filename(bucket, filename)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f), 0777); err != nil {
		return fmt.Errorf("could not create dirs for:  %s: %w", f, err)
	}

	if err := os.WriteFile(f, contents, 0666); err != nil {
		return fmt.Errorf("could not write:  %s: %w", f, err)
	}

	// Force a new modification time, since this is what Generation is based on.
	now := time.Now().UTC()
	_ = os.Chtimes(f, now, now)

	InitScrubbedMeta(meta, filename)
	meta.Metageneration = 1
	meta.Generation = now.UnixNano()
	if meta.TimeCreated == "" {
		meta.TimeCreated = now.UTC().Format(time.RFC3339Nano)
	}
	meta.Id = fmt.Sprintf("%s/%s/%d", bucket, filename, meta.Generation)
	meta.Etag = fmt.Sprintf("%d", meta.Generation)

	fMeta := metaFilename(f)
	if err := os.WriteFile(fMeta, mustJson(meta), 0666); err != nil {
		return fmt.Errorf("could not write metadata file: %s: %w", fMeta, err)
	}

	return nil
}

func (fs *filestore) UpdateMeta(bucket string, filename string, meta *storage.Object, metagen int64) error {
	InitScrubbedMeta(meta, filename)
	meta.Metageneration = metagen

	f, err := fs.filename(bucket, filename)
	if err != nil {
		return err
	}
	fMeta := metaFilename(f)
	if err := os.WriteFile(fMeta, mustJson(meta), 0666); err != nil {
		return fmt.Errorf("could not write metadata file: %s: %w", fMeta, err)
	}

	return nil
}

func (fs *filestore) Copy(srcBucket string, srcFile string, dstBucket string, dstFile string) (bool, error) {
	// Make sure it's there
	meta, err := fs.GetMeta(dontNeedUrls, srcBucket, srcFile)
	if err != nil {
		return false, err
	}
	// Handle object-not-found
	if meta == nil {
		return false, nil
	}

	// Copy with metadata
	f1, err := fs.filename(srcBucket, srcFile)
	if err != nil {
		return false, err
	}
	contents, err := os.ReadFile(f1)
	if err != nil {
		return false, err
	}
	meta.TimeCreated = "" // reset creation time on the dest file
	err = fs.Add(dstBucket, dstFile, contents, meta)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (fs *filestore) Delete(bucket string, filename string) error {
	f, err := fs.filename(bucket, filename)
	if err != nil {
		return err
	}
	bucketDir, err := fs.filename(bucket, "")
	if err != nil {
		return err
	}

	err = func() error {
		// Check if the bucket exists
		if _, err := os.Stat(f); os.IsNotExist(err) {
			return os.ErrNotExist
		}

		// Remove the bucket
		if filename == "" {
			return os.RemoveAll(f)
		}

		// Remove just the file and the associated metadata file
		if err := os.Remove(f); err != nil {
			return err
		}
		err := os.Remove(metaFilename(f))
		if os.IsNotExist(err) {
			// Legacy files do not have an accompanying metadata file.
			return nil
		}
		return err
	}()
	if err != nil {
		if os.IsNotExist(err) {
			return err
		}
		return fmt.Errorf("could not delete %s: %w", f, err)
	}

	// Try to delete empty directories
	for fp := filepath.Dir(f); len(fp) > len(bucketDir); fp = filepath.Dir(fp) {
		files, err := os.ReadDir(fp)
		if err != nil || len(files) > 0 {
			// Quit trying to delete the directory
			break
		}
		if err := os.Remove(fp); err != nil {
			// If removing fails, quit trying
			break
		}
	}
	return nil
}

func (fs *filestore) ReadMeta(baseUrl HttpBaseUrl, bucket string, filename string, fInfo os.FileInfo) (*storage.Object, error) {
	if fInfo.IsDir() {
		return nil, nil
	}

	f, err := fs.filename(bucket, filename)
	if err != nil {
		return nil, err
	}
	obj := &storage.Object{}
	fMeta := metaFilename(f)
	buf, err := os.ReadFile(fMeta)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("could not read metadata file %s: %w", fMeta, err)
		}
	}

	if len(buf) != 0 {
		if err := json.NewDecoder(bytes.NewReader(buf)).Decode(obj); err != nil {
			return nil, fmt.Errorf("could not parse file attributes %q for %s: %w", buf, f, err)
		}
	}

	InitMetaWithUrls(baseUrl, obj, bucket, filename, uint64(fInfo.Size()))
	// obj.Generation = fInfo.ModTime().UnixNano() // use the mod time as the generation number
	obj.Updated = fInfo.ModTime().UTC().Format(time.RFC3339Nano)
	return obj, nil
}

// filename resolves the on-disk path for an object. Both bucket and filename
// originate from untrusted request paths, so we reject any component that would
// escape gcsDir via path traversal (e.g. a percent-encoded "../" that decodes
// after HTTP routing). filepath.IsLocal rejects absolute paths, "..", and other
// names that don't stay within the directory they're evaluated in.
func (fs *filestore) filename(bucket string, filename string) (string, error) {
	if !filepath.IsLocal(bucket) {
		return "", fmt.Errorf("invalid bucket name %q", bucket)
	}
	if filename == "" {
		return filepath.Join(fs.gcsDir, bucket), nil
	}
	if !filepath.IsLocal(filename) {
		return "", fmt.Errorf("invalid object name %q", filename)
	}
	if strings.HasSuffix(filename, "/") {
		// A folder placeholder. Join drops the trailing separator, which would
		// resolve the object onto the directory it names.
		return filepath.Join(fs.gcsDir, bucket, filename, folderMarker), nil
	}
	return filepath.Join(fs.gcsDir, bucket, filename), nil
}

// LocalPath implements LocalStore.
func (fs *filestore) LocalPath(bucket string, filename string) (string, error) {
	f, err := fs.filename(bucket, filename)
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(filename, "/") {
		// A folder placeholder, which fs.filename resolves to the folderMarker file
		// inside the folder ("a/" -> "a/.emufolder").
		return filepath.Dir(f), nil
	}
	return f, nil
}

func metaFilename(filename string) string {
	return filename + metaExtention
}

// Walk invokes cb for every object in the bucket in object-name order, along with the
// directories passed on the way (which are not objects, and carry no trailing "/").
//
// The ordering is load-bearing: ListObjects pages by remembering the last name it
// returned and skipping everything <= it on the next request, so a name that arrives
// out of order is dropped from the listing altogether. filepath.Walk can't provide it,
// because it orders siblings by filename while the caller compares object names, and
// the two disagree — the directory "a" holds the objects "a/…", which sort after the
// sibling file "a.txt" since '.' < '/', yet filepath.Walk visits the directory first.
func (fs *filestore) Walk(ctx context.Context, bucket string, cb func(ctx context.Context, filename string, fInfo os.FileInfo) error) error {
	root, err := fs.filename(bucket, "")
	if err != nil {
		return err
	}

	fInfo, err := os.Lstat(root)
	if err != nil {
		// A bucket that has never been written to has no directory of its own. Callers
		// recognize that with os.IsNotExist, so hand the error back unwrapped.
		return err
	}

	err = fs.walkDir(ctx, root, "", fInfo, cb)
	if errors.Is(err, filepath.SkipDir) || errors.Is(err, filepath.SkipAll) {
		return nil
	}
	return err
}

// walkEntry is a directory entry paired with the object name it is reported under.
type walkEntry struct {
	// sortKey orders the entry among its siblings by object name instead of by
	// filename. See walkDir for why the two differ.
	sortKey string

	// objName is the name cb sees, which is the entry's own name for a file, the
	// folder it stands for for a marker, and the path so far for a directory.
	objName string

	de os.DirEntry
}

// walkDir invokes cb for dir itself and then, in object-name order, for everything
// below it. objName is the name dir is reported under, "" for the bucket root.
func (fs *filestore) walkDir(ctx context.Context, dir, objName string, fInfo os.FileInfo, cb func(ctx context.Context, filename string, fInfo os.FileInfo) error) error {
	if err := cb(ctx, objName, fInfo); err != nil {
		if errors.Is(err, filepath.SkipDir) {
			return nil
		}
		return err
	}

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("walk error at %s: %w", objName, err)
	}

	// Object names are "/"-separated on every platform, matching the keys they came from.
	prefix := objName
	if prefix != "" {
		prefix += "/"
	}

	entries := make([]walkEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		name := de.Name()
		if !de.IsDir() && strings.HasSuffix(name, metaExtention) {
			// Ignore metadata files.
			continue
		}

		entry := walkEntry{sortKey: name, objName: prefix + name, de: de}
		switch {
		case de.IsDir():
			// Every object below "a" is named "a/…", so that is where "a" sorts.
			entry.sortKey = name + "/"
		case name == folderMarker:
			if prefix == "" {
				// Directly in the bucket root, so it names no folder. Not reachable
				// through Add, which rejects the key "/", but don't emit "" if it is.
				continue
			}
			// Report the marker under the name of the object it stands for, so callers
			// see the "a/" object Cloud Storage would have. That name is a prefix of
			// every other name in the directory, so it always sorts first.
			entry.objName = prefix
			entry.sortKey = ""
		}
		entries = append(entries, entry)
	}
	slices.SortFunc(entries, func(a, b walkEntry) int {
		return strings.Compare(a.sortKey, b.sortKey)
	})

	for _, entry := range entries {
		info, err := entry.de.Info()
		if err != nil {
			if os.IsNotExist(err) {
				return err
			}
			return fmt.Errorf("walk error at %s: %w", entry.objName, err)
		}

		if entry.de.IsDir() {
			if err := fs.walkDir(ctx, filepath.Join(dir, entry.de.Name()), entry.objName, info, cb); err != nil {
				return err
			}
			continue
		}

		if err := cb(ctx, entry.objName, info); err != nil {
			// filepath.Walk's convention: SkipDir from a file skips the rest of the
			// directory holding it.
			if errors.Is(err, filepath.SkipDir) {
				return nil
			}
			return err
		}
	}
	return nil
}
