package gcsemu

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
	"gotest.tools/v3/assert"
)

const (
	invalidBucketName = "fullstory-non-existant-bucket"
)

var (
	testCases = []struct {
		name string
		f    func(t *testing.T, bh BucketHandle)
	}{
		{"Basics", testBasics},
		{"MultipleFiles", testMultipleFiles},
		{"HugeFile", testHugeFile},
		{"HugeFile_MultipleOfChunkSize", testHugeFileMultipleOfChunkSize},
		{"HugeFileWithConditional", testHugeFileWithConditional},
		{"ConditionalUpdates", testConditionalUpdates},
		{"GenNotMatchDoesntExist", testGenNotMatchDoesntExist},
		{"CopyBasics", testCopyBasics},
		{"Compose", testCompose},
		{"CopyMetadata", testCopyMetadata},
		{"CopyConditionals", testCopyConditionals},
	}
)

const (
	v1      = `This file is for gcsemu_intg_test.go, please ignore (v1)`
	v2      = `This file is for gcsemu_intg_test.go, please ignore (this is version 2)`
	source1 = `This is source file number 1`
	source2 = `This is source file number 2`
)

type BucketHandle struct {
	Name string
	*storage.BucketHandle
}

func initBucket(t *testing.T, bh BucketHandle) {
	ctx := context.Background()

	_ = bh.Delete(ctx)
	err := bh.Create(ctx, "dev", &storage.BucketAttrs{})
	assert.NilError(t, err, "failed")

	attrs, err := bh.Attrs(ctx)
	assert.NilError(t, err, "failed")
	assert.Equal(t, bh.Name, attrs.Name, "wrong")
}

func testBasics(t *testing.T, bh BucketHandle) {
	const name = "gscemu-test/1.txt"
	ctx := context.Background()
	oh := bh.Object(name)

	// Forcibly delete the object at the start, make sure it doesn't exist.
	err := oh.Delete(ctx)
	if err != nil {
		assert.Equal(t, storage.ErrObjectNotExist, err, "wrong error")
	}

	// Should not exist.
	_, err = oh.Attrs(ctx)
	assert.Equal(t, storage.ErrObjectNotExist, err, "wrong error")

	// Checker funcs
	checkAttrs := func(attrs *storage.ObjectAttrs, content string, metagen int64) {
		assert.Equal(t, name, attrs.Name, "wrong")
		assert.Equal(t, bh.Name, attrs.Bucket, "wrong")
		assert.Equal(t, int64(len(content)), attrs.Size, "wrong")
		assert.Equal(t, metagen, attrs.Metageneration, "wrong")
		checkSum := md5.Sum([]byte(content))
		assert.DeepEqual(t, checkSum[:], attrs.MD5)
	}

	checkObject := func(content string, metagen int64) *storage.ObjectAttrs {
		attrs, err := oh.Attrs(ctx)
		assert.NilError(t, err, "failed")
		checkAttrs(attrs, content, metagen)

		r, err := oh.NewReader(ctx)
		assert.NilError(t, err, "failed")
		data, err := io.ReadAll(r)
		assert.NilError(t, err, "failed")
		assert.NilError(t, r.Close(), "failed")
		assert.Equal(t, content, string(data), "wrong data")
		return attrs
	}

	// Create the object.
	w := oh.NewWriter(ctx)
	assert.NilError(t, write(w, v1), "failed")
	checkAttrs(w.Attrs(), v1, 1)

	// Read the object.
	attrs := checkObject(v1, 1)
	assert.Assert(t, attrs.Generation != 0, "expected non-zero")
	gen := attrs.Generation

	// Update the object to version 2.  Also test MD5 setting.
	w = oh.NewWriter(ctx)
	checkSum := md5.Sum([]byte(v2))
	w.MD5 = checkSum[:]
	assert.NilError(t, write(w, v2), "failed")
	checkAttrs(w.Attrs(), v2, 1)
	assert.Assert(t, gen != w.Attrs().Generation, "expected different gen")
	gen = w.Attrs().Generation

	// Read the object again.
	attrs = checkObject(v2, 1)
	assert.Equal(t, gen, attrs.Generation, "expected same gen")

	// Update the attrs.
	attrs, err = oh.Update(ctx, storage.ObjectAttrsToUpdate{
		ContentType: "text/plain",
	})
	assert.NilError(t, err, "failed")
	checkAttrs(attrs, v2, 2)
	assert.Equal(t, "text/plain", attrs.ContentType, "wrong")
	assert.Equal(t, gen, attrs.Generation, "expected same gen")

	// Delete the object.
	assert.NilError(t, oh.Delete(ctx), "failed")

	// Should not exist.
	_, err = oh.Attrs(ctx)
	assert.Equal(t, storage.ErrObjectNotExist, err, "wrong error")

	// Should not be able to update attrs.
	_, err = oh.Update(ctx, storage.ObjectAttrsToUpdate{
		ContentType: "text/plain",
	})
	assert.Equal(t, storage.ErrObjectNotExist, err, "wrong error")
}

func testMultipleFiles(t *testing.T, bh BucketHandle) {
	dir := "multi-test/"
	ctx := context.Background()

	files := []string{"file1", "file2", "file3"}
	for _, f := range files {
		oh := bh.Object(dir + f)
		w := oh.NewWriter(ctx)
		assert.NilError(t, write(w, v1), "failed to write file %s", dir+f)
	}

	iter := bh.Objects(ctx, &storage.Query{Prefix: dir})
	for _, f := range files {
		obj, err := iter.Next()
		assert.NilError(t, err, "failed to fetch next object")
		assert.Equal(t, dir+f, obj.Name, "wrong filename")
	}

	// No more objects should exist
	_, err := iter.Next()
	assert.Equal(t, iterator.Done, err, "iteration not finished or failed after first bucket object")
}

// Tests resumable GCS uploads.
func testHugeFile(t *testing.T, bh BucketHandle) {
	doHugeFile(t, bh, "gscemu-test/huge.txt", googleapi.DefaultUploadChunkSize+4*1024*1024)
}

func testHugeFileMultipleOfChunkSize(t *testing.T, bh BucketHandle) {
	doHugeFile(t, bh, "gscemu-test/huge2.txt", googleapi.DefaultUploadChunkSize*4)
}

func doHugeFile(t *testing.T, bh BucketHandle, name string, size int) {
	ctx := context.Background()
	oh := bh.Object(name)

	// Forcibly delete the object at the start, make sure it doesn't exist.
	err := oh.Delete(ctx)
	if err != nil {
		assert.Equal(t, storage.ErrObjectNotExist, err, "wrong error")
	}

	// Should not exist.
	_, err = oh.Attrs(ctx)
	assert.Equal(t, storage.ErrObjectNotExist, err, "wrong error")

	// Create the object.
	w := oh.NewWriter(ctx)
	hash, err := writeHugeObject(t, w, size)
	assert.NilError(t, err, "failed")

	attrs, err := oh.Attrs(ctx)
	assert.NilError(t, err, "failed")
	assert.Equal(t, size, int(attrs.Size), "wrong")
	assert.DeepEqual(t, hash, attrs.MD5)
}

func writeHugeObject(t *testing.T, w *storage.Writer, sz int) ([]byte, error) {
	data := []byte(`0123456789ABCDEF`)
	hash := md5.New()
	for i := 0; i < sz/len(data); i++ {
		n, err := w.Write(data)
		_, _ = hash.Write(data)
		assert.NilError(t, err, "failed")
		assert.Equal(t, n, len(data), "short write")
	}
	return hash.Sum(nil), w.Close()
}

// Tests resumable GCS uploads.
func testHugeFileWithConditional(t *testing.T, bh BucketHandle) {
	const name = "gscemu-test/huge2.txt"
	const size = googleapi.DefaultUploadChunkSize*2 + 1024

	ctx := context.Background()
	oh := bh.Object(name)

	// Forcibly delete the object at the start, make sure it doesn't exist.
	err := oh.Delete(ctx)
	if err != nil {
		assert.Equal(t, storage.ErrObjectNotExist, err, "wrong error")
	}

	// Should not exist.
	_, err = oh.Attrs(ctx)
	assert.Equal(t, storage.ErrObjectNotExist, err, "wrong error")

	// Create the object.
	w := oh.If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)
	hash, err := writeHugeObject(t, w, size)
	assert.NilError(t, err, "failed")

	attrs, err := oh.Attrs(ctx)
	assert.NilError(t, err, "failed")
	assert.Equal(t, size, int(attrs.Size), "wrong")
	assert.DeepEqual(t, hash, attrs.MD5)

	// Should fail this time.
	w = oh.If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)
	_, err = writeHugeObject(t, w, size)
	assert.Equal(t, http.StatusPreconditionFailed, httpStatusCodeOf(err), "wrong error %T: %s", err, err)
}

func testConditionalUpdates(t *testing.T, bh BucketHandle) {
	const name = "gscemu-test/2.txt"
	ctx := context.Background()
	oh := bh.Object(name)

	// Forcibly delete the object at the start, make sure it doesn't exist.
	err := oh.Delete(ctx)
	if err != nil {
		assert.Equal(t, storage.ErrObjectNotExist, err, "wrong error")
	}

	// Ensure write fails
	w := oh.If(storage.Conditions{GenerationMatch: 1}).NewWriter(ctx)
	err = write(w, "bogus")
	assert.Equal(t, http.StatusPreconditionFailed, httpStatusCodeOf(err), "wrong error %T: %s", err, err)

	// Now actually write it.
	w = oh.If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)
	assert.NilError(t, write(w, v1), "failed")
	attrs := w.Attrs()
	t.Logf("attrs.Generation=%d attrs.Metageneration=%d", attrs.Generation, attrs.Metageneration)

	expectFailConds := func(expectCode int, conds storage.Conditions) {
		// Ensure attr update fails.
		_, err = oh.If(conds).Update(ctx, storage.ObjectAttrsToUpdate{ContentType: "text/plain"})
		assert.Equal(t, expectCode, httpStatusCodeOf(err), "wrong error %T: %s", err, err)

		// Ensure write fails
		w := oh.If(conds).NewWriter(ctx)
		err = write(w, "bogus")
		assert.Equal(t, expectCode, httpStatusCodeOf(err), "wrong error %T: %s", err, err)

		// Ensure delete fails
		err = oh.If(conds).Delete(ctx)
		assert.Equal(t, expectCode, httpStatusCodeOf(err), "wrong error %T: %s", err, err)
	}

	for i, conds := range []storage.Conditions{
		{
			DoesNotExist: true,
		},
		{
			GenerationMatch: attrs.Generation + 1,
		},
		{
			MetagenerationMatch: attrs.Metageneration + 1,
		},
		{
			GenerationMatch:     attrs.Generation,
			MetagenerationMatch: attrs.Metageneration + 1,
		},
		{
			GenerationMatch:     attrs.Generation + 1,
			MetagenerationMatch: attrs.Metageneration,
		},
		{
			GenerationNotMatch: attrs.Generation,
		},
		{
			MetagenerationNotMatch: attrs.Metageneration,
		},
		{
			GenerationNotMatch:  attrs.Generation,
			MetagenerationMatch: attrs.Metageneration,
		},
		{
			GenerationMatch:        attrs.Generation,
			MetagenerationNotMatch: attrs.Metageneration,
		},
	} {
		t.Logf("case %d", i)
		expectCode := http.StatusPreconditionFailed
		if i >= 5 {
			// For some reason, "not match" cases return 304 rather than 412.
			expectCode = http.StatusNotModified
		}
		expectFailConds(expectCode, conds)
	}

	// Actually update the attrs.
	attrs, err = oh.If(storage.Conditions{
		GenerationMatch:     attrs.Generation,
		MetagenerationMatch: attrs.Metageneration,
	}).Update(ctx, storage.ObjectAttrsToUpdate{
		ContentType: "text/plain",
	})
	assert.NilError(t, err, "failed")

	// Actually update the content.
	w = oh.If(storage.Conditions{
		GenerationMatch:     attrs.Generation,
		MetagenerationMatch: attrs.Metageneration,
	}).NewWriter(ctx)
	assert.NilError(t, write(w, v2), "failed")
	attrs = w.Attrs()

	// Actually delete.
	err = oh.If(storage.Conditions{
		GenerationMatch:     attrs.Generation,
		MetagenerationMatch: attrs.Metageneration,
	}).Delete(ctx)
	assert.NilError(t, err, "failed")

	// Should not exist.
	_, err = oh.Attrs(ctx)
	assert.Equal(t, storage.ErrObjectNotExist, err, "wrong error")
}

func testGenNotMatchDoesntExist(t *testing.T, bh BucketHandle) {
	// How does generation not match interact with a non-existent file?

	const name = "gscemu-test-gen-not-match.txt"
	ctx := context.Background()
	oh := bh.Object(name)

	// Forcibly delete the object at the start, make sure it doesn't exist.
	err := oh.Delete(ctx)
	if err != nil {
		assert.Equal(t, storage.ErrObjectNotExist, err, "wrong error")
	}

	// Write should fail on non-existent object, even though the generation doesn't match.
	w := oh.If(storage.Conditions{GenerationNotMatch: 1}).NewWriter(ctx)
	err = write(w, "bogus")
	assert.Equal(t, http.StatusPreconditionFailed, httpStatusCodeOf(err), "wrong error %T: %s", err, err)
}

func testCopyBasics(t *testing.T, bh BucketHandle) {
	ctx := context.Background()

	file1 := "file-1"
	file2 := "file-1-again"

	src := bh.Object(file1)
	dest := bh.Object(file2)

	// Forcibly delete the object at the start, make sure it doesn't exist.
	_ = src.Delete(ctx)
	_ = dest.Delete(ctx)

	// Should not exist.
	_, err := src.Attrs(ctx)
	assert.Equal(t, storage.ErrObjectNotExist, err, "wrong error")

	// Create the object.
	w := src.NewWriter(ctx)
	n, err := io.Copy(w, strings.NewReader(v1))
	assert.NilError(t, err, "failed")
	assert.Equal(t, n, int64(len(v1)), "wrong length")
	assert.NilError(t, w.Close(), "failed")

	// Wait a ms to ensure different timestamps.
	time.Sleep(time.Millisecond)

	// Copy the object
	destAttrs, err := dest.CopierFrom(src).Run(ctx)
	assert.NilError(t, err, "failed to copy")

	// Read the object.
	r, err := dest.NewReader(ctx)
	assert.NilError(t, err, "failed")
	data, err := io.ReadAll(r)
	assert.NilError(t, err, "failed")
	assert.NilError(t, r.Close(), "failed")
	assert.Equal(t, string(data), v1, "wrong data")

	// Check the metadata reread correct
	reDestAttrs, err := dest.Attrs(ctx)
	assert.NilError(t, err, "failed")
	assert.DeepEqual(t, destAttrs, reDestAttrs)

	// Check the metadata was copied and makes sense.
	srcAttrs, err := src.Attrs(ctx)
	assert.NilError(t, err, "failed")
	expectAttrs := *srcAttrs

	// Some things should be different
	assert.Assert(t, srcAttrs.Name != destAttrs.Name, "should not equal: %s", destAttrs.Name)
	expectAttrs.Name = destAttrs.Name

	assert.Assert(t, srcAttrs.MediaLink != destAttrs.MediaLink, "should not equal: %s", destAttrs.MediaLink)
	expectAttrs.MediaLink = destAttrs.MediaLink

	assert.Assert(t, srcAttrs.Generation != destAttrs.Generation, "should not equal: %d", destAttrs.Generation)
	expectAttrs.Generation = destAttrs.Generation

	assert.Assert(t, srcAttrs.Created != destAttrs.Created, "should not equal: %s", destAttrs.Created)
	expectAttrs.Created = destAttrs.Created

	assert.Assert(t, srcAttrs.Updated != destAttrs.Updated, "should not equal: %s", destAttrs.Updated)
	expectAttrs.Updated = destAttrs.Updated

	expectAttrs.Etag = destAttrs.Etag

	// Rest should be same
	assert.DeepEqual(t, expectAttrs, *destAttrs)

	// Delete the object.
	assert.NilError(t, src.Delete(ctx), "failed")
	assert.NilError(t, dest.Delete(ctx), "failed")

	// Copy an object that doesn't exist
	_, err = dest.CopierFrom(src).Run(ctx)
	assert.Equal(t, http.StatusNotFound, httpStatusCodeOf(err), "wrong error %T: %s", err, err)
}

func testCompose(t *testing.T, bh BucketHandle) {
	ctx := context.Background()

	srcFiles := []string{source1, source2}
	srcGens := []int64{0, 0}
	manualCompose := ""
	dstName := "gcs-test-data/dest.txt"
	dstNameSecondary := "gcs-test-data/dest-secondary.txt"

	srcs := make([]*storage.ObjectHandle, len(srcFiles))
	for i, src := range srcFiles {
		name := fmt.Sprintf("gcs-test-sources/src-%d.txt", i)
		srcs[i] = bh.Object(name)
		// Forcibly delete the object at the start, make sure it doesn't exist.
		_ = srcs[i].Delete(ctx)

		// Should not exist.
		_, err := srcs[i].Attrs(ctx)
		assert.Equal(t, storage.ErrObjectNotExist, err, "wrong error")

		// Create the object.
		w := srcs[i].NewWriter(ctx)
		w.ContentType = "text/csv"
		n, err := io.Copy(w, strings.NewReader(src))
		assert.NilError(t, err, "failed")
		assert.Equal(t, n, int64(len(src)), "wrong length")
		assert.NilError(t, w.Close(), "failed")
		srcGens[i] = w.Attrs().Generation

		manualCompose += src
	}

	dest := bh.Object(dstName)

	destSecondary := bh.Object(dstNameSecondary)
	err := destSecondary.Delete(ctx) // this one needs to be deleted to start
	assert.Assert(t, err == nil || err == storage.ErrObjectNotExist, "failed to delete secondary")

	composer := dest.ComposerFrom(srcs...)
	composer.ContentType = "text/plain"
	attrs, err := composer.Run(ctx)
	assert.NilError(t, err, "failed to run compose")

	assert.Equal(t, dest.BucketName(), attrs.Bucket, "bucket doesn't match")
	assert.Equal(t, dest.ObjectName(), attrs.Name, "object name doesn't match")
	assert.Equal(t, "text/plain", attrs.ContentType, "content type doesn't match")
	r, err := dest.NewReader(ctx)
	assert.NilError(t, err, "failed to create reader for composed file")
	data, err := io.ReadAll(r)
	assert.NilError(t, err, "failed to read from composed file")
	assert.NilError(t, r.Close(), "failed to close composed file reader")
	assert.Equal(t, manualCompose, string(data), "content doesn't match")

	// Issue the same request with incorrect generation on a source.
	composer = dest.If(storage.Conditions{GenerationMatch: attrs.Generation}).ComposerFrom(
		srcs[0].If(storage.Conditions{GenerationMatch: srcGens[0]}),     // correct
		srcs[1].If(storage.Conditions{GenerationMatch: srcGens[1] + 1})) // incorrect
	composer.ContentType = "text/plain"
	_, err = composer.Run(ctx)
	assert.ErrorContains(t, err, "googleapi: Error 412")
	assert.Equal(t, http.StatusPreconditionFailed, httpStatusCodeOf(err), "expected precondition failed")

	// Issue the same request with incorrect generation on the destination.
	composer = dest.If(storage.Conditions{DoesNotExist: true}).ComposerFrom(
		srcs[0].If(storage.Conditions{GenerationMatch: srcGens[0]}),
		srcs[1].If(storage.Conditions{GenerationMatch: srcGens[1]}))
	composer.ContentType = "text/plain"
	_, err = composer.Run(ctx)
	assert.ErrorContains(t, err, "googleapi: Error 412")
	assert.Equal(t, http.StatusPreconditionFailed, httpStatusCodeOf(err), "expected precondition failed")

	// Issue the a request does not exist destination.
	composer = destSecondary.If(storage.Conditions{DoesNotExist: true}).ComposerFrom(
		srcs[0].If(storage.Conditions{GenerationMatch: srcGens[0]}),
		srcs[1].If(storage.Conditions{GenerationMatch: srcGens[1]}))
	composer.ContentType = "text/plain"
	_, err = composer.Run(ctx)
	assert.NilError(t, err, "failed to run compose")
	// The resulting data should be correct (like in the original test).
	r, err = destSecondary.NewReader(ctx)
	assert.NilError(t, err, "failed to create reader for composed file")
	data, err = io.ReadAll(r)
	assert.NilError(t, err, "failed to read from composed file")
	assert.NilError(t, r.Close(), "failed to close composed file reader")
	assert.Equal(t, manualCompose, string(data), "content doesn't match")

	// Use the new destination as the source for another compose. This is how we append
	// Additionally, use generation conditions for all of the source objects.
	composer = dest.ComposerFrom(
		dest.If(storage.Conditions{GenerationMatch: attrs.Generation}),
		srcs[0].If(storage.Conditions{GenerationMatch: srcGens[0]}))
	newAttrs, err := composer.Run(ctx)
	assert.NilError(t, err, "failed to run compose")
	assert.Equal(t, "", newAttrs.ContentType, "content type doesn't match")

	r, err = dest.NewReader(ctx)
	assert.NilError(t, err, "failed to create reader for composed file")
	data, err = io.ReadAll(r)
	assert.NilError(t, err, "failed to read from composed file")
	assert.NilError(t, r.Close(), "failed to close composed file reader")
	assert.Equal(t, manualCompose+source1, string(data), "content doesn't match")

	// Make sure we get a 404 if the source doesn't exist
	dneObj := bh.Object("dneObject")
	_ = dneObj.Delete(ctx)
	// Should not exist.
	_, err = dneObj.Attrs(ctx)
	assert.Equal(t, storage.ErrObjectNotExist, err, "wrong error")

	composer = dest.ComposerFrom(dneObj)
	_, err = composer.Run(ctx)
	assert.Equal(t, http.StatusNotFound, httpStatusCodeOf(err), "wrong error returned")
}

func testCopyMetadata(t *testing.T, bh BucketHandle) {
	// TODO(dk): Metadata-rewriting on copy is not currently implemented.
	t.Skip()
}

func testCopyConditionals(t *testing.T, bh BucketHandle) {
	// TODO(dk): Conditional support for copy is not currently implemented.
	t.Skip()
}

func write(w *storage.Writer, content string) error {
	n, err := io.Copy(w, strings.NewReader(content))
	if err != nil {
		return err
	}
	if n != int64(len(content)) {
		panic("not all content sent")
	}
	return w.Close()
}
