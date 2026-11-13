package cueutil

import (
	"os"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"encr.dev/pkg/vfs"
)

var testMeta = &Meta{
	APIBaseURL: "http://localhost:4000",
	EnvName:    "local",
	EnvType:    EnvType_Development,
	CloudType:  CloudType_Local,
}

// metaCue declares every tag Meta passes to the loader, like the encore.gen.cue
// written for editors does.
const metaCue = `#Meta: {
	APIBaseURL: string @tag(APIBaseURL)
	Environment: {
		Name:  string @tag(EnvName)
		Type:  string @tag(EnvType)
		Cloud: string @tag(CloudType)
	}
}
`

// configTree returns the config files of an app with one service per name.
func configTree(t *testing.T, services ...string) *vfs.VFS {
	t.Helper()

	files := vfs.New()
	if _, err := files.AddFile("cue.mod/module.cue", []byte(`module: "encr.app/test"`+"\n"), time.Now()); err != nil {
		t.Fatalf("unable to add cue.mod/module.cue: %v", err)
	}

	for _, svc := range services {
		body := metaCue + `
CallbackURL: "\(#Meta.APIBaseURL)/` + svc + `.MyAPI"
EnvName:     #Meta.Environment.Name
Cloud:       #Meta.Environment.Cloud
`
		if _, err := files.AddFile(svc+"/"+svc+".cue", []byte("package "+svc+"\n\n"+body), time.Now()); err != nil {
			t.Fatalf("unable to add %s/%s.cue: %v", svc, svc, err)
		}
	}
	return files
}

// TestLoadFromDirSharedTree checks that every service loads its own config
// from one written tree, and that removing the tree removes the directory.
func TestLoadFromDirSharedTree(t *testing.T) {
	c := qt.New(t)
	files := configTree(t, "svc1", "svc2")

	// Load both services from a single copy of the tree.
	dir, cleanup, err := WriteToDisk(files)
	c.Assert(err, qt.IsNil)
	t.Cleanup(cleanup)
	for _, svc := range []string{"svc1", "svc2"} {
		cfg, err := LoadFromDir(dir, svc, testMeta)
		c.Assert(err, qt.IsNil)

		data, err := cfg.MarshalJSON()
		c.Assert(err, qt.IsNil)
		c.Assert(string(data), qt.JSONEquals, map[string]any{
			"CallbackURL": "http://localhost:4000/" + svc + ".MyAPI",
			"EnvName":     "local",
			"Cloud":       "local",
		})
	}
	cleanup()
	c.Assert(dirRemoved(dir), qt.IsTrue, qt.Commentf("temporary directory %s still exists", dir))
}

// TestLoadFromDirIncompleteConfig checks a config file that does not resolve is
// reported as an error, and the tree can be removed afterwards.
func TestLoadFromDirIncompleteConfig(t *testing.T) {
	c := qt.New(t)
	files := configTree(t, "svc1")
	if _, err := files.AddFile("svc1/svc1.cue", []byte("package svc1\n\n"+metaCue+"\nCallbackURL: string\n"), time.Now()); err != nil {
		t.Fatalf("unable to add svc1/svc1.cue: %v", err)
	}

	dir, cleanup, err := WriteToDisk(files)
	c.Assert(err, qt.IsNil)
	t.Cleanup(cleanup)

	_, err = LoadFromDir(dir, "svc1", testMeta)
	c.Assert(err, qt.Not(qt.IsNil))

	cleanup()
	c.Assert(dirRemoved(dir), qt.IsTrue, qt.Commentf("temporary directory %s still exists", dir))
}

// dirRemoved reports whether the given directory is gone.
func dirRemoved(dir string) bool {
	_, err := os.Stat(dir)
	return os.IsNotExist(err)
}
