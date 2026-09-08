package dockerbuild

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/cockroachdb/errors"
	"golang.org/x/sync/semaphore"

	"encr.dev/pkg/xos"
)

// Across concurrent builds in one process, metadata syscalls share a CPU-sized
// budget (at most four). Per-image worker limits alone overdrive the filesystem
// when several images are assembled together. These slots cover only preparation,
// never compression or a wait for another node.
var tarPreparationSlots = semaphore.NewWeighted(int64(min(4, runtime.GOMAXPROCS(0))))

// A node's children retain os.ReadDir's lexical order. Workers only fill their
// own nodes; the copier's mutable state is updated later in depth-first order.
type tarNode struct {
	path       HostPath
	dirEntry   fs.DirEntry
	entry      *tarEntry
	dst        *tarCopier
	depsParent bool
	children   []*tarNode
	err        error
}

func (tc *tarCopier) copyDir(ctx context.Context, desc *dirCopyDesc, workers int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	fi, err := os.Lstat(desc.SrcPath.String())
	if err != nil {
		return errors.WithStack(err)
	}
	root := &tarNode{path: desc.SrcPath, dirEntry: fs.FileInfoToDirEntry(fi)}
	if err := prepareTarTree(ctx, root, workers, func(node *tarNode) {
		if err := tarPreparationSlots.Acquire(ctx, 1); err != nil {
			node.err = err
			return
		}
		defer tarPreparationSlots.Release(1)
		node.err = tc.prepareNode(desc, node)
	}); err != nil {
		return err
	}
	var merge func(*tarNode) error
	merge = func(node *tarNode) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if node.err != nil {
			return node.err
		}
		if node.depsParent {
			rel, err := desc.SrcPath.Rel(node.path)
			if err != nil {
				return err
			}
			if err := node.dst.MkdirAll(desc.DstPath.Join(string(rel.ToImage())).Dir(), 0755); err != nil {
				return err
			}
		}
		if node.entry != nil {
			node.dst.appendEntry(node.entry)
		}
		for _, child := range node.children {
			if err := merge(child); err != nil {
				return err
			}
		}
		return nil
	}
	return errors.WithStack(merge(root))
}

// One coordinator dispatches work; workers never enqueue children themselves,
// so a broad/deep tree cannot deadlock a bounded work channel. There are only
// workers goroutines, regardless of tree size. Cancellation joins all of them.
func prepareTarTree(ctx context.Context, root *tarNode, workers int, prepare func(*tarNode)) error {
	ctx, cancel := context.WithCancel(ctx)
	jobs := make(chan *tarNode)
	done := make(chan *tarNode)
	var wg sync.WaitGroup
	defer func() { cancel(); close(jobs); wg.Wait() }()
	for range workers {
		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case node, ok := <-jobs:
					if !ok {
						return
					}
					prepare(node)
					select {
					case done <- node:
					case <-ctx.Done():
						return
					}
				}
			}
		})
	}
	pending := []*tarNode{root}
	active := 0
	for len(pending) > 0 || active > 0 {
		var out chan *tarNode
		var next *tarNode
		if len(pending) > 0 {
			out, next = jobs, pending[len(pending)-1]
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- next:
			pending[len(pending)-1] = nil
			pending = pending[:len(pending)-1]
			active++
		case node := <-done:
			active--
			if node.err == nil {
				pending = append(pending, node.children...)
			}
		}
	}
	return ctx.Err()
}

func (tc *tarCopier) prepareNode(desc *dirCopyDesc, node *tarNode) error {
	path, d := node.path, node.dirEntry
	if !shouldInclude(desc, path) || desc.ExcludeSrcPaths[path] {
		return nil
	}
	rel, err := desc.SrcPath.Rel(path)
	if err != nil {
		return errors.WithStack(err)
	}
	if !d.IsDir() && isVolatilePnpmMetadata(rel) {
		return nil
	}
	node.dst = tc
	if desc.DepsCopier != nil {
		if within, root := nodeModulesPath(rel); within {
			node.dst, node.depsParent = desc.DepsCopier, root
		}
	}
	dstPath := desc.DstPath.Join(string(rel.ToImage()))
	var link ImagePath
	isSymlink := d.Type()&fs.ModeSymlink != 0
	if !isSymlink && runtime.GOOS == "windows" {
		if junction, _ := xos.IsWindowsJunctionPoint(path.String()); junction {
			return errors.Newf("%q is a windows junction point and cannot be copied to a docker image. Use symlinks instead.", path)
		}
	}
	if isSymlink {
		target, err := os.Readlink(path.String())
		if err != nil {
			return errors.WithStack(err)
		}
		link, err = tc.rewriteSymlink(desc, path, HostPath(target))
		if err != nil {
			return errors.WithStack(err)
		}
		if link == "" {
			return nil
		}
	}
	fi, err := d.Info()
	if err != nil {
		return errors.WithStack(err)
	}
	node.entry, err = node.dst.prepareFile(dstPath, path, fi, link)
	if err != nil {
		return errors.Wrap(err, "add file")
	}
	if d.IsDir() {
		children, err := os.ReadDir(path.String())
		if err != nil {
			return errors.WithStack(err)
		}
		for _, child := range children {
			node.children = append(node.children, &tarNode{path: HostPath(filepath.Join(path.String(), child.Name())), dirEntry: child})
		}
	}
	return nil
}
