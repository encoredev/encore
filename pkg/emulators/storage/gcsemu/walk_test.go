package gcsemu

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"google.golang.org/api/storage/v1"
)

func names(objs *storage.Objects) []string {
	var out []string
	for _, item := range objs.Items {
		out = append(out, item.Name)
	}
	return out
}

func TestListObjectsMatchGlob(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(t.TempDir())
	emu := NewGcsEmu(Options{Store: store})

	for _, key := range []string{"a.txt", "img/a.png", "img/b.png", "img/deep/c.png"} {
		if err := store.Add("bkt", key, []byte("x"), &storage.Object{}); err != nil {
			t.Fatal(err)
		}
	}

	objs, err := emu.ListObjects(ctx, "", "bkt", ListOptions{MatchGlob: "**.png"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"img/a.png", "img/b.png", "img/deep/c.png"}; !reflect.DeepEqual(names(objs), want) {
		t.Errorf("items = %v, want %v", names(objs), want)
	}

	objs, err = emu.ListObjects(ctx, "", "bkt", ListOptions{MatchGlob: "img/*.png"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"img/a.png", "img/b.png"}; !reflect.DeepEqual(names(objs), want) {
		t.Errorf("items = %v, want %v", names(objs), want)
	}

	// Non-matching objects must not consume the page budget, or a sparse match
	// would come back as a run of empty pages.
	objs, err = emu.ListObjects(ctx, "", "bkt", ListOptions{MatchGlob: "**deep**", MaxResults: 1})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"img/deep/c.png"}; !reflect.DeepEqual(names(objs), want) {
		t.Errorf("items = %v, want %v", names(objs), want)
	}

	if _, err := emu.ListObjects(ctx, "", "bkt", ListOptions{MatchGlob: "["}); err == nil {
		t.Error("expected an error for a malformed glob")
	}
}

func TestListObjectsClampsMaxResults(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(t.TempDir())
	emu := NewGcsEmu(Options{Store: store})

	for i := 0; i < MaxResults+10; i++ {
		key := fmt.Sprintf("obj-%05d.txt", i)
		if err := store.Add("bkt", key, []byte("x"), &storage.Object{}); err != nil {
			t.Fatal(err)
		}
	}

	for _, maxResults := range []int{0, -1, MaxResults, MaxResults + 1, 1_000_000} {
		objs, err := emu.ListObjects(ctx, "", "bkt", ListOptions{MaxResults: maxResults})
		if err != nil {
			t.Fatal(err)
		}
		if len(objs.Items) != MaxResults {
			t.Errorf("MaxResults=%d returned %d items, want %d", maxResults, len(objs.Items), MaxResults)
		}
		if objs.NextPageToken == "" {
			t.Errorf("MaxResults=%d returned no page token, but the bucket holds more", maxResults)
		}
	}

	// A limit below the cap is still honored.
	objs, err := emu.ListObjects(ctx, "", "bkt", ListOptions{MaxResults: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(objs.Items) != 7 {
		t.Errorf("MaxResults=7 returned %d items, want 7", len(objs.Items))
	}
}

func TestListObjectsGlobRejectsDelimiter(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(t.TempDir())
	emu := NewGcsEmu(Options{Store: store})
	if err := store.Add("bkt", "a/b.txt", []byte("x"), &storage.Object{}); err != nil {
		t.Fatal(err)
	}

	for _, delimiter := range []string{"", "/"} {
		if _, err := emu.ListObjects(ctx, "", "bkt", ListOptions{MatchGlob: "**", Delimiter: delimiter}); err != nil {
			t.Errorf("delimiter %q with matchGlob: %v, want it accepted", delimiter, err)
		}
	}

	for _, delimiter := range []string{"-", "//", "|"} {
		_, err := emu.ListObjects(ctx, "", "bkt", ListOptions{MatchGlob: "**", Delimiter: delimiter})
		if err == nil {
			t.Errorf("delimiter %q with matchGlob: no error, want one", delimiter)
			continue
		}
		if code := httpStatusCodeOf(err); code != http.StatusBadRequest {
			t.Errorf("delimiter %q: status %d, want 400", delimiter, code)
		}
	}

	// Without a glob any delimiter is still fine.
	if _, err := emu.ListObjects(ctx, "", "bkt", ListOptions{Delimiter: "-"}); err != nil {
		t.Errorf("delimiter without matchGlob: %v, want it accepted", err)
	}
}

// TestListObjectsReportsPartialFailure pins that a listing which couldn't be completed
// is reported as an error rather than returned short.
//
// A caller learns it has seen every object by the absence of a page token, so handing
// back what was found so far would report a truncated bucket as a complete one — the
// listing would look authoritative while quietly omitting whatever came after the
// failure.
func TestListObjectsReportsPartialFailure(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := NewFileStore(dir)
	emu := NewGcsEmu(Options{Store: store})

	for _, key := range []string{"a.txt", "b.txt", "m/1.txt", "z.txt"} {
		if err := store.Add("bkt", key, []byte("x"), &storage.Object{}); err != nil {
			t.Fatal(err)
		}
	}

	// Walking fails partway: a sub-directory that can't be read.
	sub := filepath.Join(dir, "bkt", "m")
	if err := os.Chmod(sub, 0); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(sub, 0o755) }()
	if _, err := os.ReadDir(sub); err == nil {
		t.Skip("directory permissions are not enforced here (running as root?)")
	}

	if _, err := emu.ListObjects(ctx, "", "bkt", ListOptions{}); err == nil {
		t.Error("a listing that could not be completed returned no error")
	}

	// Resolving an object's metadata fails: a corrupt sidecar.
	if err := os.Chmod(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bkt", "b.txt"+metaExtention), []byte("{not json"), 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := emu.ListObjects(ctx, "", "bkt", ListOptions{}); err == nil {
		t.Error("a listing with an unreadable object returned no error")
	}
}
