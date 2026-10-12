package dockerbuild

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
)

// Captured from the serial assembler before parallel preparation/read-ahead,
// running on Linux. Tar hashes pin exact headers, contents, symlinks, filtering
// and DFS order. Symlink modes are normalized to 0777 so macOS agrees.
func TestTarPreparationGolden(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses Unix modes and symlinks")
	}
	spec := tarGoldenFixture(t)
	want := []string{
		"sha256:b5e3a1a89f24e88e228aaf3b5aaafff51e6ca1c899173c0e96758b45f0d64171",
		"sha256:182a34f2d7128ceac5160f5ccc161d8c50ebdf059dc688252956e8206950e41c",
		"sha256:26cc06029f9a9c6a627bc516cbd7c974685b2285d1ae91d73846fcfb49a116da",
	}
	for _, workers := range []int{1, 2, 4} {
		for _, budget := range []int{-1, 64 << 10, 256 << 10, 1 << 20} {
			cfg := ImageBuildConfig{PreparationConcurrency: workers, ReadAheadBytes: budget}
			layers, err := buildImageFilesystem(t.Context(), spec, &cfg)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, layer := range layers {
				r, err := layer.opener()
				if err != nil {
					t.Fatal(err)
				}
				h, _, err := v1.SHA256(r)
				fns.CloseIgnore(r)
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, h.String())
			}
			if !slices.Equal(got, want) {
				t.Fatalf("workers=%d ahead=%d: %q", workers, budget, got)
			}
		}
	}
}

func TestTarOptionsPreserveCompressedDigest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses Unix modes and symlinks")
	}
	spec := tarGoldenFixture(t)
	// Exercise more than four real disk sources in the dependency layer. Keep
	// the original fixture unchanged for the pre-optimization tar goldens above.
	bundle, ok := spec.BundleSource.Get()
	if !ok {
		t.Fatal("fixture has no source bundle")
	}
	for i := range 6 {
		path := filepath.Join(bundle.Source.String(), "node_modules", "pkg", fmt.Sprintf("parallel-%d.js", i))
		if err := os.WriteFile(path, bytes.Repeat([]byte{byte(i)}, 128<<10), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, level := range []int{1, 5} {
		var want []v1.Hash
		for _, cfg := range []ImageBuildConfig{
			{},
			{PreparationConcurrency: 1, ReadAheadBytes: -1},
			{PreparationConcurrency: 4, ReadAheadBytes: 64 << 10, ReadAheadConcurrency: 1},
			{PreparationConcurrency: 2, ReadAheadBytes: 256 << 10, ReadAheadConcurrency: 2},
			{PreparationConcurrency: 4, ReadAheadBytes: 64 << 10, ReadAheadConcurrency: 4},
			{PreparationConcurrency: 4, ReadAheadBytes: 1 << 20},
		} {
			cfg.CompressionLevel = level
			img, err := BuildImage(t.Context(), spec, cfg)
			if err != nil {
				t.Fatal(err)
			}
			layers, err := img.Layers()
			if err != nil {
				fns.CloseIgnore(img)
				t.Fatal(err)
			}
			var got []v1.Hash
			for _, l := range layers {
				h, err := l.Digest()
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, h)
				r, err := l.Compressed()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.Copy(io.Discard, r); err != nil {
					t.Fatal(err)
				}
				fns.CloseIgnore(r)
			}
			if err := img.Close(); err != nil {
				t.Fatal(err)
			}
			if want == nil {
				want = got
			} else if !slices.Equal(got, want) {
				t.Fatalf("level %d changed gzip bytes", level)
			}
		}
	}
}

func tarGoldenFixture(t *testing.T) *ImageSpec {
	t.Helper()
	root := t.TempDir()
	// a/x before a.txt distinguishes WalkDir DFS order from sorting full paths.
	files := map[string][]byte{
		"a/x": []byte("nested"), "a.txt": []byte("sibling"), "empty": nil,
		"node_modules/pkg/index.js":                     bytes.Repeat([]byte("export const value = 42;\n"), 40000),
		"node_modules/pkg/fixtures/.modules.yaml":       []byte("keep"),
		"node_modules/.modules.yaml":                    []byte("skip"),
		"node_modules/.pnpm-workspace-state-v1.json":    []byte("skip"),
		"excluded/secret":                               []byte("skip"),
		"deep/" + strings.Repeat("long-", 30) + "/file": []byte("long header"),
	}
	for name, data := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Make the golden independent of the process umask. Directory and file
	// modes match the original fixture captured with umask 022.
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		mode := fs.FileMode(0644)
		if d.IsDir() {
			mode = 0755
		}
		return os.Chmod(path, mode)
	}); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"link": "a/x", "node_modules/pkg/link": "index.js", "escape": "../outside"} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	return &ImageSpec{OS: "linux", Arch: "amd64",
		BundleSource: option.Some(BundleSourceSpec{Source: HostPath(root), Dest: "/workspace", AppRootRelpath: ".", ExcludeSource: []RelPath{"excluded"}}),
		BuildInfo:    BuildInfoSpec{InfoPath: "/encore/build-info.json"},
		WriteFiles:   map[ImagePath][]byte{"/encore/meta": []byte("metadata")},
	}
}

func TestTarReadAheadRejectsTruncatedSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large-file")
	if err := os.WriteFile(path, make([]byte, 2<<20), 0644); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	tc := newTarCopier(setFileTimes(layerEpoch))
	if err := tc.CopyFile("/file", HostPath(path), fi, ""); err != nil {
		t.Fatal(err)
	}
	opener := tc.Opener()
	if err := os.Truncate(path, 500000); err != nil {
		t.Fatal(err)
	}
	// Real source -> read-ahead -> gzip: failure must reach image assembly rather
	// than producing a corrupt tar or panicking in a background goroutine.
	if _, err := compressLayer(t.Context(), &Image{dir: t.TempDir()}, opener, 5); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("compress truncated source = %v", err)
	}
}
