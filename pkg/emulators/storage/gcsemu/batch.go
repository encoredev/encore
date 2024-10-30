package gcsemu

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
)

// BatchHandler handles emulated GCS http requests for "storage.googleapis.com/batch/storage/v1".
func (g *GcsEmu) BatchHandler(w http.ResponseWriter, r *http.Request) {
	// First parse the entire incoming message.
	reader, err := r.MultipartReader()
	if err != nil {
		g.gapiError(w, httpStatusCodeOf(err), err.Error())
		return
	}

	var reqs []*http.Request
	var contentIds []string
	for i := 0; true; i++ {
		part, err := reader.NextPart()
		if err == io.EOF {
			break // done
		} else if err != nil {
			g.gapiError(w, http.StatusBadRequest, err.Error())
			return
		}

		if ct := part.Header.Get("Content-Type"); ct != "application/http" {
			g.gapiError(w, http.StatusBadRequest, fmt.Sprintf("Content-Type: want=application/http, got=%s", ct))
			return
		}

		contentId := part.Header.Get("Content-ID")

		content, err := io.ReadAll(part)
		_ = part.Close()
		if err != nil {
			g.gapiError(w, http.StatusBadRequest, fmt.Sprintf("part=%d, Content-ID=%s: read error %v", i, contentId, err))
			return
		}

		newReader := bufio.NewReader(bytes.NewReader(content))
		req, err := http.ReadRequest(newReader)
		if err != nil {
			g.gapiError(w, http.StatusBadRequest, fmt.Sprintf("part=%d, Content-ID=%s: unable to parse request %v", i, contentId, err))
			return
		}
		// Any remaining bytes are the body.
		rem, _ := io.ReadAll(newReader)
		if len(rem) > 0 {
			req.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(rem)), nil
			}
			req.Body, _ = req.GetBody()
		}
		if cte := part.Header.Get("Content-Transfer-Encoding"); cte != "" {
			req.Header.Set("Transfer-Encoding", cte)
		}
		// encoded requests don't include a host, so patch it up from the incoming request
		req.Host = r.Host
		reqs = append(reqs, req)
		contentIds = append(contentIds, contentId)
	}

	// At this point, we can respond with a 200.
	mw := multipart.NewWriter(w)
	w.Header().Set("Content-Type", "multipart/mixed; boundary="+mw.Boundary())
	w.WriteHeader(http.StatusOK)

	// run each request
	for i := range reqs {
		req, contentId := reqs[i], contentIds[i]

		rw := httptest.NewRecorder()
		g.Handler(rw, req)
		rsp := rw.Result()
		rsp.ContentLength = int64(rw.Body.Len())

		partHeaders := textproto.MIMEHeader{}
		partHeaders.Set("Content-Type", "application/http")
		if contentId != "" {
			if contentId[0] == '<' {
				contentId = "<response-" + contentId[1:]
			} else {
				contentId = "response-" + contentId
			}
			partHeaders.Set("Content-ID", contentId)
		}

		pw, _ := mw.CreatePart(partHeaders)
		if err := rsp.Write(pw); err != nil {
			g.log(err, "failed to write")
			return
		}
	}

	if err := mw.Close(); err != nil {
		g.log(err, "failed to close")
		return
	}
}
