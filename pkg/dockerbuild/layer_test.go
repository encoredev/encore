package dockerbuild

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
)

func TestCompressedLayerReadsSourceOnce(t *testing.T) {
	var tarData bytes.Buffer
	tw := tar.NewWriter(&tarData)
	payload := bytes.Repeat([]byte("a dependency's source code\n"), 8192)
	if err := tw.WriteHeader(&tar.Header{Name: "app.js", Size: int64(len(payload)), Mode: 0644}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	for _, level := range []int{1, 5} {
		t.Run(string(rune('0'+level)), func(t *testing.T) {
			opens := 0
			opener := func() (io.ReadCloser, error) {
				opens++
				if opens > 1 {
					return nil, errors.New("source reopened")
				}
				return io.NopCloser(bytes.NewReader(tarData.Bytes())), nil
			}
			layer, err := compressLayer(t.Context(), &Image{dir: t.TempDir()}, opener, level)
			if err != nil {
				t.Fatal(err)
			}
			if opens != 1 {
				t.Fatalf("opens = %d", opens)
			}
			// Compare to the former implementation, including the exact gzip digest.
			old, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(tarData.Bytes())), nil
			}, tarball.WithCompressionLevel(level))
			if err != nil {
				t.Fatal(err)
			}
			wantDigest, err := old.Digest()
			if err != nil {
				t.Fatal(err)
			}
			if layer.digest != wantDigest {
				t.Fatalf("compressed digest = %s, want %s", layer.digest, wantDigest)
			}
			wantDiffID, _, err := v1.SHA256(bytes.NewReader(tarData.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			if layer.diffID != wantDiffID {
				t.Fatalf("diffID = %s, want %s", layer.diffID, wantDiffID)
			}
			// Push retries and concurrent consumers read independent file handles.
			var wg sync.WaitGroup
			for range 4 {
				wg.Go(func() {
					r, err := layer.Compressed()
					if err != nil {
						t.Error(err)
						return
					}
					defer fns.CloseIgnore(r)
					digest, size, err := v1.SHA256(r)
					if err != nil {
						t.Error(err)
						return
					}
					if digest != wantDigest || size != layer.size {
						t.Errorf("blob = %s / %d, want %s / %d", digest, size, wantDigest, layer.size)
					}
				})
			}
			wg.Wait()
			r, err := layer.Uncompressed()
			if err != nil {
				t.Fatal(err)
			}
			defer fns.CloseIgnore(r)
			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tarData.Bytes()) {
				t.Fatal("uncompressed bytes changed")
			}
			if opens != 1 {
				t.Fatalf("source reopened %d times", opens)
			}
		})
	}
}

func TestImageBlobLifetimeAndParallelOrder(t *testing.T) {
	root, blobs := t.TempDir(), t.TempDir()
	source := filepath.Join(root, "app.js")
	if err := os.WriteFile(source, []byte("original source"), 0600); err != nil {
		t.Fatal(err)
	}
	spec := &ImageSpec{OS: "linux", Arch: "amd64", CopyData: map[ImagePath]HostPath{"/app.js": HostPath(source)}, BuildInfo: BuildInfoSpec{InfoPath: "/build-info.json"}}
	var digest v1.Hash
	for _, workers := range []int{1, 2, 4} {
		img, err := BuildImage(t.Context(), spec, ImageBuildConfig{TempDir: blobs, CompressionConcurrency: workers})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { fns.CloseIgnore(img) })
		d, err := img.Digest()
		if err != nil {
			t.Fatal(err)
		}
		if workers == 1 {
			digest = d
		} else if d != digest {
			t.Fatal("parallelism changed image digest")
		}
		// Once built, even removal of the source cannot change or break an export.
		if err := os.Remove(source); err != nil {
			t.Fatal(err)
		}
		if err := tarball.Write(name.MustParseReference("test:latest"), img, io.Discard); err != nil {
			t.Fatal(err)
		}
		img.mu.Lock()
		openReaders := len(img.readers)
		img.mu.Unlock()
		if openReaders != 0 {
			t.Fatalf("export leaked %d readers after EOF", openReaders)
		}
		layers, err := img.Layers()
		if err != nil {
			t.Fatal(err)
		}
		partial, err := layers[0].Compressed()
		if err != nil {
			t.Fatal(err)
		}
		defer fns.CloseIgnore(partial)
		if err := img.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := partial.Read(make([]byte, 1)); !errors.Is(err, fs.ErrClosed) {
			t.Fatalf("partial reader still open after image Close: %v", err)
		}
		if _, err := layers[0].Compressed(); !errors.Is(err, fs.ErrClosed) {
			t.Fatalf("reopened closed image: %v", err)
		}
		entries, err := os.ReadDir(blobs)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("leaked blobs: %v", entries)
		}
		if err := os.WriteFile(source, []byte("original source"), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

type badConfigImage struct{ v1.Image }

func (badConfigImage) ConfigFile() (*v1.ConfigFile, error) { return nil, errors.New("bad config") }

func TestImageCleansBlobsOnFailure(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		dir := t.TempDir()
		ctx, stop := context.WithCancel(t.Context())
		if cancel {
			stop()
		}
		img, err := BuildImage(ctx, &ImageSpec{BuildInfo: BuildInfoSpec{InfoPath: "/build-info.json"}, WriteFiles: map[ImagePath][]byte{"/config": []byte("payload")}}, ImageBuildConfig{
			TempDir: dir, BaseImageOverride: option.Some[v1.Image](badConfigImage{empty.Image}),
		})
		stop()
		if !cancel && (err == nil || !strings.Contains(err.Error(), "bad config")) {
			t.Fatalf("expected config failure after compression: %v", err)
		}
		if cancel && !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation: %v", err)
		}
		if err == nil || img != nil {
			t.Fatal("expected failed build")
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("leaked blobs: %v", entries)
		}
	}
}

type cancelLayerReader struct {
	cancel context.CancelFunc
	closed bool
}

func (r *cancelLayerReader) Read(p []byte) (int, error) { r.cancel(); return copy(p, "data"), nil }
func (r *cancelLayerReader) Close() error               { r.closed = true; return nil }

func TestCompressionCancellationClosesSource(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	src := &cancelLayerReader{cancel: cancel}
	dir := t.TempDir()
	img := &Image{dir: dir}
	layer, err := compressLayer(ctx, img, func() (io.ReadCloser, error) { return src, nil }, 5)
	if !errors.Is(err, context.Canceled) || layer != nil || !src.closed {
		t.Fatalf("layer=%v err=%v closed=%v", layer, err, src.closed)
	}
	// The image owns partial files as well; its cleanup removes them.
	if err := img.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("directory still exists: %v", err)
	}
}
