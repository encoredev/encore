package gcsemu

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"encr.dev/pkg/emulators/storage/gcsutil"
	"google.golang.org/api/storage/v1"
)

// MaxResults is the most entries a single list page holds, and the number returned
// when the caller asks for no limit. Cloud Storage documents both roles for the same
// number: "The service will use this parameter or 1,000 items, whichever is smaller",
// with a default of 1000. A larger request is capped rather than rejected.
const MaxResults = 1000

// ListOptions are the options for listing the objects in a bucket.
type ListOptions struct {
	// Prefix restricts the results to objects whose name begins with Prefix.
	Prefix string

	// Delimiter, when non-empty, rolls objects whose name contains Delimiter
	// after Prefix up into the returned Prefixes instead of returning them as objects.
	Delimiter string

	// Cursor is an exclusive lower bound on the object names to return.
	// It is the object name encoded in the caller's page token; see gcsutil.DecodePageToken.
	Cursor string

	// MatchGlob, when non-empty, restricts the results to objects whose full name
	// matches the given glob pattern. See compileGlob for the supported syntax.
	MatchGlob string

	// MaxResults caps the number of objects and rolled-up prefixes returned.
	// If <= 0 or above MaxResults, MaxResults is used.
	MaxResults int
}

// ListObjects lists the objects in the given bucket.
//
// If the bucket does not exist it returns an error satisfying os.IsNotExist.
func (g *GcsEmu) ListObjects(ctx context.Context, baseUrl HttpBaseUrl, bucket string, opts ListOptions) (*storage.Objects, error) {
	var errAbort = errors.New("sentinel error to abort walk")

	maxResults := opts.MaxResults
	if maxResults <= 0 || maxResults > MaxResults {
		maxResults = MaxResults
	}

	// Cloud Storage requires that a glob search either roll results up by path or not
	// at all: "delimiter must be either excluded or set to '/' in requests that use
	// the matchGlob parameter".
	if opts.MatchGlob != "" && opts.Delimiter != "" && opts.Delimiter != "/" {
		return nil, fmtErrorfCode(http.StatusBadRequest,
			"invalid delimiter parameter: must be unset or \"/\" when matchGlob is used")
	}

	var glob *regexp.Regexp
	if opts.MatchGlob != "" {
		var err error
		glob, err = compileGlob(opts.MatchGlob)
		if err != nil {
			return nil, fmtErrorfCode(http.StatusBadRequest, "invalid matchGlob parameter: %s", err)
		}
	}

	type item struct {
		filename string
		fInfo    os.FileInfo
	}
	var found []item
	var prefixes []string
	seenPrefixes := make(map[string]bool)

	dbgWalk := func(fmt string, args ...any) {
		if g.verbose {
			g.log(nil, fmt, args...)
		}
	}

	// lastMatch is the name of the last object accepted by the filters, whether it was
	// returned as an object or rolled up into a prefix. It seeds the next page token.
	var lastMatch string
	moreResults := false
	count := 0
	err := g.store.Walk(ctx, bucket, func(ctx context.Context, filename string, fInfo os.FileInfo) error {
		dbgWalk("walk: %s", filename)

		// A glob can force us to scan the whole bucket, so make sure we stay cancellable.
		if err := ctx.Err(); err != nil {
			return err
		}

		// If we're beyond the prefix, we're completely done.
		if greaterThanPrefix(filename, opts.Prefix) {
			dbgWalk("%q > prefix=%q aborting", filename, opts.Prefix)
			return errAbort
		}

		// In the filesystem implementation, skip any directories strictly less than the cursor or prefix.
		if fInfo != nil && fInfo.IsDir() {
			if lessThanPrefix(filename, opts.Cursor) {
				dbgWalk("%q < cursor=%q skip dir", filename, opts.Cursor)
				return filepath.SkipDir
			}
			if lessThanPrefix(filename, opts.Prefix) {
				dbgWalk("%q < prefix=%q skip dir", filename, opts.Prefix)
				return filepath.SkipDir
			}
			// Directories are not objects, so they are never reported. A folder that
			// was explicitly created has a placeholder object inside it, which the
			// store reports under the folder's own name.
			return nil // keep going
		}

		// If the file is <= cursor, or < prefix, skip.
		if filename <= opts.Cursor {
			dbgWalk("%q <= cursor=%q skipping", filename, opts.Cursor)
			return nil
		}
		if !strings.HasPrefix(filename, opts.Prefix) {
			dbgWalk("%q < prefix=%q skipping", filename, opts.Prefix)
			return nil
		}

		// Non-matching objects don't consume the page budget, so that a sparse
		// glob still fills a page instead of returning mostly-empty ones.
		if glob != nil && !glob.MatchString(filename) {
			dbgWalk("%q does not match glob=%q skipping", filename, opts.MatchGlob)
			return nil
		}

		if count >= maxResults {
			moreResults = true
			return errAbort
		}
		count++
		lastMatch = filename

		if opts.Delimiter != "" {
			// See if the filename (beyond the prefix) contains delimiter, if it does, don't record the item,
			// instead record the prefix (including the delimiter).
			withoutPrefix := strings.TrimPrefix(filename, opts.Prefix)
			delimiterPos := strings.Index(withoutPrefix, opts.Delimiter)
			if delimiterPos >= 0 {
				// Got a hit, reconstruct the item's prefix, including the trailing delimiter
				itemPrefix := filename[:len(opts.Prefix)+delimiterPos+len(opts.Delimiter)]
				if !seenPrefixes[itemPrefix] {
					seenPrefixes[itemPrefix] = true
					prefixes = append(prefixes, itemPrefix)
				}
				return nil
			}
		}

		found = append(found, item{
			filename: filename,
			fInfo:    fInfo,
		})
		return nil
	})
	// Sentinel error is not an error
	if err == errAbort {
		err = nil
	}
	if err != nil {
		return nil, err
	}

	// Resolve the found items.
	var items []*storage.Object
	for _, item := range found {
		obj, err := g.store.ReadMeta(baseUrl, bucket, item.filename, item.fInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve %s: %w", item.filename, err)
		}
		items = append(items, obj)
	}

	var nextPageToken = ""
	if moreResults && lastMatch != "" {
		nextPageToken = gcsutil.EncodePageToken(lastMatch)
	}

	return &storage.Objects{
		Kind:          "storage#objects",
		NextPageToken: nextPageToken,
		Items:         items,
		Prefixes:      prefixes,
	}, nil
}

// Iterate over the file system to serve a GCS list-bucket request.
func (g *GcsEmu) makeBucketListResults(ctx context.Context, baseUrl HttpBaseUrl, w http.ResponseWriter, opts ListOptions, bucket string) {
	rsp, err := g.ListObjects(ctx, baseUrl, bucket, opts)
	if err != nil {
		if os.IsNotExist(err) {
			g.gapiError(w, http.StatusNotFound, fmt.Sprintf("%s not found", bucket))
		} else if code := httpStatusCodeOf(err); code != 0 {
			g.gapiError(w, code, err.Error())
		} else {
			g.gapiError(w, http.StatusInternalServerError, "failed to iterate: "+err.Error())
		}
		return
	}

	g.jsonRespond(w, rsp)
}
