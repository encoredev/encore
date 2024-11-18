package gcsemu

import (
	"context"
	"os"
	"sync"
	"time"
	"fmt"

	"github.com/google/btree"
	"google.golang.org/api/storage/v1"
)

type memstore struct {
	mu      sync.RWMutex
	buckets map[string]*memBucket
}

var _ Store = (*memstore)(nil)

// NewMemStore returns a Store that operates purely in memory.
func NewMemStore() *memstore {
	return &memstore{buckets: map[string]*memBucket{}}
}

type memBucket struct {
	created time.Time

	// mutex required (despite lock map in gcsemu), because btree mutations are not structurally safe
	mu    sync.RWMutex
	files *btree.BTree
}

func (ms *memstore) getBucket(bucket string) *memBucket {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.buckets[bucket]
}

type memFile struct {
	meta storage.Object
	data []byte
}

func (mf *memFile) Less(than btree.Item) bool {
	// TODO(dragonsinth): is a simple lexical sort ok for Walk?
	return mf.meta.Name < than.(*memFile).meta.Name
}

var _ btree.Item = (*memFile)(nil)

func (ms *memstore) CreateBucket(bucket string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if ms.buckets[bucket] == nil {
		ms.buckets[bucket] = &memBucket{
			created: time.Now(),
			files:   btree.New(16),
		}
	}
	return nil
}

func (ms *memstore) GetBucketMeta(baseUrl HttpBaseUrl, bucket string) (*storage.Bucket, error) {
	if b := ms.getBucket(bucket); b != nil {
		obj := BucketMeta(baseUrl, bucket)
		obj.Updated = b.created.UTC().Format(time.RFC3339Nano)
		return obj, nil
	}
	return nil, nil
}

func (ms *memstore) Get(baseUrl HttpBaseUrl, bucket string, filename string) (*storage.Object, []byte, error) {
	f := ms.find(bucket, filename)
	if f != nil {
		return &f.meta, f.data, nil
	}
	return nil, nil, nil
}

func (ms *memstore) GetMeta(baseUrl HttpBaseUrl, bucket string, filename string) (*storage.Object, error) {
	f := ms.find(bucket, filename)
	if f != nil {
		meta := f.meta
		InitMetaWithUrls(baseUrl, &meta, bucket, filename, uint64(len(f.data)))
		return &meta, nil
	}
	return nil, nil
}

func (ms *memstore) Add(bucket string, filename string, contents []byte, meta *storage.Object) error {
	_ = ms.CreateBucket(bucket)

	InitScrubbedMeta(meta, filename)
	meta.Metageneration = 1

	// Cannot be overridden by caller
	now := time.Now().UTC()
	meta.Updated = now.UTC().Format(time.RFC3339Nano)
	meta.Generation = now.UnixNano()
	if meta.TimeCreated == "" {
		meta.TimeCreated = meta.Updated
	}
	meta.Id = fmt.Sprintf("%s/%s/%d", bucket, filename, meta.Generation)
	meta.Etag = fmt.Sprintf("%d", meta.Generation)

	b := ms.getBucket(bucket)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.files.ReplaceOrInsert(&memFile{
		meta: *meta,
		data: contents,
	})
	return nil
}

func (ms *memstore) UpdateMeta(bucket string, filename string, meta *storage.Object, metagen int64) error {
	f := ms.find(bucket, filename)
	if f == nil {
		return os.ErrNotExist
	}

	InitScrubbedMeta(meta, filename)
	meta.Metageneration = metagen

	b := ms.getBucket(bucket)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.files.ReplaceOrInsert(&memFile{
		meta: *meta,
		data: f.data,
	})
	return nil
}

func (ms *memstore) Copy(srcBucket string, srcFile string, dstBucket string, dstFile string) (bool, error) {
	src := ms.find(srcBucket, srcFile)
	if src == nil {
		return false, nil
	}

	// Copy with metadata
	meta := src.meta
	meta.TimeCreated = "" // reset creation time on the dest file
	err := ms.Add(dstBucket, dstFile, src.data, &meta)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (ms *memstore) Delete(bucket string, filename string) error {
	if filename == "" {
		// Remove the bucket
		ms.mu.Lock()
		defer ms.mu.Unlock()
		if _, ok := ms.buckets[bucket]; !ok {
			return os.ErrNotExist
		}

		delete(ms.buckets, bucket)
	} else if b := ms.getBucket(bucket); b != nil {
		// Remove just the file
		b.mu.Lock()
		defer b.mu.Unlock()
		if b.files.Delete(ms.key(filename)) == nil {
			// case file does not exist
			return os.ErrNotExist
		}
	} else {
		return os.ErrNotExist
	}

	return nil
}

func (ms *memstore) ReadMeta(baseUrl HttpBaseUrl, bucket string, filename string, _ os.FileInfo) (*storage.Object, error) {
	return ms.GetMeta(baseUrl, bucket, filename)
}

func (ms *memstore) Walk(ctx context.Context, bucket string, cb func(ctx context.Context, filename string, fInfo os.FileInfo) error) error {
	if b := ms.getBucket(bucket); b != nil {
		var err error
		b.mu.RLock()
		defer b.mu.RUnlock()
		b.files.Ascend(func(i btree.Item) bool {
			mf := i.(*memFile)
			err = cb(ctx, mf.meta.Name, nil)
			return err == nil
		})
		return nil
	}
	return os.ErrNotExist
}

func (ms *memstore) key(filename string) btree.Item {
	return &memFile{
		meta: storage.Object{
			Name: filename,
		},
	}
}

func (ms *memstore) find(bucket string, filename string) *memFile {
	if b := ms.getBucket(bucket); b != nil {
		b.mu.Lock()
		defer b.mu.Unlock()
		f := b.files.Get(ms.key(filename))
		if f != nil {
			return f.(*memFile)
		}
	}
	return nil
}
