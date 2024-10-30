package gcsemu

import (
	"compress/gzip"
	"io"
	"net/http"
)

// DrainRequestHandler wraps the given handler to drain the incoming request body on exit.
func DrainRequestHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			// Always drain and close the request body to properly free up the connection.
			// See https://groups.google.com/forum/#!topic/golang-nuts/pP3zyUlbT00
			_, _ = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()
		}()
		h(w, r)
	}
}

// GzipRequestHandler wraps the given handler to automatically decompress gzipped content.
func GzipRequestHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") == "gzip" {
			gzr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			r.Body = gzr
		}
		h(w, r)
	}
}
