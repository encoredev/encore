package apps

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"encr.dev/pkg/appfile"
)

func TestListEvictsMissingAppInstances(t *testing.T) {
	tests := []struct {
		name   string
		remove func(t *testing.T, root string)
	}{
		{
			name: "missing app root",
			remove: func(t *testing.T, root string) {
				t.Helper()
				if err := os.RemoveAll(root); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "missing encore.app",
			remove: func(t *testing.T, root string) {
				t.Helper()
				if err := os.Remove(filepath.Join(root, appfile.Name)); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := newTestDB(t)
			mgr := NewManager(db)
			t.Cleanup(func() {
				if err := mgr.Close(); err != nil {
					t.Errorf("close manager: %v", err)
				}
			})

			root := t.TempDir()
			writeTestAppFile(t, root)

			instance, err := mgr.Track(root)
			if err != nil {
				t.Fatalf("track app: %v", err)
			}
			if instance.watcher == nil {
				t.Fatal("expected app instance to have a watcher")
			}

			test.remove(t, root)

			got, err := mgr.List()
			if err != nil {
				t.Fatalf("list apps: %v", err)
			}
			if len(got) != 0 {
				t.Fatalf("got %d apps, want none", len(got))
			}

			var count int
			if err := db.QueryRow(`SELECT COUNT(*) FROM app WHERE root = ?`, root).Scan(&count); err != nil {
				t.Fatalf("count app rows: %v", err)
			}
			if count != 0 {
				t.Fatalf("got %d persisted app rows, want none", count)
			}

			mgr.instanceMu.Lock()
			_, cached := mgr.instances[root]
			mgr.instanceMu.Unlock()
			if cached {
				t.Fatal("missing app instance remains cached")
			}

			select {
			case <-instance.watcher.Done():
			default:
				t.Fatal("missing app instance watcher remains open")
			}

			writeTestAppFile(t, root)
			retracked, err := mgr.Track(root)
			if err != nil {
				t.Fatalf("retrack app: %v", err)
			}
			if retracked == instance {
				t.Fatal("retracked app reused evicted instance")
			}
		})
	}
}

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})

	_, err = db.Exec(`
		CREATE TABLE app (
			root TEXT PRIMARY KEY,
			local_id TEXT NOT NULL,
			platform_id TEXT NULL,
			updated_at TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("create app table: %v", err)
	}

	return db
}

func writeTestAppFile(t *testing.T, root string) {
	t.Helper()

	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatalf("create app root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, appfile.Name), []byte(`{"id":"test-app"}`), 0644); err != nil {
		t.Fatalf("write encore.app: %v", err)
	}
}
