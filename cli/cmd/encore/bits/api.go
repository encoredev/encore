package bits

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/cockroachdb/errors"
)

type Bit struct {
	ID          int64
	Slug        string
	Title       string
	Description string
	GitRepo     string
	GitBranch   string
}

type ListResponse struct {
	Bits []*Bit
}

func List(ctx context.Context) ([]*Bit, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://automativity.encore.dev/bits", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		slurp, _ := io.ReadAll(resp.Body)
		return nil, errors.Newf("got status %d: %s", resp.StatusCode, slurp)
	}
	var data ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, errors.Wrap(err, "decode json response")
	}
	return data.Bits, nil
}

var errBitNotFound = errors.New("bit not found")

func Get(ctx context.Context, slug string) (*Bit, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://automativity.encore.dev/bits/"+url.PathEscape(slug), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, errBitNotFound
	} else if resp.StatusCode != 200 {
		slurp, _ := io.ReadAll(resp.Body)
		return nil, errors.Newf("got status %d: %s", resp.StatusCode, slurp)
	}
	var bit Bit
	if err := json.NewDecoder(resp.Body).Decode(&bit); err != nil {
		return nil, errors.Wrap(err, "decode json response")
	}
	return &bit, nil
}
