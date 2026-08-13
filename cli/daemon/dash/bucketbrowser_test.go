package dash

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/storage/v1"

	"encr.dev/cli/daemon/objects"
	"encr.dev/pkg/emulators/storage/gcsemu"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

const testBucket = "photos"

// newTestTarget returns a bucketTarget backed by an empty on-disk store,
// the same kind of store `encore run` uses.
func newTestTarget(t *testing.T, bucket *meta.Bucket) *bucketTarget {
	t.Helper()
	store := gcsemu.NewFileStore(t.TempDir())
	return &bucketTarget{
		store:         store,
		emu:           gcsemu.NewGcsEmu(gcsemu.Options{Store: store}),
		bucket:        bucket,
		publicBaseURL: "http://127.0.0.1:9800/testns",
		maxProxyBytes: maxBucketProxyBytes,
	}
}

func seed(t *testing.T, target *bucketTarget, keys ...string) {
	t.Helper()
	for _, key := range keys {
		err := target.store.Add(target.bucket.Name, key, []byte("contents of "+key), &storage.Object{
			ContentType: "text/plain",
		})
		if err != nil {
			t.Fatalf("seeding %s: %v", key, err)
		}
	}
}

func keysOf(objs []bucketObject) []string {
	keys := make([]string, 0, len(objs))
	for _, obj := range objs {
		keys = append(keys, obj.Key)
	}
	return keys
}

func TestValidateObjectKey(t *testing.T) {
	valid := []string{
		"a.txt",
		"a/b/c.txt",
		"folder/",
		"a b.txt",
		"weird..name.txt",
		"...",
		"räksmörgås.txt",
	}
	for _, key := range valid {
		if err := validateObjectKey(key); err != nil {
			t.Errorf("validateObjectKey(%q) = %v, want nil", key, err)
		}
	}

	invalid := []string{
		"",
		"/a.txt",
		"../secrets",
		"a/../../secrets",
		"a/./b",
		"..",
		".",
		"a/..",
		`a\b`,
		`..\..\secrets`,
	}
	for _, key := range invalid {
		if err := validateObjectKey(key); err == nil {
			t.Errorf("validateObjectKey(%q) = nil, want an error", key)
		}
	}
}

func TestClampBucketPageSize(t *testing.T) {
	tests := []struct {
		in   int
		want int
	}{
		{0, maxBucketPageSize},
		{-1, maxBucketPageSize},
		{25, 25},
		{maxBucketPageSize, maxBucketPageSize},
		{maxBucketPageSize + 1, maxBucketPageSize},
	}
	for _, test := range tests {
		if got := clampBucketPageSize(test.in); got != test.want {
			t.Errorf("clampBucketPageSize(%d) = %d, want %d", test.in, got, test.want)
		}
	}
}

func TestListFolderView(t *testing.T) {
	ctx := context.Background()
	target := newTestTarget(t, &meta.Bucket{Name: testBucket})
	seed(t, target, "root.txt", "2023/a.jpg", "2024/a.jpg", "2024/06/b.jpg")

	// With a delimiter set, sub-paths are rolled up instead of listed.
	res, err := target.List(ctx, bucketListRequest{Delimiter: "/"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"root.txt"}; !reflect.DeepEqual(keysOf(res.Objects), want) {
		t.Errorf("objects = %v, want %v", keysOf(res.Objects), want)
	}
	if want := []string{"2023/", "2024/"}; !reflect.DeepEqual(res.Prefixes, want) {
		t.Errorf("prefixes = %v, want %v", res.Prefixes, want)
	}

	// Descending into a folder lists that folder's contents only.
	res, err = target.List(ctx, bucketListRequest{Prefix: "2024/", Delimiter: "/"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"2024/a.jpg"}; !reflect.DeepEqual(keysOf(res.Objects), want) {
		t.Errorf("objects = %v, want %v", keysOf(res.Objects), want)
	}
	if want := []string{"2024/06/"}; !reflect.DeepEqual(res.Prefixes, want) {
		t.Errorf("prefixes = %v, want %v", res.Prefixes, want)
	}

	// Without a delimiter the listing is recursive.
	res, err = target.List(ctx, bucketListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(res.Objects); got != 4 {
		t.Errorf("recursive listing returned %d objects, want 4", got)
	}
	if res.Prefixes == nil {
		t.Error("prefixes = nil, want an empty slice: the dashboard can't distinguish null from absent")
	}
}

func TestListEmptyBucket(t *testing.T) {
	// A bucket that's declared but never written to has no directory on disk.
	// Listing it should report it as empty rather than fail.
	target := newTestTarget(t, &meta.Bucket{Name: testBucket})

	res, err := target.List(context.Background(), bucketListRequest{Delimiter: "/"})
	if err != nil {
		t.Fatalf("listing a bucket that has never been written to: %v", err)
	}
	if len(res.Objects) != 0 || len(res.Prefixes) != 0 || res.NextPageToken != "" {
		t.Errorf("got %+v, want an empty listing", res)
	}
	if res.Prefixes == nil || res.Objects == nil {
		t.Errorf("got null prefixes/objects, want empty slices")
	}
}

func TestListPagination(t *testing.T) {
	ctx := context.Background()
	target := newTestTarget(t, &meta.Bucket{Name: testBucket})
	want := []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt"}
	seed(t, target, want...)

	var got []string
	pageToken := ""
	for i := 0; ; i++ {
		if i > 10 {
			t.Fatal("pagination did not terminate")
		}
		res, err := target.List(ctx, bucketListRequest{PageSize: 2, PageToken: pageToken})
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, keysOf(res.Objects)...)
		if res.NextPageToken == "" {
			break
		}
		pageToken = res.NextPageToken
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("paging through the bucket gave %v, want %v", got, want)
	}
}

// TestListPaginationWithPrefixRollup covers a page that fills up entirely with
// rolled-up sub-paths: it still has to hand back a token, or the dashboard stops
// early and silently hides the remaining folders. Each folder is reported once, on
// one page, so the dashboard can append pages as they arrive.
func TestListPaginationWithPrefixRollup(t *testing.T) {
	ctx := context.Background()
	target := newTestTarget(t, &meta.Bucket{Name: testBucket})
	seed(t, target, "a/1.txt", "b/1.txt", "c/1.txt", "d/1.txt")

	var got []string
	pageToken := ""
	for i := 0; ; i++ {
		if i > 10 {
			t.Fatal("pagination did not terminate")
		}
		res, err := target.List(ctx, bucketListRequest{Delimiter: "/", PageSize: 2, PageToken: pageToken})
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, res.Prefixes...)
		if res.NextPageToken == "" {
			break
		}
		pageToken = res.NextPageToken
	}

	if want := []string{"a/", "b/", "c/", "d/"}; !reflect.DeepEqual(got, want) {
		t.Errorf("paging through the folders gave %v, want %v", got, want)
	}
}

// TestListLargeFolder pins that how big a folder is doesn't affect how many pages it
// takes to browse the folder it sits in: it is one row in the dashboard however many
// objects are below it, so it costs one result of the page reporting it.
func TestListLargeFolder(t *testing.T) {
	ctx := context.Background()
	target := newTestTarget(t, &meta.Bucket{Name: testBucket})

	keys := make([]string, 0, 201)
	for i := range 200 {
		keys = append(keys, fmt.Sprintf("holiday/%03d.jpg", i))
	}
	keys = append(keys, "readme.txt")
	seed(t, target, keys...)

	res, err := target.List(ctx, bucketListRequest{Delimiter: "/", PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"holiday/"}; !reflect.DeepEqual(res.Prefixes, want) {
		t.Errorf("prefixes = %v, want %v", res.Prefixes, want)
	}
	if want := []string{"readme.txt"}; !reflect.DeepEqual(keysOf(res.Objects), want) {
		t.Errorf("objects = %v, want %v", keysOf(res.Objects), want)
	}
	if res.NextPageToken != "" {
		t.Error("got a page token, want the whole folder view on one page")
	}
}

func TestListInvalidPageToken(t *testing.T) {
	target := newTestTarget(t, &meta.Bucket{Name: testBucket})
	_, err := target.List(context.Background(), bucketListRequest{PageToken: "not-a-token"})
	if err == nil {
		t.Error("expected an error for a malformed page token")
	}
}

func TestSearch(t *testing.T) {
	ctx := context.Background()
	target := newTestTarget(t, &meta.Bucket{Name: testBucket})
	seed(t, target, "report.pdf", "reports/q1.pdf", "2024/annual-report.pdf", "notes.txt")

	tests := []struct {
		name         string
		req          bucketSearchRequest
		want         []string
		wantPrefixes []string
	}{
		{
			name: "prefix search is anchored at the start of the key",
			req:  bucketSearchRequest{Query: "report", Recursive: true},
			want: []string{"report.pdf", "reports/q1.pdf"},
		},
		{
			// As Cloud Storage does with a delimiter set.
			name:         "non-recursive prefix search rolls matches below a sub-path up",
			req:          bucketSearchRequest{Query: "report"},
			want:         []string{"report.pdf"},
			wantPrefixes: []string{"reports/"},
		},
		{
			name: "glob search matches anywhere in the key",
			req:  bucketSearchRequest{Query: "**report**", Mode: string(searchModeGlob)},
			want: []string{"2024/annual-report.pdf", "report.pdf", "reports/q1.pdf"},
		},
		{
			name: "glob search by extension",
			req:  bucketSearchRequest{Query: "**.txt", Mode: string(searchModeGlob)},
			want: []string{"notes.txt"},
		},
		{
			name: "glob search is scoped to the prefix",
			req:  bucketSearchRequest{Prefix: "reports/", Query: "**.pdf", Mode: string(searchModeGlob)},
			want: []string{"reports/q1.pdf"},
		},
		{
			name: "prefix search is scoped to the prefix",
			req:  bucketSearchRequest{Prefix: "reports/", Query: "q", Recursive: true},
			want: []string{"reports/q1.pdf"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := target.Search(ctx, test.req)
			if err != nil {
				t.Fatal(err)
			}
			got := keysOf(res.Objects)
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("objects = %v, want %v", got, test.want)
			}
			wantPrefixes := test.wantPrefixes
			if wantPrefixes == nil {
				wantPrefixes = []string{}
			}
			if !reflect.DeepEqual(res.Prefixes, wantPrefixes) {
				t.Errorf("prefixes = %v, want %v", res.Prefixes, wantPrefixes)
			}
		})
	}
}

// TestSearchPaginationWithPrefixRollup pins that a non-recursive search spends its pages
// the way a delimited listing does, rather than charging the page for objects a sub-path
// stands for and then not reporting them.
func TestSearchPaginationWithPrefixRollup(t *testing.T) {
	ctx := context.Background()
	target := newTestTarget(t, &meta.Bucket{Name: testBucket})
	seed(t, target, "report-2024/q1.pdf", "report-2024/q2.pdf", "report-2024/q3.pdf",
		"report-2025/q1.pdf", "report.pdf", "notes.txt")

	res, err := target.Search(ctx, bucketSearchRequest{Query: "report", PageSize: 3})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"report-2024/", "report-2025/"}; !reflect.DeepEqual(res.Prefixes, want) {
		t.Errorf("prefixes = %v, want %v", res.Prefixes, want)
	}
	if want := []string{"report.pdf"}; !reflect.DeepEqual(keysOf(res.Objects), want) {
		t.Errorf("objects = %v, want %v", keysOf(res.Objects), want)
	}
	if res.NextPageToken != "" {
		t.Error("got a page token, want every match on one page")
	}

	// A glob search rolls nothing up, so it reports the objects themselves.
	res, err = target.Search(ctx, bucketSearchRequest{Query: "report**", Mode: string(searchModeGlob)})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Prefixes) != 0 {
		t.Errorf("prefixes = %v, want none for a glob search", res.Prefixes)
	}
	if got := len(res.Objects); got != 5 {
		t.Errorf("objects = %v, want all 5 matches", keysOf(res.Objects))
	}
}

func TestSearchInvalidRequests(t *testing.T) {
	ctx := context.Background()
	target := newTestTarget(t, &meta.Bucket{Name: testBucket})
	seed(t, target, "a.txt")

	if _, err := target.Search(ctx, bucketSearchRequest{}); err == nil {
		t.Error("expected an error when the query is missing")
	}
	if _, err := target.Search(ctx, bucketSearchRequest{Query: "a", Mode: "regex"}); err == nil {
		t.Error("expected an error for an unknown search mode")
	}
	if _, err := target.Search(ctx, bucketSearchRequest{Query: "[", Mode: string(searchModeGlob)}); err == nil {
		t.Error("expected an error for a malformed glob pattern")
	}
}

func TestCreateFolderAndDelete(t *testing.T) {
	ctx := context.Background()
	target := newTestTarget(t, &meta.Bucket{Name: testBucket})

	if _, err := target.CreateFolder(bucketCreateFolderRequest{Prefix: "drafts"}); err != nil {
		t.Fatal(err)
	}

	// The placeholder object makes the otherwise empty folder visible.
	res, err := target.List(ctx, bucketListRequest{Delimiter: "/"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"drafts/"}; !reflect.DeepEqual(res.Prefixes, want) {
		t.Fatalf("prefixes = %v, want %v", res.Prefixes, want)
	}

	// Deleting the placeholder removes the folder again.
	if _, err := target.Delete(bucketDeleteRequest{Keys: []string{"drafts/"}}); err != nil {
		t.Fatal(err)
	}
	res, err = target.List(ctx, bucketListRequest{Delimiter: "/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Prefixes) != 0 {
		t.Errorf("prefixes = %v, want none after deleting the placeholder", res.Prefixes)
	}

	// Deleting something that isn't there is not an error, so that a retried
	// delete doesn't surface a failure.
	if _, err := target.Delete(bucketDeleteRequest{Keys: []string{"drafts/"}}); err != nil {
		t.Errorf("deleting a missing object: %v", err)
	}

	if _, err := target.Delete(bucketDeleteRequest{}); err == nil {
		t.Error("expected an error when no keys are given")
	}
}

// TestDeleteRejectsTraversal makes sure a key can't be used to delete files outside
// the bucket, since the local emulator maps keys onto filesystem paths.
func TestDeleteRejectsTraversal(t *testing.T) {
	target := newTestTarget(t, &meta.Bucket{Name: testBucket})
	seed(t, target, "keep.txt")

	if _, err := target.Delete(bucketDeleteRequest{Keys: []string{"../../keep.txt"}}); err == nil {
		t.Fatal("expected an error for a traversing key")
	}

	// Nothing was deleted: the whole request is rejected before any key is touched.
	res, err := target.List(context.Background(), bucketListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"keep.txt"}; !reflect.DeepEqual(keysOf(res.Objects), want) {
		t.Errorf("objects = %v, want %v", keysOf(res.Objects), want)
	}
}

func TestPublicURL(t *testing.T) {
	ctx := context.Background()

	private := newTestTarget(t, &meta.Bucket{Name: testBucket})
	seed(t, private, "a b.txt")
	res, err := private.List(ctx, bucketListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Objects[0].PublicURL; got != "" {
		t.Errorf("public_url = %q for a private bucket, want empty", got)
	}

	public := newTestTarget(t, &meta.Bucket{Name: testBucket, Public: true})
	seed(t, public, "a b.txt")
	res, err = public.List(ctx, bucketListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	want := "http://127.0.0.1:9800/testns/photos/a%20b.txt"
	if got := res.Objects[0].PublicURL; got != want {
		t.Errorf("public_url = %q, want %q", got, want)
	}
}

func TestDownloadURLTTL(t *testing.T) {
	target := newTestTarget(t, &meta.Bucket{Name: testBucket})
	seed(t, target, "a.txt")
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		ttlSeconds int
		want       time.Duration
	}{
		{"unset falls back to the default", 0, bucketSignedURLTTL},
		{"negative falls back to the default", -5, bucketSignedURLTTL},
		{"honored when in range", 3600, time.Hour},
		{"clamped to the maximum", int(maxBucketSignedURLTTL.Seconds()) * 2, maxBucketSignedURLTTL},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := target.DownloadURL(bucketDownloadURLRequest{Key: "a.txt", TTLSeconds: test.ttlSeconds}, now)
			if err != nil {
				t.Fatal(err)
			}
			if got := res.ExpiresAt.Sub(now); got != test.want {
				t.Errorf("expires_at is %v after now, want %v", got, test.want)
			}
			if res.Method != "GET" {
				t.Errorf("method = %q, want GET", res.Method)
			}

			// The expiry the public bucket server enforces must match what we reported.
			u, err := url.Parse(res.URL)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := u.Query().Get("X-Goog-Expires"), strconv.Itoa(int(test.want.Seconds())); got != want {
				t.Errorf("X-Goog-Expires = %q, want %q", got, want)
			}
		})
	}

	if _, err := target.DownloadURL(bucketDownloadURLRequest{Key: "missing.txt"}, now); err == nil {
		t.Error("expected an error for an object that doesn't exist")
	}
	if _, err := target.DownloadURL(bucketDownloadURLRequest{Key: "../a.txt"}, now); err == nil {
		t.Error("expected an error for a traversing key")
	}
}

// TestDownloadURLServedByPublicBucketServer checks that the URL we hand out is one
// the daemon's public bucket server actually accepts, and that it stops working once
// it expires.
func TestDownloadURLServedByPublicBucketServer(t *testing.T) {
	target := newTestTarget(t, &meta.Bucket{Name: testBucket})
	seed(t, target, "a.txt")

	public := objects.NewPublicBucketServer("", func(ns string) (gcsemu.Store, bool) {
		if ns != "testns" {
			return nil, false
		}
		return target.store, true
	})
	srv := httptest.NewServer(public)
	defer srv.Close()
	target.publicBaseURL = srv.URL + "/testns"

	res, err := target.DownloadURL(bucketDownloadURLRequest{Key: "a.txt", TTLSeconds: 60}, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	resp, err := srv.Client().Get(res.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d, body %q", res.URL, resp.StatusCode, body)
	}
	if want := "contents of a.txt"; string(body) != want {
		t.Errorf("body = %q, want %q", body, want)
	}

	// A URL generated far enough in the past is no longer accepted.
	expired, err := target.DownloadURL(bucketDownloadURLRequest{Key: "a.txt", TTLSeconds: 60},
		time.Now().Add(-10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	resp, err = srv.Client().Get(expired.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Error("an expired download URL was still served")
	}
}

func TestServeContent(t *testing.T) {
	target := newTestTarget(t, &meta.Bucket{Name: testBucket})

	// Upload.
	body := strings.NewReader("hello world")
	req := httptest.NewRequest(http.MethodPut, "/__encore/objects/content?key=greeting.txt", body)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	w := httptest.NewRecorder()
	target.upload(w, req, "greeting.txt")
	if w.Code != http.StatusOK {
		t.Fatalf("upload: status %d, body %q", w.Code, w.Body)
	}
	// Encore Cloud's upload endpoint answers with the same body, so a shared client
	// doesn't have to special-case local.
	if got, want := strings.TrimSpace(w.Body.String()), `{"ok":true}`; got != want {
		t.Errorf("upload response = %s, want %s", got, want)
	}

	// Download.
	req = httptest.NewRequest(http.MethodGet, "/__encore/objects/content?key=greeting.txt", nil)
	w = httptest.NewRecorder()
	target.download(w, req, "greeting.txt", false)
	if w.Code != http.StatusOK {
		t.Fatalf("download: status %d, body %q", w.Code, w.Body)
	}
	if got := w.Body.String(); got != "hello world" {
		t.Errorf("body = %q, want %q", got, "hello world")
	}
	if got := w.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want the content type it was uploaded with", got)
	}
	if got := w.Header().Get("Content-Disposition"); got != "inline; filename=greeting.txt" {
		t.Errorf("Content-Disposition = %q, want it to be inline", got)
	}
	// Object contents share an origin with the dashboard, so they must not be sniffed
	// or allowed to run scripts.
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := w.Header().Get("Content-Security-Policy"); got != "sandbox" {
		t.Errorf("Content-Security-Policy = %q, want sandbox", got)
	}

	// Download as an attachment.
	req = httptest.NewRequest(http.MethodGet, "/__encore/objects/content?key=greeting.txt&download=true", nil)
	w = httptest.NewRecorder()
	target.download(w, req, "greeting.txt", true)
	if got := w.Header().Get("Content-Disposition"); got != "attachment; filename=greeting.txt" {
		t.Errorf("Content-Disposition = %q, want an attachment", got)
	}

	// A missing object is a 404, not a 500.
	req = httptest.NewRequest(http.MethodGet, "/__encore/objects/content?key=nope.txt", nil)
	w = httptest.NewRecorder()
	target.download(w, req, "nope.txt", false)
	if w.Code != http.StatusNotFound {
		t.Errorf("download of a missing object: status %d, want 404", w.Code)
	}
}

func TestServeContentUploadRejectsFolderKey(t *testing.T) {
	target := newTestTarget(t, &meta.Bucket{Name: testBucket})
	req := httptest.NewRequest(http.MethodPut, "/__encore/objects/content?key=folder/", strings.NewReader("x"))
	w := httptest.NewRecorder()
	target.upload(w, req, "folder/")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", w.Code)
	}
}

// TestServeContentUploadTooLarge covers both ways an oversized upload is turned
// away: on the length it declares, and — for a body that declares none or lies
// about it — on the bytes that actually arrive.
func TestServeContentUploadTooLarge(t *testing.T) {
	const limit = 8

	t.Run("declared length", func(t *testing.T) {
		target := newTestTarget(t, &meta.Bucket{Name: testBucket})
		target.maxProxyBytes = limit

		req := httptest.NewRequest(http.MethodPut, "/__encore/objects/content?key=big.bin",
			strings.NewReader(strings.Repeat("x", limit+1)))
		w := httptest.NewRecorder()
		target.upload(w, req, "big.bin")
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("status %d, want 413", w.Code)
		}
	})

	t.Run("undeclared length", func(t *testing.T) {
		target := newTestTarget(t, &meta.Bucket{Name: testBucket})
		target.maxProxyBytes = limit

		req := httptest.NewRequest(http.MethodPut, "/__encore/objects/content?key=big.bin",
			strings.NewReader(strings.Repeat("x", limit+1)))
		req.ContentLength = -1
		w := httptest.NewRecorder()
		target.upload(w, req, "big.bin")
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("status %d, want 413", w.Code)
		}
	})

	t.Run("at the limit", func(t *testing.T) {
		target := newTestTarget(t, &meta.Bucket{Name: testBucket})
		target.maxProxyBytes = limit

		req := httptest.NewRequest(http.MethodPut, "/__encore/objects/content?key=ok.bin",
			strings.NewReader(strings.Repeat("x", limit)))
		w := httptest.NewRecorder()
		target.upload(w, req, "ok.bin")
		if w.Code != http.StatusOK {
			t.Errorf("status %d, want 200: an object exactly at the limit is allowed", w.Code)
		}
	})
}

// TestServeContentDownloadTooLarge pins that an object too big to proxy is refused
// rather than read into the daemon's memory. Encore Cloud answers the same way, and
// the dashboard falls back to a download URL.
func TestServeContentDownloadTooLarge(t *testing.T) {
	target := newTestTarget(t, &meta.Bucket{Name: testBucket})
	seed(t, target, "big.bin")
	target.maxProxyBytes = 1

	req := httptest.NewRequest(http.MethodGet, "/__encore/objects/content?key=big.bin", nil)
	w := httptest.NewRecorder()
	target.download(w, req, "big.bin", false)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status %d, want 413", w.Code)
	}
}

func TestObjectFilename(t *testing.T) {
	tests := map[string]string{
		"a.txt":       "a.txt",
		"a/b/c.txt":   "c.txt",
		"a/b/folder/": "folder",
	}
	for key, want := range tests {
		if got := objectFilename(key); got != want {
			t.Errorf("objectFilename(%q) = %q, want %q", key, got, want)
		}
	}
}

// TestSearchGlobInFolderWithMetacharacters pins that the folder a search is scoped to
// is treated as a literal key prefix, not as part of the pattern. Object keys are
// opaque and routinely contain glob syntax, so browsing into "a[1]/" and searching
// there has to work like browsing into any other folder.
//
// A backslash is worth its place here: it is a legal filename character and the glob
// escape character both, so it only works if the prefix is escaped and unescaped in
// step. The dashboard rejects such keys on the way in, but an app writing through the
// objects SDK doesn't go through that check, so buckets really do hold them.
func TestSearchGlobInFolderWithMetacharacters(t *testing.T) {
	ctx := context.Background()
	for _, folder := range []string{"a[1]/", "a{x}/", "a*b/", "a?b/", "a,b/", "a}b/", `a\b/`, "plain/"} {
		target := newTestTarget(t, &meta.Bucket{Name: testBucket})
		seed(t, target, folder+"photo.png", folder+"notes.txt", "elsewhere/photo.png")

		res, err := target.Search(ctx, bucketSearchRequest{
			Prefix: folder, Query: "**.png", Mode: string(searchModeGlob),
		})
		if err != nil {
			t.Errorf("folder %q: %v", folder, err)
			continue
		}
		if want := []string{folder + "photo.png"}; !reflect.DeepEqual(keysOf(res.Objects), want) {
			t.Errorf("folder %q: objects = %v, want %v", folder, keysOf(res.Objects), want)
		}
	}
}

// TestListPaginationOrdering pins that paging the dashboard's listing returns every
// object exactly once, in order. A key like "a.txt" sorts before the contents of the
// folder "a/", which is not the order the two sit in on disk.
func TestListPaginationOrdering(t *testing.T) {
	ctx := context.Background()
	target := newTestTarget(t, &meta.Bucket{Name: testBucket})
	want := []string{"a!c.txt", "a-b.txt", "a.txt", "a/z.txt", "b.txt", "b/x/y.txt"}
	seed(t, target, want...)

	res, err := target.List(ctx, bucketListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got := keysOf(res.Objects); !reflect.DeepEqual(got, want) {
		t.Errorf("unpaged listing = %v, want %v", got, want)
	}

	for _, pageSize := range []int{1, 2, 3, 5} {
		var got []string
		pageToken := ""
		for i := 0; ; i++ {
			if i > 20 {
				t.Fatalf("PageSize=%d: pagination did not terminate", pageSize)
			}
			res, err := target.List(ctx, bucketListRequest{PageSize: pageSize, PageToken: pageToken})
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, keysOf(res.Objects)...)
			if res.NextPageToken == "" {
				break
			}
			pageToken = res.NextPageToken
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("PageSize=%d paged through %v, want %v", pageSize, got, want)
		}
	}
}
