// Package github provides utilities for interacting with GitHub repositories.
package github

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
)

// Tree contains information about a (sub-)tree in a GitHub repository.
type Tree struct {
	Owner  string // GitHub owner (user or organization)
	Repo   string // repository name
	Branch string // branch name
	Path   string // path to subtree ("." for whole project)
}

// Name reports a suitable name of the top-level directory in the tree.
// It defaults to the repository name, unless a Path is given
// in which case it is the last component of the path.
func (t *Tree) Name() string {
	if base := path.Base(t.Path); base != "." {
		return base
	}
	return t.Repo
}

// ParseTree parses a GitHub repository URL into a Tree.
//
// Valid URLs are:
// - github.com/owner/repo
// - github.com/owner/repo/tree/<branch>
// - github.com/owner/repo/tree/<branch>/<path>
//
// If the URL does not contain a branch, the default branch is queried
// using GitHub's API.
func ParseTree(ctx context.Context, s string) (*Tree, error) {
	switch {
	case strings.HasPrefix(s, "http"):
		// Already an URL; do nothing
	case strings.HasPrefix(s, "github.com"):
		// Assume a URL without the scheme
		s = "https://" + s
	}

	u, err := url.Parse(s)
	if err != nil {
		return nil, errors.Wrap(err, "invalid tree string")
	}

	if u.Host != "github.com" {
		return nil, errors.Newf("url host must be github.com, not %q", u.Host)
	}

	// Path must be one of:
	// "/owner/repo"
	// "/owner/repo/tree/<branch>"
	// "/owner/repo/tree/<branch>/path"
	parts := strings.SplitN(u.Path, "/", 6)
	switch {
	case len(parts) == 3: // "/owner/repo"
		owner, repo := parts[1], parts[2]
		// Check the default branch
		var resp struct {
			DefaultBranch string `json:"default_branch"`
		}
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, errors.Wrap(err, "lookup default branch")
		} else if err := slurpJSON(req, &resp); err != nil {
			return nil, errors.Wrap(err, "lookup default branch")
		}
		return &Tree{
			Owner:  owner,
			Repo:   repo,
			Branch: resp.DefaultBranch,
			Path:   ".",
		}, nil
	case len(parts) >= 5: // "/owner/repo"
		owner, repo, t, branch := parts[1], parts[2], parts[3], parts[4]
		p := "."
		if len(parts) == 6 {
			p = parts[5]
		}
		if t != "tree" {
			return nil, errors.Newf("invalid url: %s", u)
		}
		return &Tree{
			Owner:  owner,
			Repo:   repo,
			Branch: branch,
			Path:   p,
		}, nil
	default:
		return nil, errors.Newf("unsupported url: %s", u)
	}
}

func slurpJSON(req *http.Request, respData any) error {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "send request")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return errors.Newf("got non-200 response: %s: %s", resp.Status, body)
	}
	if err := json.NewDecoder(resp.Body).Decode(respData); err != nil {
		return errors.Wrap(err, "decode response")
	}
	return nil
}

var ErrEmptyTree = errors.New("empty tree")

// ExtractTree downloads a (sub-)tree from a GitHub repository and writes it to dst.
func ExtractTree(ctx context.Context, tree *Tree, dst string) error {
	url := fmt.Sprintf("https://codeload.github.com/%s/%s/tar.gz/%s", tree.Owner, tree.Repo, tree.Branch)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return errors.Wrap(err, "create request")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "send request")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.Newf("GET %s: got non-200 response: %s", url, resp.Status)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return errors.Wrap(err, "read gzip response")
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	prefix := path.Join(tree.Repo+"-"+tree.Branch, tree.Path)
	prefix += "/"
	files := 0

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			if files == 0 {
				return ErrEmptyTree
			}
			return nil
		} else if err != nil {
			return errors.Wrap(err, "read repository data")
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		if p := path.Clean(hdr.Name); strings.HasPrefix(p, prefix) {
			files++
			p = p[len(prefix):]
			filePath := filepath.Join(dst, filepath.FromSlash(p))
			if err := createFile(tr, filePath); err != nil {
				return errors.Wrapf(err, "create %s", p)
			}
		}
	}
}

// createFile creates the given file, creating any non-existent parent directories
// in the process. It returns an error if the file already exists.
func createFile(src io.Reader, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, src)
	if err2 := f.Close(); err == nil {
		err = err2
	}
	return err
}
