package gcsemu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	cloudstorage "cloud.google.com/go/storage"
	"google.golang.org/api/storage/v1"
)

const (
	metaExtention = ".emumeta"
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
	bucketDir := filepath.Join(fs.gcsDir, bucket)
	return os.MkdirAll(bucketDir, 0777)
}

func (fs *filestore) GetBucketMeta(baseUrl HttpBaseUrl, bucket string) (*storage.Bucket, error) {
	f := fs.filename(bucket, "")
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

	f := fs.filename(bucket, filename)
	contents, err := os.ReadFile(f)
	if err != nil {
		return nil, nil, fmt.Errorf("reading  %s: %w", f, err)
	}
	return obj, contents, nil
}

func (fs *filestore) GetMeta(baseUrl HttpBaseUrl, bucket string, filename string) (*storage.Object, error) {
	f := fs.filename(bucket, filename)
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
	f := fs.filename(bucket, filename)
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

	fMeta := metaFilename(fs.filename(bucket, filename))
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
	f1 := fs.filename(srcBucket, srcFile)
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
	f := fs.filename(bucket, filename)

	err := func() error {
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
	for fp := filepath.Dir(f); len(fp) > len(fs.filename(bucket, "")); fp = filepath.Dir(fp) {
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

	f := fs.filename(bucket, filename)
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

func (fs *filestore) filename(bucket string, filename string) string {
	if filename == "" {
		return filepath.Join(fs.gcsDir, bucket)
	}
	return filepath.Join(fs.gcsDir, bucket, filename)
}

func metaFilename(filename string) string {
	return filename + metaExtention
}

func (fs *filestore) Walk(ctx context.Context, bucket string, cb func(ctx context.Context, filename string, fInfo os.FileInfo) error) error {
	root := filepath.Join(fs.gcsDir, bucket)
	return filepath.Walk(root, func(path string, fInfo os.FileInfo, err error) error {
		if strings.HasSuffix(path, metaExtention) {
			// Ignore metadata files
			return nil
		}

		filename := strings.TrimPrefix(path, root)
		filename = strings.TrimPrefix(filename, "/")
		if err != nil {
			if os.IsNotExist(err) {
				return err
			}
			return fmt.Errorf("walk error at %s: %w", filename, err)
		}

		if err := cb(ctx, filename, fInfo); err != nil {
			return err
		}
		return nil
	})
}
