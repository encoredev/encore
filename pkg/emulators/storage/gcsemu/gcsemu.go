// Package gcsemu implements a Google Cloud Storage emulator for development.
package gcsemu

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	cloudstorage "cloud.google.com/go/storage"
	"encr.dev/pkg/emulators/storage/gcsutil"
	"github.com/bluele/gcache"
	"google.golang.org/api/storage/v1"
)

const maybeNotImplementedErrorMsg = "This may be a valid request, but we haven't implemented it in gcsemu yet."

// Options configure the emulator.
type Options struct {
	// A storage layer to use; if nil, defaults to in-mem storage.
	Store Store

	// If true, log verbosely.
	Verbose bool

	// Optional log function. `err` will be `nil` for informational/debug messages.
	Log func(err error, fmt string, args ...interface{})
}

// GcsEmu is a Google Cloud Storage emulator for development.
type GcsEmu struct {
	// The directory which contains gcs emulation.
	store Store
	locks *gcsutil.TransientLockMap

	uploadIds gcache.Cache
	idCounter int32

	verbose bool
	log     func(err error, fmt string, args ...interface{})
}

// NewGcsEmu creates a new Google Cloud Storage emulator.
func NewGcsEmu(opts Options) *GcsEmu {
	if opts.Store == nil {
		opts.Store = NewMemStore()
	}
	if opts.Log == nil {
		opts.Log = func(_ error, _ string, _ ...interface{}) {}
	}
	return &GcsEmu{
		store:     opts.Store,
		locks:     gcsutil.NewTransientLockMap(),
		uploadIds: gcache.New(1024).LRU().Build(),
		verbose:   opts.Verbose,
		log:       opts.Log,
	}
}

func lockName(bucket string, filename string) string {
	return bucket + "/" + filename
}

// Register the emulator's HTTP handlers on the given mux.
func (g *GcsEmu) Register(mux *http.ServeMux) {
	mux.HandleFunc("/", DrainRequestHandler(GzipRequestHandler(g.Handler)))
	mux.HandleFunc("/batch/storage/v1", DrainRequestHandler(GzipRequestHandler(g.BatchHandler)))
}

// Handler handles emulated GCS http requests for "storage.googleapis.com".
func (g *GcsEmu) Handler(w http.ResponseWriter, r *http.Request) {
	baseUrl := dontNeedUrls
	{
		host := requestHost(r)
		if host != "" {
			// Prepend the proto.
			if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
				baseUrl = HttpBaseUrl("https://" + host + "/")
			} else {
				baseUrl = HttpBaseUrl("http://" + host + "/")
			}
		}
	}

	ctx := r.Context()
	p, ok := ParseGcsUrl(r.URL)
	if !ok {
		g.gapiError(w, http.StatusBadRequest, "unrecognized request")
		return
	}
	object := p.Object
	bucket := p.Bucket

	if err := r.ParseForm(); err != nil {
		g.gapiError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse form: %s", err))
		return
	}

	conds, err := parseConds(r.Form)
	if err != nil {
		g.gapiError(w, http.StatusBadRequest, err.Error())
		return
	}

	if g.verbose {
		if object == "" {
			g.log(nil, "%s request for bucket %q", r.Method, bucket)
		} else {
			g.log(nil, "%s request for bucket %q, object %q", r.Method, bucket, object)
		}
	}

	switch r.Method {
	case "DELETE":
		g.handleGcsDelete(ctx, w, bucket, object, conds)
	case "GET":
		if object == "" {
			if strings.HasSuffix(r.URL.Path, "/o") {
				g.handleGcsListBucket(ctx, baseUrl, w, r.URL.Query(), bucket)
			} else {
				g.handleGcsMetadataRequest(baseUrl, w, bucket, object)
			}
		} else {
			alt := r.URL.Query().Get("alt")
			if alt == "media" || (p.IsPublic && alt == "") {
				g.handleGcsMediaRequest(baseUrl, w, r.Header.Get("Accept-Encoding"), bucket, object)
			} else if alt == "json" || (!p.IsPublic && alt == "") {
				g.handleGcsMetadataRequest(baseUrl, w, bucket, object)
			} else {
				// should never happen?
				g.gapiError(w, http.StatusBadRequest, fmt.Sprintf("unsupported value for alt param to GET: %q\n%s", alt, maybeNotImplementedErrorMsg))
			}
		}
	case "PATCH":
		alt := r.URL.Query().Get("alt")
		if alt == "json" || r.Header.Get("Content-Type") == "application/json" {
			g.handleGcsUpdateMetadataRequest(ctx, baseUrl, w, r, bucket, object, conds)
		} else {
			// should never happen?
			g.gapiError(w, http.StatusBadRequest, fmt.Sprintf("unsupported value for alt param to PATCH: %q\n%s", alt, maybeNotImplementedErrorMsg))
		}
	case "POST":
		if bucket == "" {
			g.handleGcsNewBucket(ctx, w, r, conds)
		} else if object == "" {
			g.handleGcsNewObject(ctx, baseUrl, w, r, bucket, conds)
		} else if strings.Contains(object, "/compose") {
			// TODO: enforce other conditions outside of generation
			g.handleGcsCompose(ctx, baseUrl, w, r, bucket, object, conds)
		} else if strings.Contains(object, "/rewriteTo/") {
			g.handleGcsCopy(ctx, baseUrl, w, bucket, object)
		} else if r.Form.Get("upload_id") != "" {
			g.handleGcsNewObjectResume(ctx, baseUrl, w, r, r.Form.Get("upload_id"))
		} else {
			// unsupported method, or maybe should never happen
			g.gapiError(w, http.StatusBadRequest, fmt.Sprintf("unsupported POST request: %v\n%s", r.URL, maybeNotImplementedErrorMsg))
		}
	case "PUT":
		if r.Form.Get("upload_id") != "" {
			g.handleGcsNewObjectResume(ctx, baseUrl, w, r, r.Form.Get("upload_id"))
		} else {
			// unsupported method, or maybe should never happen
			g.gapiError(w, http.StatusBadRequest, fmt.Sprintf("unsupported PUT request: %v\n%s", r.URL, maybeNotImplementedErrorMsg))
		}
	default:
		g.gapiError(w, http.StatusMethodNotAllowed, "")
	}
}

func (g *GcsEmu) handleGcsCompose(ctx context.Context, baseUrl HttpBaseUrl, w http.ResponseWriter, r *http.Request, bucket, object string, conds cloudstorage.Conditions) {
	var req storage.ComposeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.gapiError(w, http.StatusBadRequest, "bad compose request")
		return
	}
	// Get the composed object name from the path
	parts := strings.Split(object, "/compose")
	if len(parts) != 2 {
		g.gapiError(w, http.StatusBadRequest, "bad compose request")
		return
	}
	dst := composeObj{
		filename: parts[0],
		conds:    conds,
	}

	srcs := make([]composeObj, len(req.SourceObjects))
	for i, sObj := range req.SourceObjects {
		var generationMatch int64
		if sObj.ObjectPreconditions != nil {
			generationMatch = sObj.ObjectPreconditions.IfGenerationMatch
		}
		srcs[i] = composeObj{
			filename: sObj.Name,
			conds: cloudstorage.Conditions{
				GenerationMatch: generationMatch,
			},
		}
	}
	var obj *storage.Object
	if err := g.locks.Run(ctx, lockName(bucket, dst.filename), func(_ context.Context) error {
		var err error
		obj, err = g.finishCompose(baseUrl, bucket, dst, srcs, req.Destination)
		return err
	}); err != nil {
		g.gapiError(w, httpStatusCodeOf(err), fmt.Sprintf("failed to compose objects: %s", err))
		return
	}
	g.jsonRespond(w, &obj)
}

func (g *GcsEmu) handleGcsListBucket(ctx context.Context, baseUrl HttpBaseUrl, w http.ResponseWriter, params url.Values, bucket string) {
	delimiter := params.Get("delimiter")
	prefix := params.Get("prefix")
	pageToken := params.Get("pageToken")

	var cursor string
	if pageToken != "" {
		lastFilename, err := gcsutil.DecodePageToken(pageToken)
		if err != nil {
			g.gapiError(w, http.StatusBadRequest, fmt.Sprintf("invalid pageToken parameter (failed to decode) %s: %s", pageToken, err))
			return
		}
		cursor = lastFilename
	}

	maxResults := 1000
	maxResultsStr := params.Get("maxResults")
	if maxResultsStr != "" {
		var err error
		maxResults, err = strconv.Atoi(maxResultsStr)
		if err != nil || maxResults < 1 {
			g.gapiError(w, http.StatusBadRequest, fmt.Sprintf("invalid maxResults parameter: %s", maxResultsStr))
			return
		}
	}

	g.makeBucketListResults(ctx, baseUrl, w, delimiter, cursor, prefix, bucket, maxResults)
}

func (g *GcsEmu) handleGcsDelete(ctx context.Context, w http.ResponseWriter, bucket string, filename string, conds cloudstorage.Conditions) {
	err := g.locks.Run(ctx, lockName(bucket, filename), func(ctx context.Context) error {
		// Find the existing file / meta.
		obj, err := g.store.GetMeta(dontNeedUrls, bucket, filename)
		if err != nil {
			return fmt.Errorf("failed to check existence of %s/%s: %w", bucket, filename, err)
		}

		if err := validateConds(obj, conds); err != nil {
			return err
		}

		if err := g.store.Delete(bucket, filename); err != nil {
			if os.IsNotExist(err) {
				return fmtErrorfCode(http.StatusNotFound, "%s/%s not found", bucket, filename)
			}
			return fmt.Errorf("failed to delete %s/%s: %w", bucket, filename, err)
		}

		return nil
	})
	if err != nil {
		g.gapiError(w, httpStatusCodeOf(err), err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (g *GcsEmu) handleGcsMediaRequest(baseUrl HttpBaseUrl, w http.ResponseWriter, acceptEncoding, bucket, filename string) {
	obj, contents, err := g.store.Get(baseUrl, bucket, filename)
	if err != nil {
		g.gapiError(w, http.StatusInternalServerError, fmt.Sprintf("failed to check existence of %s/%s: %s", bucket, filename, err))
		return
	}
	if obj == nil {
		g.gapiError(w, http.StatusNotFound, fmt.Sprintf("%s/%s not found", bucket, filename))
		return
	}

	w.Header().Set("Content-Type", obj.ContentType)
	w.Header().Set("X-Goog-Generation", strconv.FormatInt(obj.Generation, 10))
	w.Header().Set("X-Goog-Metageneration", strconv.FormatInt(obj.Metageneration, 10))
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Type, Content-Length, Content-Encoding, Date, X-Goog-Generation, X-Goog-Metageneration")
	w.Header().Set("Content-Disposition", obj.ContentDisposition)

	if obj.ContentEncoding == "gzip" {
		if strings.Contains(acceptEncoding, "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
		} else {
			// Uncompress on behalf of the client.
			buf := bytes.NewBuffer(contents)
			gzipReader, err := gzip.NewReader(buf)
			if err != nil {
				g.gapiError(w, http.StatusInternalServerError, fmt.Sprintf("failed to gunzip from %s/%s: %s", bucket, filename, err))
			}
			if _, err := io.Copy(w, gzipReader); err != nil {
				g.gapiError(w, http.StatusInternalServerError, fmt.Sprintf("failed to copy+gunzip from %s/%s: %s", bucket, filename, err))
			}
			if err := gzipReader.Close(); err != nil {
				g.gapiError(w, http.StatusInternalServerError, fmt.Sprintf("failed to copy+gunzip from %s/%s: %s", bucket, filename, err))
			}
			return
		}
	}

	// Just write the contents
	w.Header().Set("Content-Length", strconv.Itoa(len(contents)))
	if _, err := w.Write(contents); err != nil {
		g.gapiError(w, http.StatusInternalServerError, fmt.Sprintf("failed to copy from %s/%s: %s", bucket, filename, err))
	}
}

func (g *GcsEmu) handleGcsMetadataRequest(baseUrl HttpBaseUrl, w http.ResponseWriter, bucket string, filename string) {
	var obj interface{}
	var err error
	if filename == "" {
		var b *storage.Bucket
		b, err = g.store.GetBucketMeta(baseUrl, bucket)
		if b != nil {
			obj = b
		}
	} else {
		var o *storage.Object
		o, err = g.store.GetMeta(baseUrl, bucket, filename)
		if o != nil {
			obj = o
		}
	}

	if err != nil {
		g.gapiError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get meta for %s/%s: %s", bucket, filename, err))
		return
	}
	if obj == nil {
		g.gapiError(w, http.StatusNotFound, fmt.Sprintf("%s/%s not found", bucket, filename))
		return
	}
	g.jsonRespond(w, obj)
}

func (g *GcsEmu) handleGcsUpdateMetadataRequest(ctx context.Context, baseUrl HttpBaseUrl, w http.ResponseWriter, r *http.Request, bucket, filename string, conds cloudstorage.Conditions) {
	var obj *storage.Object
	err := g.locks.Run(ctx, lockName(bucket, filename), func(ctx context.Context) error {
		// Find the existing file / meta.
		var err error
		obj, err = g.store.GetMeta(baseUrl, bucket, filename)
		if err != nil {
			return fmt.Errorf("failed to check existence of %s/%s: %w", bucket, filename, err)
		}

		if obj == nil {
			return nil
		}

		if err := validateConds(obj, conds); err != nil {
			return err
		}

		// Update via json decode.
		metagen := obj.Metageneration
		err = json.NewDecoder(r.Body).Decode(&obj)
		if err != nil {
			return fmtErrorfCode(http.StatusBadRequest, "failed to parse request: %w", err)
		}

		if err := g.store.UpdateMeta(bucket, filename, obj, metagen+1); err != nil {
			return fmt.Errorf("failed to update attrs of %s/%s: %w", bucket, filename, err)
		}

		return nil
	})

	if err != nil {
		g.gapiError(w, httpStatusCodeOf(err), err.Error())
		return
	}
	if obj == nil {
		g.gapiError(w, http.StatusNotFound, fmt.Sprintf("%s/%s not found", bucket, filename))
		return
	}

	// Respond with the updated metadata.
	obj, err = g.store.GetMeta(baseUrl, bucket, filename)
	if err != nil {
		g.gapiError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get meta for %s/%s: %s", bucket, filename, err))
		return
	}
	g.jsonRespond(w, obj)
}

func (g *GcsEmu) handleGcsCopy(ctx context.Context, baseUrl HttpBaseUrl, w http.ResponseWriter, b1 string, objectPaths string) {
	// TODO(dk): this operation supports conditionals and metadata rewriting, but the emulator implementation currently does not.
	// See https://cloud.google.com/storage/docs/json_api/v1/objects/rewrite
	parts := strings.Split(objectPaths, "/rewriteTo/b/")
	// Copy is implemented using the Rewrite API, with object strings of format /o/sourceObject/rewriteTo/b/destinationBucket/o/destinationObject
	if len(parts) != 2 {
		g.gapiError(w, http.StatusBadRequest, fmt.Sprintf("Bad rewrite request format: %s", objectPaths))
		return
	}
	f1 := parts[0]
	destParts := strings.Split(parts[1], "/o/")
	if len(parts) != 2 {
		g.gapiError(w, http.StatusBadRequest, fmt.Sprintf("Bad rewrite request, expected object/file split: %s", parts[1]))
		return
	}
	b2 := destParts[0]
	f2 := destParts[1]

	// Must lock the destination object.
	var obj *storage.Object
	err := g.locks.Run(ctx, lockName(b2, f2), func(ctx context.Context) error {
		if ok, err := g.store.Copy(b1, f1, b2, f2); err != nil {
			return err
		} else if !ok {
			return nil // file missing
		} else {
			obj, err = g.store.GetMeta(baseUrl, b2, f2)
			return err
		}
	})
	if err != nil {
		g.gapiError(w, httpStatusCodeOf(err), fmt.Sprintf("failed to copy: %s", err))
		return
	}
	if obj == nil {
		g.gapiError(w, http.StatusNotFound, fmt.Sprintf("%s not found", b1+"/"+f1))
		return
	}

	rr := storage.RewriteResponse{
		Kind:                "storage#rewriteResponse",
		TotalBytesRewritten: int64(obj.Size),
		ObjectSize:          int64(obj.Size),
		Done:                true,
		RewriteToken:        "-not-implemented-",
		Resource:            obj,
	}

	g.jsonRespond(w, &rr)
}

type uploadData struct {
	Object storage.Object
	Conds  cloudstorage.Conditions
	data   []byte
}

func (g *GcsEmu) handleGcsNewBucket(ctx context.Context, w http.ResponseWriter, r *http.Request, _ cloudstorage.Conditions) {
	var bucket storage.Bucket
	if err := json.NewDecoder(r.Body).Decode(&bucket); err != nil {
		g.gapiError(w, http.StatusBadRequest, "failed to parse body as json")
		return
	}
	bucketName := bucket.Name

	err := g.locks.Run(ctx, lockName(bucketName, ""), func(ctx context.Context) error {
		if err := g.store.CreateBucket(bucketName); err != nil {
			return fmt.Errorf("could not create bucket %s: %w", bucketName, err)
		}
		return nil
	})

	if err != nil {
		g.gapiError(w, httpStatusCodeOf(err), err.Error())
		return
	}

	g.jsonRespond(w, bucket)
}

func (g *GcsEmu) handleGcsNewObject(ctx context.Context, baseUrl HttpBaseUrl, w http.ResponseWriter, r *http.Request, bucket string, conds cloudstorage.Conditions) {
	switch r.Form.Get("uploadType") {
	case "media":
		// simple upload
		name := r.Form.Get("name")
		if name == "" {
			g.gapiError(w, http.StatusBadRequest, "missing object name")
			return
		}

		contents, err := io.ReadAll(r.Body)
		if err != nil {
			g.gapiError(w, http.StatusBadRequest, "failed to read body")
			return
		}

		obj := &storage.Object{
			Bucket:      bucket,
			ContentType: r.Header.Get("Content-Type"),
			Name:        name,
			Size:        uint64(len(contents)),
		}

		meta, err := g.finishUpload(ctx, baseUrl, obj, contents, bucket, conds)
		if err != nil {
			g.gapiError(w, httpStatusCodeOf(err), err.Error())
			return
		}

		w.Header().Set("x-goog-generation", strconv.FormatInt(meta.Generation, 10))
		w.Header().Set("X-Goog-Metageneration", strconv.FormatInt(meta.Metageneration, 10))
		g.jsonRespond(w, meta)
		return
	case "resumable":
		var obj storage.Object
		if err := json.NewDecoder(r.Body).Decode(&obj); err != nil {
			g.gapiError(w, http.StatusBadRequest, "failed to parse body as json")
			return
		}
		obj.Bucket = bucket

		nextId := atomic.AddInt32(&g.idCounter, 1)
		id := strconv.Itoa(int(nextId))
		_ = g.uploadIds.Set(id, &uploadData{
			Object: obj,
			Conds:  conds,
		})

		w.Header().Set("Location", ObjectUrl(baseUrl, bucket, obj.Name)+"?upload_id="+id)
		w.Header().Set("Content-Type", obj.ContentType)
		w.WriteHeader(http.StatusCreated)
		return
	case "multipart":
		obj, contents, err := readMultipartInsert(r)
		if err != nil {
			g.gapiError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse request: %s", err))
			return
		}

		meta, err := g.finishUpload(ctx, baseUrl, obj, contents, bucket, conds)
		if err != nil {
			g.gapiError(w, httpStatusCodeOf(err), err.Error())
			return
		}

		w.Header().Set("x-goog-generation", strconv.FormatInt(meta.Generation, 10))
		w.Header().Set("X-Goog-Metageneration", strconv.FormatInt(meta.Metageneration, 10))
		g.jsonRespond(w, meta)
		return
	default:
		// TODO
		g.gapiError(w, http.StatusNotImplemented, "not yet implemented")
		return

	}
}

func (g *GcsEmu) handleGcsNewObjectResume(ctx context.Context, baseUrl HttpBaseUrl, w http.ResponseWriter, r *http.Request, id string) {
	found, err := g.uploadIds.GetIFPresent(id)
	if err != nil {
		g.gapiError(w, http.StatusInternalServerError, fmt.Sprintf("unexpected error: %s", err))
		return
	}
	if found == nil {
		g.gapiError(w, http.StatusNotFound, "no such id")
		return
	}

	u := found.(*uploadData)

	contents, err := io.ReadAll(r.Body)
	if err != nil {
		g.gapiError(w, http.StatusBadRequest, fmt.Sprintf("failed to ready body: %s", err))
		return
	}

	contentRange := r.Header.Get("Content-Range")
	if contentRange == "" {
		g.gapiError(w, http.StatusBadRequest, "expected Content-Range")
		return
	}

	// Parse the content range
	byteRange := parseByteRange(contentRange)
	if byteRange == nil {
		g.gapiError(w, http.StatusBadRequest, "malformed Content-Range header")
		return
	}

	if byteRange.lo == -1 && len(contents) != 0 || byteRange.lo != -1 && len(contents) != int(byteRange.hi+1-byteRange.lo) {
		g.gapiError(w, http.StatusBadRequest, fmt.Sprintf("Content-Range does not match content size: range=%v, len=%v", contentRange, len(contents)))
		return
	}

	if len(u.data) < int(byteRange.lo) {
		g.gapiError(w, http.StatusBadRequest, "missing content")
		return
	}

	// Apply the content to our stored data.
	if byteRange.lo != -1 {
		u.data = u.data[:byteRange.lo] // truncate a previous write if we've seen this range before
	}
	u.data = append(u.data, contents...)

	// Are we done?
	if byteRange.sz < 0 || len(u.data) < int(byteRange.sz) {
		// Not finished; save the contents and tell the client to resume.
		w.Header().Set("Range", fmt.Sprintf("bytes=0-%d", len(u.data)-1))
		w.Header().Set("Content-Type", u.Object.ContentType)
		if r.Header.Get("X-Guploader-No-308") == "yes" {
			w.Header().Set("X-Http-Status-Code-Override", "308")
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusPermanentRedirect)
		}
		return
	}

	// Done
	meta, err := g.finishUpload(ctx, baseUrl, &u.Object, u.data, u.Object.Bucket, u.Conds)
	if err != nil {
		g.gapiError(w, httpStatusCodeOf(err), err.Error())
		return
	}

	g.uploadIds.Remove(id)
	w.Header().Set("x-goog-generation", strconv.FormatInt(meta.Generation, 10))
	w.Header().Set("X-Goog-Metageneration", strconv.FormatInt(meta.Metageneration, 10))
	g.jsonRespond(w, meta)
}

func (g *GcsEmu) finishUpload(ctx context.Context, baseUrl HttpBaseUrl, obj *storage.Object, contents []byte, bucket string, conds cloudstorage.Conditions) (*storage.Object, error) {
	filename := obj.Name
	bHash := md5.Sum(contents)
	contentHash := bHash[:]
	md5Hash := base64.StdEncoding.EncodeToString(contentHash)
	if obj.Md5Hash != "" {
		h, err := base64.StdEncoding.DecodeString(obj.Md5Hash)
		if err != nil {
			return nil, fmtErrorfCode(http.StatusBadRequest, "not a valid md5 hash: %w", err)
		}
		if !bytes.Equal(contentHash, h) {
			return nil, fmtErrorfCode(http.StatusBadRequest, "md5 hash %s != expected %s", obj.Md5Hash, md5Hash)
		}
	}
	obj.Md5Hash = md5Hash
	obj.Etag = strconv.Quote(md5Hash)

	err := g.locks.Run(ctx, lockName(bucket, filename), func(ctx context.Context) error {
		// Find the existing file / meta.
		existing, err := g.store.GetMeta(baseUrl, bucket, filename)
		if err != nil {
			return fmt.Errorf("failed to check existence of %s/%s: %w", bucket, filename, err)
		}

		if err := validateConds(existing, conds); err != nil {
			return err
		}

		if existing != nil {
			obj.TimeCreated = existing.TimeCreated
		}

		if err := g.store.Add(bucket, filename, contents, obj); err != nil {
			return fmt.Errorf("failed to create %s/%s: %w", bucket, filename, err)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// respond with object metadata
	meta, err := g.store.GetMeta(baseUrl, bucket, filename)
	if err != nil {
		return nil, fmt.Errorf("failed to get meta for %s/%s: %w", bucket, filename, err)
	}

	meta.Id = fmt.Sprintf("%s/%s/%d", bucket, filename, meta.Generation)
	return meta, nil
}

// Returns true if item is strictly greater than anything that begins with prefix
func greaterThanPrefix(item string, prefix string) bool {
	if len(item) < len(prefix) {
		return item > prefix
	}
	return item[:len(prefix)] > prefix
}

// Returns true if item is strictly less than anything that begins with prefix
func lessThanPrefix(item string, prefix string) bool {
	if len(item) < len(prefix) {
		return item < prefix[:len(item)]
	}
	return item < prefix
}

var (
	emptyConds        = cloudstorage.Conditions{}
	doesNotExistConds = cloudstorage.Conditions{DoesNotExist: true}
)

func validateConds(obj *storage.Object, cond cloudstorage.Conditions) error {
	if obj == nil {
		// The only way a nil object can succeed is if the conds are exactly equal to empty or doesNotExist
		if cond == emptyConds || cond == doesNotExistConds {
			return nil
		}
		return fmtErrorfCode(http.StatusPreconditionFailed, "precondition failed")
	}

	// obj != nil from here on

	if cond.DoesNotExist {
		return fmtErrorfCode(http.StatusPreconditionFailed, "precondition failed")
	}

	if cond.GenerationMatch != 0 && obj.Generation != cond.GenerationMatch {
		return fmtErrorfCode(http.StatusPreconditionFailed, "precondition failed")
	}

	if cond.GenerationNotMatch != 0 && obj.Generation == cond.GenerationNotMatch {
		// not-match failures use a different code
		return fmtErrorfCode(http.StatusNotModified, "precondition failed")
	}

	if cond.MetagenerationMatch != 0 && obj.Metageneration != cond.MetagenerationMatch {
		return fmtErrorfCode(http.StatusPreconditionFailed, "precondition failed")
	}

	if cond.MetagenerationNotMatch != 0 && obj.Metageneration == cond.MetagenerationNotMatch {
		// not-match failures use a different code
		return fmtErrorfCode(http.StatusNotModified, "precondition failed")
	}

	return nil
}

func parseConds(vals url.Values) (cloudstorage.Conditions, error) {
	var ret cloudstorage.Conditions
	for i, e := range []struct {
		paramName string
		ref       *int64
	}{
		{"ifGenerationMatch", &ret.GenerationMatch},
		{"ifGenerationNotMatch", &ret.GenerationNotMatch},
		{"ifMetagenerationMatch", &ret.MetagenerationMatch},
		{"ifMetagenerationNotMatch", &ret.MetagenerationNotMatch},
	} {
		v := vals.Get(e.paramName)
		if v == "" {
			continue
		}
		val, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return ret, fmt.Errorf("failed to parse %s=%s: %w", e.paramName, v, err)
		}
		*e.ref = val
		if i == 0 {
			// Special case
			ret.DoesNotExist = val == 0
		}
	}

	return ret, nil
}

const (
	gcsMaxComposeSources = 32
)

func (g *GcsEmu) finishCompose(baseUrl HttpBaseUrl, bucket string, dst composeObj, srcs []composeObj, meta *storage.Object) (*storage.Object, error) {
	if len(srcs) > gcsMaxComposeSources {
		return nil, fmtErrorfCode(http.StatusBadRequest, "too many sources")
	}

	// TODO: consider moving this to disk to handle very large compose operations
	var data []byte
	metas := make([]*storage.Object, len(srcs))
	for i, src := range srcs {
		meta, contents, err := g.store.Get(baseUrl, bucket, src.filename)
		if err != nil {
			return nil, fmt.Errorf("failed to get object %s: %w", src.filename, err)
		}
		if meta == nil {
			return nil, fmtErrorfCode(http.StatusNotFound, "no such source object %s", src.filename)
		}
		if err := validateConds(meta, src.conds); err != nil {
			return nil, err
		}
		data = append(data, contents...)
		metas[i] = meta
	}

	for _, m := range metas {
		meta.ComponentCount += m.ComponentCount
	}
	// composite objects do not have an MD5 hash (https://cloud.google.com/storage/docs/composite-objects)
	meta.Md5Hash = ""

	dstMeta, err := g.store.GetMeta(baseUrl, bucket, dst.filename)
	if err != nil {
		return nil, fmt.Errorf("failed to get object %s: %w", dst.filename, err)
	}
	if err := validateConds(dstMeta, dst.conds); err != nil {
		return nil, err
	}
	if dstMeta != nil {
		meta.TimeCreated = dstMeta.TimeCreated
	}
	if err := g.store.Add(bucket, dst.filename, data, meta); err != nil {
		return nil, fmt.Errorf("failed to add new file: %w", err)
	}
	return g.store.GetMeta(baseUrl, bucket, dst.filename)
}

// InitBucket creates the given bucket directly.
func (g *GcsEmu) InitBucket(bucketName string) error {
	return g.locks.Run(context.Background(), lockName(bucketName, ""), func(ctx context.Context) error {
		if err := g.store.CreateBucket(bucketName); err != nil {
			return fmt.Errorf("could not create bucket: %s: %w", bucketName, err)
		}
		return nil
	})
}
