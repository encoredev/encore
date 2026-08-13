package gcsemu

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"google.golang.org/api/storage/v1"

	"encr.dev/pkg/emulators/storage/gcsutil"
)

// TestListObjectsEmptyFolders pins that a created folder behaves exactly like the
// zero-byte "<folder>/" object Cloud Storage represents it as: rolled up into a prefix
// from outside, returned as an item from inside, and reported by no other means.
func TestListObjectsEmptyFolders(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(t.TempDir())
	emu := NewGcsEmu(Options{Store: store})

	for _, key := range []string{"a/1.txt", "c/2.txt"} {
		if err := store.Add("bkt", key, []byte("x"), &storage.Object{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Add("bkt", "b/", nil, &storage.Object{}); err != nil {
		t.Fatal(err)
	}

	// The empty folder sits alongside the folders that do hold objects, and only once.
	objs, err := emu.ListObjects(ctx, "", "bkt", ListOptions{Delimiter: "/"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a/", "b/", "c/"}; !reflect.DeepEqual(objs.Prefixes, want) {
		t.Errorf("prefixes = %v, want %v", objs.Prefixes, want)
	}
	if len(objs.Items) != 0 {
		t.Errorf("items = %v, want none: everything is below a folder", objs.Items)
	}

	// Listing the empty folder returns its placeholder, as Cloud Storage does.
	objs, err = emu.ListObjects(ctx, "", "bkt", ListOptions{Prefix: "b/", Delimiter: "/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(objs.Prefixes) != 0 {
		t.Errorf("prefixes = %v, want none", objs.Prefixes)
	}
	if len(objs.Items) != 1 || objs.Items[0].Name != "b/" {
		t.Errorf("items = %v, want just the b/ placeholder", names(objs))
	} else if objs.Items[0].Size != 0 {
		t.Errorf("placeholder size = %d, want 0", objs.Items[0].Size)
	}

	// A folder nobody created has no placeholder, only the objects below it.
	objs, err = emu.ListObjects(ctx, "", "bkt", ListOptions{Prefix: "a/", Delimiter: "/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(objs.Prefixes) != 0 {
		t.Errorf("prefixes = %v, want none", objs.Prefixes)
	}
	if want := []string{"a/1.txt"}; !reflect.DeepEqual(names(objs), want) {
		t.Errorf("items = %v, want %v", names(objs), want)
	}
}

// TestFolderPlaceholderIsAnObject pins that the placeholder is reachable by name like
// any other object, which is what lets the browser delete an empty folder.
func TestFolderPlaceholderIsAnObject(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(t.TempDir())
	emu := NewGcsEmu(Options{Store: store})

	if err := store.Add("bkt", "b/", nil, &storage.Object{}); err != nil {
		t.Fatal(err)
	}

	obj, err := store.GetMeta(dontNeedUrls, "bkt", "b/")
	if err != nil {
		t.Fatal(err)
	}
	if obj == nil || obj.Name != "b/" {
		t.Fatalf("GetMeta(b/) = %v, want the placeholder", obj)
	}

	// A glob covering the folder matches it, since it is just a key.
	objs, err := emu.ListObjects(ctx, "", "bkt", ListOptions{MatchGlob: "**b**"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"b/"}; !reflect.DeepEqual(names(objs), want) {
		t.Errorf("items = %v, want %v", names(objs), want)
	}

	// Deleting it by name removes the folder outright.
	if err := store.Delete("bkt", "b/"); err != nil {
		t.Fatal(err)
	}
	objs, err = emu.ListObjects(ctx, "", "bkt", ListOptions{Delimiter: "/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(objs.Prefixes) != 0 || len(objs.Items) != 0 {
		t.Errorf("listing = %v/%v, want empty", objs.Prefixes, names(objs))
	}
}

// TestFolderPlaceholderSurvivesSiblings pins that emptying a folder leaves it in place,
// which is what distinguishes a created folder from an implicit one.
func TestFolderPlaceholderSurvivesSiblings(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(t.TempDir())
	emu := NewGcsEmu(Options{Store: store})

	if err := store.Add("bkt", "b/", nil, &storage.Object{}); err != nil {
		t.Fatal(err)
	}
	if err := store.Add("bkt", "b/1.txt", []byte("x"), &storage.Object{}); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("bkt", "b/1.txt"); err != nil {
		t.Fatal(err)
	}

	objs, err := emu.ListObjects(ctx, "", "bkt", ListOptions{Delimiter: "/"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"b/"}; !reflect.DeepEqual(objs.Prefixes, want) {
		t.Errorf("prefixes = %v, want %v: the folder was created, so it outlives its contents", objs.Prefixes, want)
	}
}

func names(objs *storage.Objects) []string {
	var out []string
	for _, item := range objs.Items {
		out = append(out, item.Name)
	}
	return out
}

func TestListObjectsMatchGlob(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(t.TempDir())
	emu := NewGcsEmu(Options{Store: store})

	for _, key := range []string{"a.txt", "img/a.png", "img/b.png", "img/deep/c.png"} {
		if err := store.Add("bkt", key, []byte("x"), &storage.Object{}); err != nil {
			t.Fatal(err)
		}
	}

	objs, err := emu.ListObjects(ctx, "", "bkt", ListOptions{MatchGlob: "**.png"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"img/a.png", "img/b.png", "img/deep/c.png"}; !reflect.DeepEqual(names(objs), want) {
		t.Errorf("items = %v, want %v", names(objs), want)
	}

	objs, err = emu.ListObjects(ctx, "", "bkt", ListOptions{MatchGlob: "img/*.png"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"img/a.png", "img/b.png"}; !reflect.DeepEqual(names(objs), want) {
		t.Errorf("items = %v, want %v", names(objs), want)
	}

	// Non-matching objects must not consume the page budget, or a sparse match
	// would come back as a run of empty pages.
	objs, err = emu.ListObjects(ctx, "", "bkt", ListOptions{MatchGlob: "**deep**", MaxResults: 1})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"img/deep/c.png"}; !reflect.DeepEqual(names(objs), want) {
		t.Errorf("items = %v, want %v", names(objs), want)
	}

	if _, err := emu.ListObjects(ctx, "", "bkt", ListOptions{MatchGlob: "["}); err == nil {
		t.Error("expected an error for a malformed glob")
	}
}

// TestStoreRejectsBucketTraversal pins that a bucket name can't reach outside gcsDir.
// Listing and searching go through Walk, which resolves the bucket to a directory
// without any object key being involved, so the object-key checks don't cover it.
func TestStoreRejectsBucketTraversal(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := NewFileStore(filepath.Join(dir, "gcs"))

	secret := filepath.Join(dir, "secrets", "creds.txt")
	if err := os.MkdirAll(filepath.Dir(secret), 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secret, []byte("shh"), 0666); err != nil {
		t.Fatal(err)
	}

	walked := false
	err := store.Walk(ctx, "../secrets", func(ctx context.Context, filename string, fInfo os.FileInfo) error {
		walked = true
		return nil
	})
	if err == nil {
		t.Error("Walk(../secrets) = nil, want an error")
	}
	if walked {
		t.Error("Walk visited files outside the storage directory")
	}

	if err := store.CreateBucket("../secrets"); err == nil {
		t.Error("CreateBucket(../secrets) = nil, want an error")
	}
}

// TestListObjectsIsLexical pins that a filestore listing arrives in object-name order.
//
// The page token a client carries is just the last name it saw, and the next request
// skips everything <= it, so any name delivered out of order is dropped from the
// listing entirely. The order is easy to get wrong because it isn't the order the
// files sit in on disk: "a.txt" sorts before the objects inside the directory "a",
// since '.' < '/', but the directory comes first in a plain filesystem walk.
func TestListObjectsIsLexical(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(t.TempDir())
	emu := NewGcsEmu(Options{Store: store})

	keys := []string{"a.txt", "a/z.txt", "a-b.txt", "a!c.txt", "zz.txt", "zz/deep/x.txt"}
	for _, key := range keys {
		if err := store.Add("bkt", key, []byte("x"), &storage.Object{}); err != nil {
			t.Fatal(err)
		}
	}
	// A folder placeholder names the directory holding it, so it has to precede
	// everything below it — including a subdirectory sorting before the marker file.
	if err := store.Add("bkt", "a/", nil, &storage.Object{}); err != nil {
		t.Fatal(err)
	}
	if err := store.Add("bkt", "a/.aa/deep.txt", []byte("x"), &storage.Object{}); err != nil {
		t.Fatal(err)
	}

	objs, err := emu.ListObjects(ctx, "", "bkt", ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"a!c.txt", "a-b.txt", "a.txt", "a/", "a/.aa/deep.txt", "a/z.txt", "zz.txt", "zz/deep/x.txt",
	}
	if got := names(objs); !reflect.DeepEqual(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}
}

// TestListObjectsPagesEveryObject pins that paging never drops an object, whatever the
// page size. See TestListObjectsIsLexical for why the ordering is what puts this at risk.
func TestListObjectsPagesEveryObject(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(t.TempDir())
	emu := NewGcsEmu(Options{Store: store})

	want := []string{"a!c.txt", "a-b.txt", "a.txt", "a/z.txt", "b.txt", "b/x/y.txt"}
	for _, key := range want {
		if err := store.Add("bkt", key, []byte("x"), &storage.Object{}); err != nil {
			t.Fatal(err)
		}
	}

	for _, maxResults := range []int{1, 2, 3, 5, 100} {
		var got []string
		opts := ListOptions{MaxResults: maxResults}
		for i := 0; ; i++ {
			if i > 20 {
				t.Fatalf("maxResults=%d: pagination did not terminate", maxResults)
			}
			objs, err := emu.ListObjects(ctx, "", "bkt", opts)
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, names(objs)...)
			if objs.NextPageToken == "" {
				break
			}
			cursor, err := gcsutil.DecodePageToken(objs.NextPageToken)
			if err != nil {
				t.Fatal(err)
			}
			opts.Cursor = cursor
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("maxResults=%d: paged through %v, want %v", maxResults, got, want)
		}
	}
}

// TestListObjectsPagesEveryPrefixOnce pins how a delimited listing spends a page: on
// results the caller hasn't seen. A sub-path counts once however many objects it stands
// for — Cloud Storage's maxResults is the "maximum combined number of entries in items[]
// and prefixes[]" — so a folder holding more objects than fit in a page is still one
// prefix on one page, rather than a run of pages repeating the same sub-path.
func TestListObjectsPagesEveryPrefixOnce(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(t.TempDir())
	emu := NewGcsEmu(Options{Store: store})

	var wantPrefixes []string
	for folder := range 5 {
		wantPrefixes = append(wantPrefixes, fmt.Sprintf("f%d/", folder))
		for i := range 50 {
			key := fmt.Sprintf("f%d/%03d.txt", folder, i)
			if err := store.Add("bkt", key, []byte("x"), &storage.Object{}); err != nil {
				t.Fatal(err)
			}
		}
	}
	wantItems := []string{"g.txt", "h.txt"} // objects of the root's own, sorting last
	for _, key := range wantItems {
		if err := store.Add("bkt", key, []byte("x"), &storage.Object{}); err != nil {
			t.Fatal(err)
		}
	}

	for _, maxResults := range []int{1, 2, 3, 7, 100} {
		var gotPrefixes, gotItems []string
		pages := 0
		opts := ListOptions{Delimiter: "/", MaxResults: maxResults}
		for {
			if pages > 20 {
				t.Fatalf("maxResults=%d: pagination did not terminate", maxResults)
			}
			objs, err := emu.ListObjects(ctx, "", "bkt", opts)
			if err != nil {
				t.Fatal(err)
			}
			pages++
			if got := len(objs.Items) + len(objs.Prefixes); got > maxResults {
				t.Errorf("maxResults=%d: page held %d results", maxResults, got)
			} else if got == 0 {
				t.Errorf("maxResults=%d: page %d held no results at all", maxResults, pages)
			}
			gotPrefixes = append(gotPrefixes, objs.Prefixes...)
			gotItems = append(gotItems, names(objs)...)
			if objs.NextPageToken == "" {
				break
			}
			cursor, err := gcsutil.DecodePageToken(objs.NextPageToken)
			if err != nil {
				t.Fatal(err)
			}
			opts.Cursor = cursor
		}

		// Every sub-path exactly once, in order, however small the pages are.
		if !reflect.DeepEqual(gotPrefixes, wantPrefixes) {
			t.Errorf("maxResults=%d: paged through prefixes %v, want %v", maxResults, gotPrefixes, wantPrefixes)
		}
		if !reflect.DeepEqual(gotItems, wantItems) {
			t.Errorf("maxResults=%d: paged through items %v, want %v", maxResults, gotItems, wantItems)
		}
		// ceil(results/n) pages exactly: nothing but a result was ever charged for.
		if want := (len(wantPrefixes) + len(wantItems) + maxResults - 1) / maxResults; pages != want {
			t.Errorf("maxResults=%d: took %d pages, want %d", maxResults, pages, want)
		}
	}
}

// TestListObjectsSkipsReportedPrefix pins that paging past a reported sub-path doesn't
// walk what's below it again. Those objects can only roll up into the same prefix and be
// dropped, so revisiting them costs a stat apiece for nothing — which is what made paging
// a large folder take time quadratic in its size.
func TestListObjectsSkipsReportedPrefix(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(t.TempDir())
	counting := &countingStore{Store: store}
	emu := NewGcsEmu(Options{Store: counting})

	for i := range 200 {
		if err := store.Add("bkt", fmt.Sprintf("big/%03d.txt", i), []byte("x"), &storage.Object{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{"y.txt", "z.txt"} {
		if err := store.Add("bkt", key, []byte("x"), &storage.Object{}); err != nil {
			t.Fatal(err)
		}
	}

	// The first page reports "big/" off its first object; the pages after it must
	// not look at the other 199.
	opts := ListOptions{Delimiter: "/", MaxResults: 1}
	var visited []int
	for pages := 0; ; pages++ {
		if pages > 5 {
			t.Fatal("pagination did not terminate")
		}
		counting.walked = 0
		objs, err := emu.ListObjects(ctx, "", "bkt", opts)
		if err != nil {
			t.Fatal(err)
		}
		visited = append(visited, counting.walked)
		if objs.NextPageToken == "" {
			break
		}
		cursor, err := gcsutil.DecodePageToken(objs.NextPageToken)
		if err != nil {
			t.Fatal(err)
		}
		opts.Cursor = cursor
	}

	if len(visited) != 3 {
		t.Fatalf("took %d pages, want 3 (big/, y.txt, z.txt)", len(visited))
	}
	for _, walked := range visited[1:] {
		// A handful covers the root and what the page reports; the point is that it
		// isn't the 200 objects below "big/".
		if walked > 10 {
			t.Errorf("pages walked %v names, want the pages after the first to skip the folder", visited)
		}
	}
}

// countingStore counts the names a listing walks past.
type countingStore struct {
	Store
	walked int
}

func (s *countingStore) Walk(ctx context.Context, bucket string, cb func(ctx context.Context, filename string, fInfo os.FileInfo) error) error {
	return s.Store.Walk(ctx, bucket, func(ctx context.Context, filename string, fInfo os.FileInfo) error {
		s.walked++
		return cb(ctx, filename, fInfo)
	})
}

// TestListObjectsClampsMaxResults pins that a caller can't ask for a bigger page than
// Cloud Storage would give: "The service will use this parameter or 1,000 items,
// whichever is smaller." An over-large request is capped, not rejected.
func TestListObjectsClampsMaxResults(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(t.TempDir())
	emu := NewGcsEmu(Options{Store: store})

	for i := 0; i < MaxResults+10; i++ {
		key := fmt.Sprintf("obj-%05d.txt", i)
		if err := store.Add("bkt", key, []byte("x"), &storage.Object{}); err != nil {
			t.Fatal(err)
		}
	}

	for _, maxResults := range []int{0, -1, MaxResults, MaxResults + 1, 1_000_000} {
		objs, err := emu.ListObjects(ctx, "", "bkt", ListOptions{MaxResults: maxResults})
		if err != nil {
			t.Fatal(err)
		}
		if len(objs.Items) != MaxResults {
			t.Errorf("MaxResults=%d returned %d items, want %d", maxResults, len(objs.Items), MaxResults)
		}
		if objs.NextPageToken == "" {
			t.Errorf("MaxResults=%d returned no page token, but the bucket holds more", maxResults)
		}
	}

	// A limit below the cap is still honored.
	objs, err := emu.ListObjects(ctx, "", "bkt", ListOptions{MaxResults: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(objs.Items) != 7 {
		t.Errorf("MaxResults=7 returned %d items, want 7", len(objs.Items))
	}
}

// TestListObjectsGlobRejectsDelimiter pins the constraint Cloud Storage documents:
// "delimiter must be either excluded or set to '/' in requests that use the matchGlob
// parameter". Rolling results up by anything else has no meaning for a glob.
func TestListObjectsGlobRejectsDelimiter(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(t.TempDir())
	emu := NewGcsEmu(Options{Store: store})
	if err := store.Add("bkt", "a/b.txt", []byte("x"), &storage.Object{}); err != nil {
		t.Fatal(err)
	}

	for _, delimiter := range []string{"", "/"} {
		if _, err := emu.ListObjects(ctx, "", "bkt", ListOptions{MatchGlob: "**", Delimiter: delimiter}); err != nil {
			t.Errorf("delimiter %q with matchGlob: %v, want it accepted", delimiter, err)
		}
	}

	for _, delimiter := range []string{"-", "//", "|"} {
		_, err := emu.ListObjects(ctx, "", "bkt", ListOptions{MatchGlob: "**", Delimiter: delimiter})
		if err == nil {
			t.Errorf("delimiter %q with matchGlob: no error, want one", delimiter)
			continue
		}
		if code := httpStatusCodeOf(err); code != http.StatusBadRequest {
			t.Errorf("delimiter %q: status %d, want 400", delimiter, code)
		}
	}

	// Without a glob any delimiter is still fine.
	if _, err := emu.ListObjects(ctx, "", "bkt", ListOptions{Delimiter: "-"}); err != nil {
		t.Errorf("delimiter without matchGlob: %v, want it accepted", err)
	}
}

// TestListObjectsReportsPartialFailure pins that a listing which couldn't be completed
// is reported as an error rather than returned short.
//
// A caller learns it has seen every object by the absence of a page token, so handing
// back what was found so far would report a truncated bucket as a complete one — the
// listing would look authoritative while quietly omitting whatever came after the
// failure.
func TestListObjectsReportsPartialFailure(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := NewFileStore(dir)
	emu := NewGcsEmu(Options{Store: store})

	for _, key := range []string{"a.txt", "b.txt", "m/1.txt", "z.txt"} {
		if err := store.Add("bkt", key, []byte("x"), &storage.Object{}); err != nil {
			t.Fatal(err)
		}
	}

	// Walking fails partway: a sub-directory that can't be read.
	sub := filepath.Join(dir, "bkt", "m")
	if err := os.Chmod(sub, 0); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(sub, 0o755) }()
	if _, err := os.ReadDir(sub); err == nil {
		t.Skip("directory permissions are not enforced here (running as root?)")
	}

	if _, err := emu.ListObjects(ctx, "", "bkt", ListOptions{}); err == nil {
		t.Error("a listing that could not be completed returned no error")
	}

	// Resolving an object's metadata fails: a corrupt sidecar.
	if err := os.Chmod(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bkt", "b.txt"+metaExtention), []byte("{not json"), 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := emu.ListObjects(ctx, "", "bkt", ListOptions{}); err == nil {
		t.Error("a listing with an unreadable object returned no error")
	}
}
