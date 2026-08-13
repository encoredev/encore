package dash

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/storage/v1"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/namespace"
	"encr.dev/cli/daemon/run"
	"encr.dev/pkg/emulators/storage/gcsemu"
	"encr.dev/pkg/emulators/storage/gcsutil"
	"encr.dev/pkg/fns"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

const (
	// maxBucketPageSize caps how many objects a single list or search request returns.
	maxBucketPageSize = 1000

	// bucketSignedURLTTL is the lifetime of a download URL when the caller doesn't ask for one.
	bucketSignedURLTTL = 15 * time.Minute

	// maxBucketSignedURLTTL is the longest a minted download URL may stay valid.
	// It matches the cap Encore Cloud applies, so the dashboard sees one contract.
	maxBucketSignedURLTTL = time.Hour

	// maxBucketProxyBytes caps an object served or accepted through the content
	// endpoint, in both directions, matching the cap Encore Cloud proxies under.
	maxBucketProxyBytes = 1 << 30 // 1 GiB

	// maxBucketDeleteKeys caps the keys one delete request may name, matching the
	// cap Encore Cloud applies.
	maxBucketDeleteKeys = 1000
)

// errBucketNotFound reports that the application doesn't declare the requested bucket.
var errBucketNotFound = errors.New("bucket not found")

// bucketBrowser serves the developer dashboard's object storage browser, backed by
// the local object storage emulator.
type bucketBrowser struct {
	apps *apps.Manager
	run  *run.Manager
	ns   *namespace.Manager
}

// bucketObject is the JSON representation of a single object.
//
// The field names mirror Encore Cloud's bucket browser API.
type bucketObject struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	ContentType  string    `json:"content_type,omitempty"`
	ETag         string    `json:"etag,omitempty"`
	StorageClass string    `json:"storage_class,omitempty"`
	Updated      time.Time `json:"updated"`

	// PublicURL is set only for objects in public buckets.
	PublicURL string `json:"public_url,omitempty"`
}

type bucketListRequest struct {
	AppID  string `json:"app_id"`
	Bucket string `json:"bucket"`

	// Prefix restricts the results to objects whose key starts with Prefix.
	Prefix string `json:"prefix"`

	// Delimiter, when set (the dashboard uses "/"), rolls objects below a
	// sub-path up into Prefixes instead of listing them individually.
	Delimiter string `json:"delimiter"`

	PageToken string `json:"page_token"`
	PageSize  int    `json:"page_size"`
}

type bucketListResponse struct {
	Prefixes      []string       `json:"prefixes"`
	Objects       []bucketObject `json:"objects"`
	NextPageToken string         `json:"next_page_token"`
}

type bucketSearchRequest struct {
	AppID  string `json:"app_id"`
	Bucket string `json:"bucket"`

	// Query is the term to search for, interpreted according to Mode.
	Query string `json:"query"`

	// Mode is either "prefix" (the default) or "glob".
	Mode string `json:"mode"`

	// Prefix restricts the search to objects below the given prefix.
	Prefix string `json:"prefix"`

	// Recursive reports whether a prefix search should descend into sub-paths.
	// It has no effect on glob searches, which always span the whole prefix.
	Recursive bool `json:"recursive"`

	PageToken string `json:"page_token"`
	PageSize  int    `json:"page_size"`
}

type bucketSearchResponse struct {
	// Prefixes holds the sub-paths a non-recursive search matched below, the way Cloud
	// Storage reports them. A recursive or glob search rolls nothing up, so it's empty.
	Prefixes      []string       `json:"prefixes"`
	Objects       []bucketObject `json:"objects"`
	NextPageToken string         `json:"next_page_token"`
}

type bucketDeleteRequest struct {
	AppID  string   `json:"app_id"`
	Bucket string   `json:"bucket"`
	Keys   []string `json:"keys"`
}

type bucketCreateFolderRequest struct {
	AppID  string `json:"app_id"`
	Bucket string `json:"bucket"`
	Prefix string `json:"prefix"`
}

type bucketDownloadURLRequest struct {
	AppID      string `json:"app_id"`
	Bucket     string `json:"bucket"`
	Key        string `json:"key"`
	TTLSeconds int    `json:"ttl_seconds"`
}

type bucketSignedURLResponse struct {
	URL       string    `json:"url"`
	Method    string    `json:"method"`
	ExpiresAt time.Time `json:"expires_at"`
}

type bucketOKResponse struct {
	OK bool `json:"ok"`
}

// searchMode identifies how a search query is matched against object keys.
type searchMode string

const (
	searchModePrefix searchMode = "prefix"
	searchModeGlob   searchMode = "glob"
)

// The methods below are the dashboard's entry points. Each resolves which bucket the
// request refers to and hands off to the bucketTarget method of the same name.

func (b *bucketBrowser) List(ctx context.Context, req bucketListRequest) (*bucketListResponse, error) {
	target, err := b.target(ctx, req.AppID, req.Bucket)
	if err != nil {
		return nil, err
	}
	return target.List(ctx, req)
}

func (b *bucketBrowser) Search(ctx context.Context, req bucketSearchRequest) (*bucketSearchResponse, error) {
	target, err := b.target(ctx, req.AppID, req.Bucket)
	if err != nil {
		return nil, err
	}
	return target.Search(ctx, req)
}

func (b *bucketBrowser) Delete(ctx context.Context, req bucketDeleteRequest) (*bucketOKResponse, error) {
	target, err := b.target(ctx, req.AppID, req.Bucket)
	if err != nil {
		return nil, err
	}
	return target.Delete(req)
}

func (b *bucketBrowser) CreateFolder(ctx context.Context, req bucketCreateFolderRequest) (*bucketOKResponse, error) {
	target, err := b.target(ctx, req.AppID, req.Bucket)
	if err != nil {
		return nil, err
	}
	return target.CreateFolder(req)
}

func (b *bucketBrowser) DownloadURL(ctx context.Context, req bucketDownloadURLRequest) (*bucketSignedURLResponse, error) {
	target, err := b.target(ctx, req.AppID, req.Bucket)
	if err != nil {
		return nil, err
	}
	return target.DownloadURL(req, time.Now())
}

// List lists the objects and sub-paths directly below the requested prefix.
func (t *bucketTarget) List(ctx context.Context, req bucketListRequest) (*bucketListResponse, error) {
	objs, err := t.list(ctx, gcsemu.ListOptions{
		Prefix:     req.Prefix,
		Delimiter:  req.Delimiter,
		MaxResults: clampBucketPageSize(req.PageSize),
	}, req.PageToken)
	if err != nil {
		return nil, err
	}

	return &bucketListResponse{
		Prefixes:      bucketPrefixes(objs),
		Objects:       t.objects(objs),
		NextPageToken: objs.NextPageToken,
	}, nil
}

// Search finds objects matching a query, either by key prefix or by glob pattern.
func (t *bucketTarget) Search(ctx context.Context, req bucketSearchRequest) (*bucketSearchResponse, error) {
	if req.Query == "" {
		return nil, errors.New("missing query")
	}

	mode := searchMode(req.Mode)
	if mode == "" {
		mode = searchModePrefix
	}

	opts := gcsemu.ListOptions{
		Prefix:     req.Prefix,
		MaxResults: clampBucketPageSize(req.PageSize),
	}
	switch mode {
	case searchModePrefix:
		// Match keys that begin with the query, relative to the prefix being browsed.
		opts.Prefix = req.Prefix + req.Query
		if !req.Recursive {
			opts.Delimiter = "/"
		}
	case searchModeGlob:
		// Glob search is always available locally, because the emulator evaluates globs.
		//
		// The prefix is a literal key, not part of the pattern, so it has to be escaped:
		// a folder named "a[1]/" would otherwise be read as a character class and match
		// nothing at all.
		opts.MatchGlob = gcsemu.EscapeGlobLiteral(req.Prefix) + req.Query
	default:
		return nil, errors.Newf("invalid search mode %q", req.Mode)
	}

	objs, err := t.list(ctx, opts, req.PageToken)
	if err != nil {
		return nil, err
	}

	return &bucketSearchResponse{
		Prefixes:      bucketPrefixes(objs),
		Objects:       t.objects(objs),
		NextPageToken: objs.NextPageToken,
	}, nil
}

// Delete removes the given objects. Keys that don't exist are ignored, so that
// deleting the same object twice isn't an error.
func (t *bucketTarget) Delete(req bucketDeleteRequest) (*bucketOKResponse, error) {
	if len(req.Keys) == 0 {
		return nil, errors.New("no keys to delete")
	}
	if len(req.Keys) > maxBucketDeleteKeys {
		return nil, errors.Newf("too many keys to delete (max %d per request)", maxBucketDeleteKeys)
	}
	for _, key := range req.Keys {
		if err := validateObjectKey(key); err != nil {
			return nil, err
		}
	}

	for _, key := range req.Keys {
		if err := t.store.Delete(t.bucket.Name, key); err != nil && !os.IsNotExist(err) {
			return nil, errors.Wrapf(err, "delete %s", key)
		}
	}
	return &bucketOKResponse{OK: true}, nil
}

// CreateFolder creates an empty placeholder object so that an otherwise empty
// folder shows up when listing with a delimiter.
//
// Cloud Storage has no folders in a flat bucket and no API to create one, so this
// writes the zero-byte object named "<folder>/" that the Cloud Console writes for
// the same purpose. Being an ordinary object is what makes it behave: listing from
// above rolls it up into a prefix, listing from inside returns it as an item, and
// deleting it by name removes the folder.
func (t *bucketTarget) CreateFolder(req bucketCreateFolderRequest) (*bucketOKResponse, error) {
	prefix := req.Prefix
	if prefix == "" {
		return nil, errors.New("missing prefix")
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	if err := validateObjectKey(prefix); err != nil {
		return nil, err
	}

	if err := t.store.Add(t.bucket.Name, prefix, nil, &storage.Object{}); err != nil {
		return nil, errors.Wrap(err, "create folder")
	}
	return &bucketOKResponse{OK: true}, nil
}

// DownloadURL returns a URL valid until now+ttl for downloading an object directly,
// suitable for sharing. It points at the daemon's public bucket server rather than at
// the dashboard, so it works without the dashboard being open.
func (t *bucketTarget) DownloadURL(req bucketDownloadURLRequest, now time.Time) (*bucketSignedURLResponse, error) {
	if err := validateObjectKey(req.Key); err != nil {
		return nil, err
	}

	// Make sure the object actually exists, so a broken link isn't handed out.
	obj, err := t.store.GetMeta(gcsemu.HttpBaseUrl(""), t.bucket.Name, req.Key)
	if err != nil {
		return nil, errors.Wrap(err, "stat object")
	} else if obj == nil {
		return nil, errors.Newf("object %q not found", req.Key)
	}

	ttl := bucketSignedURLTTL
	if req.TTLSeconds > 0 {
		ttl = min(time.Duration(req.TTLSeconds)*time.Second, maxBucketSignedURLTTL)
	}

	return &bucketSignedURLResponse{
		URL:       t.signedURL(req.Key, now, ttl),
		Method:    "GET",
		ExpiresAt: now.Add(ttl),
	}, nil
}

// ServeContent serves the raw bytes of an object, and accepts uploads.
//
//	GET|HEAD /__encore/objects/content?app_id=&bucket=&key=[&download=true]
//	PUT      /__encore/objects/content?app_id=&bucket=&key=
//
// Object contents are proxied through the daemon rather than read from the object storage
// emulator directly, so that the dashboard doesn't need to know where the emulator is
// listening and so that reading an object isn't a cross-origin request.
func (b *bucketBrowser) ServeContent(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	key := q.Get("key")
	if err := validateObjectKey(key); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	target, err := b.target(req.Context(), q.Get("app_id"), q.Get("bucket"))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errBucketNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	switch req.Method {
	case http.MethodGet, http.MethodHead:
		target.download(w, req, key, q.Get("download") == "true")
	case http.MethodPut:
		target.upload(w, req, key)
	default:
		w.Header().Set("Allow", "GET, HEAD, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// download writes the object's contents to w, as an attachment if asAttachment is set.
func (t *bucketTarget) download(w http.ResponseWriter, req *http.Request, key string, asAttachment bool) {
	// Check the size before reading, since the store hands back the whole object at
	// once: an object over the cap is rejected without ever being held in memory.
	if info, err := t.store.GetMeta(gcsemu.HttpBaseUrl(""), t.bucket.Name, key); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if info == nil {
		http.Error(w, "object not found", http.StatusNotFound)
		return
	} else if info.Size > t.maxProxyBytes {
		http.Error(w, fmt.Sprintf("object exceeds the maximum size of %d bytes that can be served through the dashboard; use a download URL instead", t.maxProxyBytes),
			http.StatusRequestEntityTooLarge)
		return
	}

	obj, contents, err := t.store.Get(gcsemu.HttpBaseUrl(""), t.bucket.Name, key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if obj == nil {
		http.Error(w, "object not found", http.StatusNotFound)
		return
	}

	contentType := obj.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	if obj.Etag != "" {
		w.Header().Set("Etag", strconv.Quote(obj.Etag))
	}

	// Object contents are served from the same origin as the dashboard itself, so make
	// sure a stored HTML or SVG file can't run scripts in it.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox")

	disposition := "inline"
	if asAttachment {
		disposition = "attachment"
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{
		"filename": objectFilename(key),
	}))

	// ServeContent takes care of HEAD, range requests and conditional requests.
	var modTime time.Time
	if updated, err := time.Parse(time.RFC3339Nano, obj.Updated); err == nil {
		modTime = updated
	}
	http.ServeContent(w, req, key, modTime, bytes.NewReader(contents))
}

// upload stores the request body as the object's contents.
func (t *bucketTarget) upload(w http.ResponseWriter, req *http.Request, key string) {
	if strings.HasSuffix(key, "/") {
		http.Error(w, "invalid object key: must not end with '/'", http.StatusBadRequest)
		return
	}

	// Reject an oversized body on its declared length first, so an upload that can't
	// be stored anyway is turned away before any of it is read. MaxBytesReader still
	// backs that up, for a request whose Content-Length understates the body or is
	// absent altogether.
	if req.ContentLength > 0 && uint64(req.ContentLength) > t.maxProxyBytes {
		http.Error(w, fmt.Sprintf("object exceeds the maximum upload size of %d bytes", t.maxProxyBytes),
			http.StatusRequestEntityTooLarge)
		return
	}

	contents, err := io.ReadAll(http.MaxBytesReader(w, req.Body, int64(t.maxProxyBytes)))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, fmt.Sprintf("object exceeds the maximum upload size of %d bytes", t.maxProxyBytes),
				http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	contentType := req.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if err := t.store.Add(t.bucket.Name, key, contents, &storage.Object{ContentType: contentType}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(bucketOKResponse{OK: true}); err != nil {
		log.Error().Err(err).Msg("dash: could not write object upload response")
	}
}

// bucketTarget is a single bucket resolved for one request.
type bucketTarget struct {
	store  gcsemu.Store
	emu    *gcsemu.GcsEmu
	bucket *meta.Bucket

	// publicBaseURL is where this app's buckets are served from, without a trailing slash.
	publicBaseURL string

	// maxProxyBytes is maxBucketProxyBytes, as a field so tests can lower it rather
	// than having to materialize an object of the real size.
	maxProxyBytes uint64
}

// target resolves the store holding the given bucket's objects.
func (b *bucketBrowser) target(ctx context.Context, appID, bucketName string) (*bucketTarget, error) {
	md, err := resolveAppMeta(b.run, b.apps, appID)
	if err != nil {
		return nil, err
	}

	bucket, ok := fns.Find(md.Buckets, func(b *meta.Bucket) bool { return b.Name == bucketName })
	if !ok {
		return nil, errors.Wrapf(errBucketNotFound, "app does not declare a bucket named %q", bucketName)
	}

	store, emu, publicBaseURL, err := b.resolveStore(ctx, appID)
	if err != nil {
		return nil, err
	}

	return &bucketTarget{
		store:         store,
		emu:           emu,
		bucket:        bucket,
		publicBaseURL: publicBaseURL,
		maxProxyBytes: maxBucketProxyBytes,
	}, nil
}

// resolveStore returns the object store backing the given app's buckets and the
// emulator serving it, along with the base URL those objects are served from.
func (b *bucketBrowser) resolveStore(ctx context.Context, appID string) (gcsemu.Store, *gcsemu.GcsEmu, string, error) {
	// Use the running app's store, so we browse exactly what the application is writing to.
	if r := b.run.FindRunByAppID(appID); r != nil && r.ResourceManager != nil {
		if srv := r.ResourceManager.GetObjects(); srv != nil {
			return srv.Store(), srv.Emu(), srv.PublicBaseURL(), nil
		}
	}

	// With no run, fall back to the active namespace's storage directory, so objects stay
	// browsable after a run stops. Note that `encore run --namespace` doesn't make that
	// namespace active, so what's shown can change once such a run exits.
	ns, err := resolveNamespace(ctx, b.run, b.apps, b.ns, appID)
	if err != nil {
		return nil, nil, "", errors.Wrap(err, "resolve namespace")
	}
	baseDir, err := b.run.ObjectsMgr.BaseDir(ns.ID)
	if err != nil {
		return nil, nil, "", errors.Wrap(err, "resolve object storage directory")
	}
	publicBaseURL := b.run.PublicBuckets.BaseAddr() + "/" + ns.ID.String()
	store := gcsemu.NewFileStore(baseDir)
	return store, gcsemu.NewGcsEmu(gcsemu.Options{Store: store}), publicBaseURL, nil
}

// list runs a list request against the bucket, translating the dashboard's opaque
// page token and treating a bucket that hasn't been written to yet as empty.
func (t *bucketTarget) list(ctx context.Context, opts gcsemu.ListOptions, pageToken string) (*storage.Objects, error) {
	if pageToken != "" {
		cursor, err := gcsutil.DecodePageToken(pageToken)
		if err != nil {
			return nil, errors.Wrap(err, "invalid page token")
		}
		opts.Cursor = cursor
	}

	objs, err := t.emu.ListObjects(ctx, gcsemu.HttpBaseUrl(""), t.bucket.Name, opts)
	if err != nil {
		if os.IsNotExist(err) {
			// The bucket is declared but nothing has been written to it yet.
			return &storage.Objects{}, nil
		}
		return nil, errors.Wrap(err, "list objects")
	}
	return objs, nil
}

// bucketPrefixes returns the rolled-up sub-paths of a listing, never null: the
// dashboard distinguishes "no sub-paths" from "field absent".
func bucketPrefixes(objs *storage.Objects) []string {
	if objs.Prefixes == nil {
		return []string{}
	}
	return objs.Prefixes
}

// objects converts emulator objects to their dashboard representation.
func (t *bucketTarget) objects(objs *storage.Objects) []bucketObject {
	out := make([]bucketObject, 0, len(objs.Items))
	for _, item := range objs.Items {
		out = append(out, t.object(item))
	}
	return out
}

// object converts a single emulator object to its dashboard representation.
func (t *bucketTarget) object(item *storage.Object) bucketObject {
	obj := bucketObject{
		Key:          item.Name,
		Size:         int64(item.Size),
		ContentType:  item.ContentType,
		ETag:         item.Etag,
		StorageClass: item.StorageClass,
	}
	if updated, err := time.Parse(time.RFC3339Nano, item.Updated); err == nil {
		obj.Updated = updated
	}
	if t.bucket.Public {
		obj.PublicURL = t.objectURL(item.Name)
	}
	return obj
}

// objectURL returns the URL the given object is served from.
func (t *bucketTarget) objectURL(key string) string {
	// Percent-encode the key while leaving the path separators intact.
	escapedKey := (&url.URL{Path: key}).EscapedPath()
	return fmt.Sprintf("%s/%s/%s", t.publicBaseURL, url.PathEscape(t.bucket.Name), escapedKey)
}

// signedURL builds a time-limited download URL for the given object.
//
// The local emulator doesn't verify signatures, it only enforces the expiry encoded in
// the query parameters, so the signature here is a stand-in that makes the URL well-formed.
// Anyone who can reach the daemon can read the objects regardless.
func (t *bucketTarget) signedURL(key string, now time.Time, ttl time.Duration) string {
	const dateLayout = "20060102T150405Z"
	date := now.UTC().Format(dateLayout)

	// Tie the stand-in signature to the object and expiry so the URL is stable
	// for as long as it is valid.
	sum := sha256.Sum256(fmt.Appendf(nil, "%s/%s/%s/%d", t.bucket.Name, key, date, int(ttl.Seconds())))

	query := url.Values{
		"X-Goog-Algorithm":     {"GOOG4-RSA-SHA256"},
		"X-Goog-Credential":    {"dummy-sa@encore.local/" + now.UTC().Format("20060102") + "/auto/storage/goog4_request"},
		"X-Goog-Date":          {date},
		"X-Goog-Expires":       {fmt.Sprintf("%d", int(ttl.Seconds()))},
		"X-Goog-SignedHeaders": {"host"},
		"X-Goog-Signature":     {hex.EncodeToString(sum[:])},
	}
	return t.objectURL(key) + "?" + query.Encode()
}

// validateObjectKey rejects keys that don't address exactly one object.
//
// Object keys are opaque strings, but the local emulator turns them into file paths,
// which normalizes them: "a/../b" would silently address the object "b", and "." or
// "a/.." the bucket itself. Rejecting those keeps the dashboard from writing objects
// it can't then address under the name it used, and keeps the file and in-memory
// backends (which key a map, so they normalize nothing) in agreement.
func validateObjectKey(key string) error {
	if key == "" {
		return errors.New("missing object key")
	}
	if strings.HasPrefix(key, "/") {
		return errors.Newf("invalid object key %q: must not start with '/'", key)
	}
	if strings.Contains(key, `\`) {
		return errors.Newf(`invalid object key %q: must not contain '\'`, key)
	}
	for segment := range strings.SplitSeq(key, "/") {
		if segment == "." || segment == ".." {
			return errors.Newf("invalid object key %q: must not contain %q path segments", key, segment)
		}
	}
	return nil
}

func clampBucketPageSize(pageSize int) int {
	if pageSize <= 0 || pageSize > maxBucketPageSize {
		return maxBucketPageSize
	}
	return pageSize
}

// objectFilename returns the last path segment of a key, for use as a download filename.
func objectFilename(key string) string {
	return path.Base(strings.TrimSuffix(key, "/"))
}
