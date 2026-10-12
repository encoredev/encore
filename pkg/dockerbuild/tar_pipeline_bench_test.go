package dockerbuild

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"encr.dev/pkg/fns"
)

// BenchmarkImageTarPipeline isolates the two tar optimizations with the same
// compressor (gzip 5, two layers), including serialization and cleanup. Each
// operation is a wave of jobs sharing immutable inputs, disk and GOMAXPROCS.
func BenchmarkImageTarPipeline(b *testing.B) {
	logger := log.Logger
	log.Logger = log.Output(io.Discard)
	defer func() { log.Logger = logger }()
	for _, layout := range []string{"balanced", "dependencies"} {
		b.Run(layout, func(b *testing.B) {
			spec, size := packagingFixture(b, layout)
			for _, workers := range []int{1, 2, 4} {
				for _, ahead := range []int{-1, 64 << 10, 256 << 10, 1 << 20} {
					for _, jobs := range []int{1, 4} {
						b.Run(fmt.Sprintf("prepare%d/ahead%d/jobs%d", workers, max(0, ahead>>10), jobs), func(b *testing.B) {
							cfg := ImageBuildConfig{PreparationConcurrency: workers, ReadAheadBytes: ahead, CompressionLevel: 5, CompressionConcurrency: 2, TempDir: b.TempDir()}
							var total time.Duration
							durations := make([]time.Duration, jobs)
							b.SetBytes(size * int64(jobs))
							b.ReportAllocs()
							b.ResetTimer()
							for range b.N {
								var g errgroup.Group
								for j := range jobs {
									g.Go(func() error {
										start := time.Now()
										img, err := BuildImage(b.Context(), spec, cfg)
										if err != nil {
											return err
										}
										defer fns.CloseIgnore(img)
										if err := tarball.Write(name.MustParseReference("bench:latest"), img, io.Discard); err != nil {
											return err
										}
										durations[j] = time.Since(start)
										return nil
									})
								}
								if err := g.Wait(); err != nil {
									b.Fatal(err)
								}
								for _, d := range durations {
									total += d
								}
							}
							b.StopTimer()
							b.ReportMetric(float64(total)/float64(time.Millisecond)/float64(b.N*jobs), "ms/job")
						})
					}
				}
			}
		})
	}
}
