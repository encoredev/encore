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

// BenchmarkImageFileReadAhead compares file-level I/O concurrency at a fixed
// 256 KiB per-layer budget, gzip 5 and two metadata/compression workers. Each
// operation is one wave of jobs sharing GOMAXPROCS and warm on-disk inputs.
func BenchmarkImageFileReadAhead(b *testing.B) {
	logger := log.Logger
	log.Logger = log.Output(io.Discard)
	defer func() { log.Logger = logger }()
	for _, layout := range []string{"balanced", "dependencies"} {
		b.Run(layout, func(b *testing.B) {
			spec, size := packagingFixture(b, layout)
			for _, readers := range []int{1, 2, 4} {
				for _, jobs := range []int{1, 4} {
					b.Run(fmt.Sprintf("files%d/jobs%d", readers, jobs), func(b *testing.B) {
						cfg := ImageBuildConfig{PreparationConcurrency: 2, ReadAheadBytes: 256 << 10, ReadAheadConcurrency: readers, CompressionLevel: 5, CompressionConcurrency: 2, TempDir: b.TempDir()}
						durations := make([]time.Duration, jobs)
						var total time.Duration
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
		})
	}
}
