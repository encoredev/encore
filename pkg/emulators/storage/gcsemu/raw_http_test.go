package gcsemu

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	api "google.golang.org/api/storage/v1"
	"gotest.tools/v3/assert"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"strings"
	"testing"
)

func testRawHttp(t *testing.T, bh BucketHandle, httpClient *http.Client, url string) {
	const name = "gscemu-test3.txt"
	const name2 = "gscemu-test4.txt"
	const delName = "gscemu-test-deletion.txt"    // used for successful deletion
	const delName2 = "gscemu-test-deletion-2.txt" // used for not found deletion

	expectMetaGen := int64(1)
	tcs := []struct {
		name          string
		makeRequest   func(*testing.T) *http.Request
		checkResponse func(*testing.T, *http.Response)
	}{
		{
			name: "rawGetObject",
			makeRequest: func(t *testing.T) *http.Request {
				u := fmt.Sprintf("%s/download/storage/v1/b/%s/o/%s?alt=media", url, bh.Name, name)
				t.Log(u)
				req, err := http.NewRequest("GET", u, nil)
				assert.NilError(t, err)
				return req
			},
			checkResponse: func(t *testing.T, rsp *http.Response) {
				body, err := io.ReadAll(rsp.Body)
				assert.NilError(t, err)
				assert.Equal(t, http.StatusOK, rsp.StatusCode)
				assert.Equal(t, v1, string(body))
			},
		},
		{
			name: "rawGetMeta",
			makeRequest: func(t *testing.T) *http.Request {
				u := fmt.Sprintf("%s/storage/v1/b/%s/o/%s", url, bh.Name, name)
				t.Log(u)
				req, err := http.NewRequest("GET", u, nil)
				assert.NilError(t, err)
				return req
			},
			checkResponse: func(t *testing.T, rsp *http.Response) {
				body, err := io.ReadAll(rsp.Body)
				assert.NilError(t, err)
				assert.Equal(t, http.StatusOK, rsp.StatusCode)

				var attrs api.Object
				err = json.NewDecoder(bytes.NewReader(body)).Decode(&attrs)
				assert.NilError(t, err)
				assert.Equal(t, name, attrs.Name)
				assert.Equal(t, bh.Name, attrs.Bucket)
				assert.Equal(t, uint64(len(v1)), attrs.Size)
				assert.Equal(t, expectMetaGen, attrs.Metageneration)
			},
		},
		{
			name: "rawPatchMeta",
			makeRequest: func(t *testing.T) *http.Request {
				u := fmt.Sprintf("%s/storage/v1/b/%s/o/%s", url, bh.Name, name)
				t.Log(u)
				req, err := http.NewRequest("PATCH", u, strings.NewReader(`{"metadata": {"type": "tabby"}}`))
				assert.NilError(t, err)
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			checkResponse: func(t *testing.T, rsp *http.Response) {
				body, err := io.ReadAll(rsp.Body)
				assert.NilError(t, err)
				assert.Equal(t, http.StatusOK, rsp.StatusCode)

				expectMetaGen++

				var attrs api.Object
				err = json.NewDecoder(bytes.NewReader(body)).Decode(&attrs)
				assert.NilError(t, err)
				assert.Equal(t, name, attrs.Name)
				assert.Equal(t, bh.Name, attrs.Bucket)
				assert.Equal(t, uint64(len(v1)), attrs.Size)
				assert.Equal(t, expectMetaGen, attrs.Metageneration)
				assert.Equal(t, "tabby", attrs.Metadata["type"])
			},
		},
		{
			name: "rawDeleteObject-Success",
			makeRequest: func(t *testing.T) *http.Request {
				u := fmt.Sprintf("%s/storage/v1/b/%s/o/%s", url, bh.Name, delName)
				t.Log(u)
				req, err := http.NewRequest("DELETE", u, nil)
				assert.NilError(t, err)
				req.Header.Set("Content-Type", "text/plain")
				return req
			},
			checkResponse: func(t *testing.T, rsp *http.Response) {
				assert.Equal(t, http.StatusNoContent, rsp.StatusCode)
			},
		},
		{
			name: "rawDeleteObject-ObjectNotFound",
			makeRequest: func(t *testing.T) *http.Request {
				u := fmt.Sprintf("%s/storage/v1/b/%s/o/%s", url, bh.Name, delName2)
				t.Log(u)
				req, err := http.NewRequest("DELETE", u, nil)
				assert.NilError(t, err)
				req.Header.Set("Content-Type", "text/plain")
				return req
			},
			checkResponse: func(t *testing.T, rsp *http.Response) {
				assert.Equal(t, http.StatusNotFound, rsp.StatusCode)
			},
		},
		{
			name: "rawDeleteObject-BucketNotFound",
			makeRequest: func(t *testing.T) *http.Request {
				u := fmt.Sprintf("%s/storage/v1/b/%s/o/%s", url, invalidBucketName, delName)
				t.Log(u)
				req, err := http.NewRequest("DELETE", u, nil)
				assert.NilError(t, err)
				req.Header.Set("Content-Type", "text/plain")
				return req
			},
			checkResponse: func(t *testing.T, rsp *http.Response) {
				assert.Equal(t, http.StatusNotFound, rsp.StatusCode)
			},
		},
		{
			name: "rawDeleteBucket-BucketNotFound",
			makeRequest: func(t *testing.T) *http.Request {
				u := fmt.Sprintf("%s/storage/v1/b/%s", url, invalidBucketName)
				t.Log(u)
				req, err := http.NewRequest("DELETE", u, nil)
				assert.NilError(t, err)
				req.Header.Set("Content-Type", "text/plain")
				return req
			},
			checkResponse: func(t *testing.T, rsp *http.Response) {
				assert.Equal(t, http.StatusNotFound, rsp.StatusCode)
			},
		},
		{
			name: "rawUpload",
			makeRequest: func(t *testing.T) *http.Request {
				u := fmt.Sprintf("%s/upload/storage/v1/b/%s/o?uploadType=media&name=%s", url, bh.Name, name2)
				t.Log(u)
				req, err := http.NewRequest("POST", u, strings.NewReader(v2))
				assert.NilError(t, err)
				req.Header.Set("Content-Type", "text/plain")
				return req
			},
			checkResponse: func(t *testing.T, rsp *http.Response) {
				body, err := io.ReadAll(rsp.Body)
				assert.NilError(t, err)
				assert.Equal(t, http.StatusOK, rsp.StatusCode)

				var attrs api.Object
				err = json.NewDecoder(bytes.NewReader(body)).Decode(&attrs)
				assert.NilError(t, err)
				assert.Equal(t, name2, attrs.Name)
				assert.Equal(t, bh.Name, attrs.Bucket)
				assert.Equal(t, uint64(len(v2)), attrs.Size)
				assert.Equal(t, int64(1), attrs.Metageneration)
			},
		},
		{
			name: "publicUrl",
			makeRequest: func(t *testing.T) *http.Request {
				u := fmt.Sprintf("%s/%s/%s?alt=media", url, bh.Name, name)
				t.Log(u)
				req, err := http.NewRequest("GET", u, nil)
				assert.NilError(t, err)
				return req
			},
			checkResponse: func(t *testing.T, rsp *http.Response) {
				body, err := io.ReadAll(rsp.Body)
				assert.NilError(t, err)
				assert.Equal(t, http.StatusOK, rsp.StatusCode)
				assert.Equal(t, v1, string(body))
			},
		},
	}

	ctx := context.Background()
	oh := bh.Object(name)

	// Create the object 1.
	w := oh.NewWriter(ctx)
	assert.NilError(t, write(w, v1))

	// Make sure object 2 is not there.
	_ = bh.Object(name2).Delete(ctx)

	// batch setup
	// Create the object for successful deletion.
	w = bh.Object(delName).NewWriter(ctx)
	assert.NilError(t, write(w, v1))
	// Make sure object for not found deletion is not there.
	_ = bh.Object(delName2).Delete(ctx)

	// Run each test individually.
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := tc.makeRequest(t)
			rsp, err := httpClient.Do(req)
			assert.NilError(t, err)
			body, err := httputil.DumpResponse(rsp, true)
			assert.NilError(t, err)
			t.Log(string(body))
			tc.checkResponse(t, rsp)
		})
	}

	// batch setup again for batch deletion step
	// Create the object for successful deletion.
	w = bh.Object(delName).NewWriter(ctx)
	assert.NilError(t, write(w, v1))
	// Make sure object for not found deletion is not there.
	_ = bh.Object(delName2).Delete(ctx)

	// Batch requests don't support upload and download, only metadata stuff.
	t.Run("batch", func(t *testing.T) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)

		// Only use the [second, fifth] requests.
		batchTcs := tcs[1:6]
		for i, tc := range batchTcs {
			req := tc.makeRequest(t)
			req.Host = ""
			req.URL.Host = ""

			p, _ := w.CreatePart(textproto.MIMEHeader{
				"Content-Type":              []string{"application/http"},
				"Content-Transfer-Encoding": []string{"binary"},
				"Content-ID":                []string{fmt.Sprintf("<id+%d>", i)},
			})
			buf, err := httputil.DumpRequest(req, true)
			assert.NilError(t, err)
			_, _ = p.Write(buf)
		}
		_ = w.Close()

		// Compile the request
		req, err := http.NewRequest("POST", fmt.Sprintf("%s/batch/storage/v1", url), &buf)
		assert.NilError(t, err)
		req.Header.Set("Content-Type", "multipart/mixed; boundary="+w.Boundary())

		body, err := httputil.DumpRequest(req, true)
		assert.NilError(t, err)
		t.Log(string(body))

		rsp, err := httpClient.Do(req)
		assert.NilError(t, err)
		assert.Equal(t, http.StatusOK, rsp.StatusCode)

		body, err = httputil.DumpResponse(rsp, true)
		assert.NilError(t, err)
		t.Log(string(body))

		// decode the multipart response
		v := rsp.Header.Get("Content-type")
		assert.Check(t, v != "")
		d, params, err := mime.ParseMediaType(v)
		assert.NilError(t, err)
		assert.Equal(t, "multipart/mixed", d)
		boundary, ok := params["boundary"]
		assert.Check(t, ok)

		r := multipart.NewReader(rsp.Body, boundary)
		for i, tc := range batchTcs {
			part, err := r.NextPart()
			assert.NilError(t, err)
			assert.Equal(t, "application/http", part.Header.Get("Content-Type"))
			assert.Equal(t, fmt.Sprintf("<response-id+%d>", i), part.Header.Get("Content-ID"))
			b, err := io.ReadAll(part)
			assert.NilError(t, err)

			// Decode the buffer into an http.Response
			rsp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(b)), nil)
			assert.NilError(t, err)
			tc.checkResponse(t, rsp)
		}
	})
}
