package dockerbuild

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/cockroachdb/errors"
)

func TestTarPreparationBoundsWorkers(t *testing.T) {
	for _, workers := range []int{1, 2, 4} {
		t.Run(fmt.Sprint(workers), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				root := &tarNode{}
				for range 100 {
					root.children = append(root.children, &tarNode{})
				}
				release := make(chan struct{})
				finished := make(chan error, 1)
				var active, entered atomic.Int64
				go func() {
					finished <- prepareTarTree(t.Context(), root, workers, func(node *tarNode) {
						if node == root {
							return
						}
						active.Add(1)
						entered.Add(1)
						<-release
						active.Add(-1)
					})
				}()
				synctest.Wait()
				if n := active.Load(); n != int64(workers) {
					t.Fatalf("active workers %d, want %d", n, workers)
				}
				if entered.Load() != int64(workers) {
					t.Fatal("started work beyond the worker limit")
				}
				close(release)
				if err := <-finished; err != nil {
					t.Fatal(err)
				}
				if active.Load() != 0 || entered.Load() != 100 {
					t.Fatal("workers did not join or skipped files")
				}
			})
		})
	}
}

func TestTarPreparationCancellationJoinsWorkers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		root := &tarNode{}
		for range 100 {
			root.children = append(root.children, &tarNode{})
		}
		finished := make(chan error, 1)
		var active atomic.Int64
		go func() {
			finished <- prepareTarTree(ctx, root, 4, func(node *tarNode) {
				if node == root {
					return
				}
				active.Add(1)
				<-ctx.Done()
				active.Add(-1)
			})
		}()
		synctest.Wait()
		if active.Load() != 4 {
			t.Fatal("workers did not start")
		}
		cancel()
		if err := <-finished; !errors.Is(err, context.Canceled) {
			t.Fatalf("result = %v", err)
		}
		if active.Load() != 0 {
			t.Fatal("returned without joining workers")
		}
	})
}
