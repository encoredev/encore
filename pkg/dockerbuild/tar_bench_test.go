package dockerbuild

import (
	"fmt"
	"io"
	"testing"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"encr.dev/pkg/fns"
)

// BenchmarkImageTar separates directory/header preparation from streaming
// tar bytes, so compression is not mistaken for tar work. It uses the same files
// as BenchmarkImagePackaging and deliberately does no hashing or gzip work.
func BenchmarkImageTar(b *testing.B) {
	logger := log.Logger
	log.Logger = log.Output(io.Discard)
	defer func() { log.Logger = logger }()
	for _, layout := range []string{"balanced", "dependencies"} {
		b.Run(layout, func(b *testing.B) {
			spec, size := packagingFixture(b, layout)

			for _, workers := range []int{1, 2, 4} {
				b.Run(fmt.Sprintf("prepare/workers%d", workers), func(b *testing.B) {
					b.ReportAllocs()
					for range b.N {
						if _, err := buildImageFilesystem(b.Context(), spec, &ImageBuildConfig{PreparationConcurrency: workers, ReadAheadBytes: -1}); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
			for _, ahead := range []int{-1, 256 << 10} {
				layers, err := buildImageFilesystem(b.Context(), spec, &ImageBuildConfig{ReadAheadBytes: ahead})
				if err != nil {
					b.Fatal(err)
				}
				for _, workers := range []int{1, 2, 4} {
					b.Run(fmt.Sprintf("read/ahead%d/workers%d", max(0, ahead>>10), workers), func(b *testing.B) {
						b.SetBytes(size)
						b.ReportAllocs()
						for range b.N {
							var g errgroup.Group
							g.SetLimit(workers)
							for _, layer := range layers {
								g.Go(func() error {
									r, err := layer.opener()
									if err != nil {
										return err
									}
									defer fns.CloseIgnore(r)
									// Match the compressor's 32 KiB copy buffer, disabling
									// io.Discard's differently sized ReadFrom fast path.
									_, err = io.CopyBuffer(struct{ io.Writer }{io.Discard}, r, make([]byte, 32<<10))
									return err
								})
							}
							if err := g.Wait(); err != nil {
								b.Fatal(err)
							}
						}
					})
				}
			}
		})
	}
}
