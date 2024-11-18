package gcsemu

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"encr.dev/pkg/emulators/storage/gcsutil"
	"google.golang.org/api/storage/v1"
)

// Iterate over the file system to serve a GCS list-bucket request.
func (g *GcsEmu) makeBucketListResults(ctx context.Context, baseUrl HttpBaseUrl, w http.ResponseWriter, delimiter string, cursor string, prefix string, bucket string, maxResults int) {
	var errAbort = errors.New("sentinel error to abort walk")

	type item struct {
		filename string
		fInfo    os.FileInfo
	}
	var found []item
	var prefixes []string
	seenPrefixes := make(map[string]bool)

	dbgWalk := func(fmt string, args ...interface{}) {
		if g.verbose {
			g.log(nil, fmt, args...)
		}
	}

	moreResults := false
	count := 0
	err := g.store.Walk(ctx, bucket, func(ctx context.Context, filename string, fInfo os.FileInfo) error {
		dbgWalk("walk: %s", filename)

		// If we're beyond the prefix, we're completely done.
		if greaterThanPrefix(filename, prefix) {
			dbgWalk("%q > prefix=%q aborting", filename, prefix)
			return errAbort
		}

		// In the filesystem implementation, skip any directories strictly less than the cursor or prefix.
		if fInfo != nil && fInfo.IsDir() {
			if lessThanPrefix(filename, cursor) {
				dbgWalk("%q < cursor=%q skip dir", filename, cursor)
				return filepath.SkipDir
			}
			if lessThanPrefix(filename, prefix) {
				dbgWalk("%q < prefix=%q skip dir", filename, prefix)
				return filepath.SkipDir
			}
			return nil // keep going
		}

		// If the file is <= cursor, or < prefix, skip.
		if filename <= cursor {
			dbgWalk("%q <= cursor=%q skipping", filename, cursor)
			return nil
		}
		if !strings.HasPrefix(filename, prefix) {
			dbgWalk("%q < prefix=%q skipping", filename, prefix)
			return nil
		}

		if count >= maxResults {
			moreResults = true
			return errAbort
		}
		count++

		if delimiter != "" {
			// See if the filename (beyond the prefix) contains delimiter, if it does, don't record the item,
			// instead record the prefix (including the delimiter).
			withoutPrefix := strings.TrimPrefix(filename, prefix)
			delimiterPos := strings.Index(withoutPrefix, delimiter)
			if delimiterPos >= 0 {
				// Got a hit, reconstruct the item's prefix, including the trailing delimiter
				itemPrefix := filename[:len(prefix)+delimiterPos+len(delimiter)]
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
		if len(found) == 0 {
			if os.IsNotExist(err) {
				g.gapiError(w, http.StatusNotFound, fmt.Sprintf("%s not found", bucket))
			} else {
				g.gapiError(w, http.StatusInternalServerError, "failed to iterate: "+err.Error())
			}
			return
		}
		// return our partial results + the cursor so that the client can retry from this point
		g.log(nil, "failed to iterate")
	}

	// Resolve the found items.
	var items []*storage.Object
	for _, item := range found {
		if obj, err := g.store.ReadMeta(baseUrl, bucket, item.filename, item.fInfo); err != nil {
			// return our partial results + the cursor so that the client can retry from this point
			g.log(nil, "failed to resolve: %s", item.filename)
			break
		} else {
			items = append(items, obj)
		}
	}

	var nextPageToken = ""
	if moreResults && len(items) > 0 {
		lastItemName := items[len(items)-1].Name
		nextPageToken = gcsutil.EncodePageToken(lastItemName)
	}

	rsp := storage.Objects{
		Kind:          "storage#objects",
		NextPageToken: nextPageToken,
		Items:         items,
		Prefixes:      prefixes,
	}

	g.jsonRespond(w, &rsp)
}
