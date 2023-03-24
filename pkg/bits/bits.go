package bits

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/cockroachdb/errors"
)

type Bit struct {
	Slug        string
	Title       string
	Description string

	// GitHubTree is a URL to the GitHub tree for this bit,
	// in the format expected by github.ParseTree.
	GitHubTree string
}

// List lists available bits.
func List(ctx context.Context) ([]*Bit, error) {
	resp, err := http.Get("https://automativity.encore.dev/bits")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		slurp, _ := ioutil.ReadAll(resp.Body)
		return nil, errors.Newf("got status %d: %s", resp.StatusCode, slurp)
	}

	var data struct {
		Bits []*Bit
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, errors.Wrap(err, "decode json response")
	}
	return data.Bits, nil
}

// ErrNotFound is reported by Get if the bit with the given slug is not found.
var ErrNotFound = errors.New("bit not found")

// Get retrieves a bit by its slug.
func Get(ctx context.Context, slug string) (*Bit, error) {
	resp, err := http.Get("https://automativity.encore.dev/bits/" + url.PathEscape(slug))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, ErrNotFound
	} else if resp.StatusCode != 200 {
		slurp, _ := ioutil.ReadAll(resp.Body)
		return nil, errors.Newf("got status %d: %s", resp.StatusCode, slurp)
	}

	var bit Bit
	if err := json.NewDecoder(resp.Body).Decode(&bit); err != nil {
		return nil, errors.Wrap(err, "decode json response")
	}
	return &bit, nil
}
