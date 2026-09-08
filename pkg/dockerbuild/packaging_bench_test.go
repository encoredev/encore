package dockerbuild

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
)

// BenchmarkImagePackaging measures the real file-backed assembler and complete
// serialization without a VM or registry. One operation is a wave of jobs sharing
// CPUs and disk; stage metrics are average per-job latency under that contention.
// Filesystem caches are warm. Inputs combine many JS files and a native binary;
// they are deterministic synthetic fixtures, not estimates for a specific app.
func BenchmarkImagePackaging(b *testing.B) {
	logger := log.Logger
	log.Logger = log.Output(io.Discard)
	defer func() { log.Logger = logger }()
	for _, layout := range []string{"balanced", "dependencies"} {
		b.Run(layout, func(b *testing.B) {
			spec, bytes := packagingFixture(b, layout)
			for _, legacy := range []bool{true, false} {
				for _, level := range []int{1, 5} {
					for _, workers := range []int{1, 2, 4} {
						if legacy && (level != 5 || workers != 1) {
							continue
						}
						for _, jobs := range []int{1, 4} {
							method := "blob"
							if legacy {
								method = "legacy"
							}
							b.Run(fmt.Sprintf("%s/gzip%d/workers%d/jobs%d", method, level, workers, jobs), func(b *testing.B) {
								cfg := ImageBuildConfig{TempDir: b.TempDir(), CompressionLevel: level, CompressionConcurrency: workers}
								type result struct {
									assemble, serialize time.Duration
									size                int64
								}
								results := make([]result, jobs)
								var assemble, serialize time.Duration
								var size int64
								b.SetBytes(bytes * int64(jobs))
								b.ReportAllocs()
								b.ResetTimer()
								for range b.N {
									var g errgroup.Group
									for j := range jobs {
										g.Go(func() error {
											start := time.Now()
											var img v1.Image
											var err error
											if legacy {
												img, err = legacyPackagingImage(b.Context(), spec, cfg)
											} else {
												var built *Image
												built, err = BuildImage(b.Context(), spec, cfg)
												if err == nil {
													defer fns.CloseIgnore(built)
													img = built
												}
											}
											if err != nil {
												return err
											}
											assembled := time.Now()
											manifest, err := img.Manifest()
											if err != nil {
												return err
											}
											var blobSize int64
											for _, l := range manifest.Layers {
												blobSize += l.Size
											}
											if err := tarball.Write(name.MustParseReference("bench:latest"), img, io.Discard); err != nil {
												return err
											}
											results[j] = result{assembled.Sub(start), time.Since(assembled), blobSize}
											return nil
										})
									}
									if err := g.Wait(); err != nil {
										b.Fatal(err)
									}
									for _, r := range results {
										assemble += r.assemble
										serialize += r.serialize
										size += r.size
									}
								}
								b.StopTimer()
								n := float64(b.N * jobs)
								b.ReportMetric(float64(assemble)/float64(time.Millisecond)/n, "assemble-ms/job")
								b.ReportMetric(float64(serialize)/float64(time.Millisecond)/n, "serialize-ms/job")
								b.ReportMetric(float64(size)/(1<<20)/n, "blob-MiB/job")
							})
						}
					}
				}
			}
		})
	}
}

// The former compression algorithm: serial source tar construction, eager digest and
// diffID calculation, followed by lazy re-compression during serialization.
func legacyPackagingImage(ctx context.Context, spec *ImageSpec, cfg ImageBuildConfig) (v1.Image, error) {
	cfg.PreparationConcurrency, cfg.ReadAheadBytes = 1, -1
	layers, err := buildImageFilesystem(ctx, spec, &cfg)
	if err != nil {
		return nil, err
	}
	var adds []v1.Layer
	for _, l := range layers {
		layer, err := tarball.LayerFromOpener(l.opener, tarball.WithCompressionLevel(cfg.CompressionLevel))
		if err != nil {
			return nil, err
		}
		adds = append(adds, layer)
	}
	return mutate.AppendLayers(empty.Image, adds...)
}

func packagingFixture(b *testing.B, layout string) (*ImageSpec, int64) {
	b.Helper()
	root := b.TempDir()
	rng := rand.New(rand.NewSource(1))
	runtimeMiB, depsMiB, appMiB := 8, 8, 8
	if layout == "dependencies" {
		runtimeMiB, depsMiB, appMiB = 2, 32, 2
	}
	write := func(path string, data []byte) {
		b.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			b.Fatal(err)
		}
	}
	writeCode := func(dir string, mib int) {
		for file := range mib * 64 {
			data := make([]byte, 16<<10)
			// Some opaque data (maps/assets) plus compressible but varying code.
			if _, err := rng.Read(data[:len(data)/4]); err != nil {
				b.Fatal(err)
			}
			for offset := len(data) / 4; offset < len(data); {
				line := fmt.Sprintf("export function f%d(arg) { return { key: %d, value: arg.property%d }; }\n", rng.Intn(1000), rng.Intn(100000), rng.Intn(100))
				offset += copy(data[offset:], line)
			}
			write(filepath.Join(dir, fmt.Sprintf("pkg-%04d", file), "index.js"), data)
		}
	}
	writeCode(filepath.Join(root, "app", "node_modules"), depsMiB)
	writeCode(filepath.Join(root, "app", "src"), appMiB)
	native := make([]byte, runtimeMiB<<20)
	if _, err := rng.Read(native[:len(native)*3/4]); err != nil {
		b.Fatal(err)
	}
	write(filepath.Join(root, "runtime.node"), native)
	return &ImageSpec{
		OS: "linux", Arch: "amd64", Entrypoint: []string{"node", "/workspace/main.js"},
		BundleSource: option.Some(BundleSourceSpec{Source: HostPath(filepath.Join(root, "app")), Dest: "/workspace", AppRootRelpath: "."}),
		CopyData:     map[ImagePath]HostPath{"/encore/runtimes/js/runtime.node": HostPath(filepath.Join(root, "runtime.node"))},
		WriteFiles:   map[ImagePath][]byte{"/encore/meta": make([]byte, 64<<10)},
		BuildInfo:    BuildInfoSpec{InfoPath: "/encore/build-info.json"},
	}, int64(runtimeMiB+depsMiB+appMiB) << 20
}
