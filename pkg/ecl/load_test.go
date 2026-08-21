package ecl

import (
	"testing"
	"testing/fstest"

	qt "github.com/frankban/quicktest"
)

func TestLoadImports(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	fsys := fstest.MapFS{
		"main.encore": &fstest.MapFile{Data: []byte(`import "policies/services.encore"
import "policies/storage.encore"

service "api" {
    cpu: default 2
}
`)},
		"policies/services.encore": &fstest.MapFile{Data: []byte(`for service {
    cpu: >= 0.25 & <= 8 | default 0.5
}
`)},
		// storage.encore imports a sibling file relative to its own directory.
		"policies/storage.encore": &fstest.MapFile{Data: []byte(`import "buckets.encore"

for sql_database {
    deletion_protection: true
}
`)},
		"policies/buckets.encore": &fstest.MapFile{Data: []byte(`for bucket {
    public_access: false
}
`)},
	}

	rs, err := Load(fsys, "main.encore")
	c.Assert(err, qt.IsNil)
	c.Assert(rs.Files, qt.HasLen, 4)

	// Imported rules participate in evaluation as one policy set.
	result := evalOK(c, rs, &Resource{Kind: "service", Name: "api"})
	c.Assert(result.Matched, qt.HasLen, 2)
	assertValue(c, result.Properties["cpu"].Value, Number(2))

	result = evalOK(c, rs, &Resource{Kind: "bucket", Name: "uploads"})
	assertValue(c, result.Properties["public_access"].Value, Bool(false))
}

func TestLoadImportCycle(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	fsys := fstest.MapFS{
		"a.encore": &fstest.MapFile{Data: []byte("import \"b.encore\"\nfor service { cpu: default 1 }\n")},
		"b.encore": &fstest.MapFile{Data: []byte("import \"a.encore\"\nfor bucket { versioning: true }\n")},
	}
	rs, err := Load(fsys, "a.encore")
	c.Assert(err, qt.IsNil)
	c.Assert(rs.Files, qt.HasLen, 2)
}

func TestLoadDuplicateImportsIncludedOnce(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	fsys := fstest.MapFS{
		"a.encore":      &fstest.MapFile{Data: []byte("import \"common.encore\"\nimport \"b.encore\"\n")},
		"b.encore":      &fstest.MapFile{Data: []byte("import \"common.encore\"\n")},
		"common.encore": &fstest.MapFile{Data: []byte("for service { cpu: default 1 }\n")},
	}
	rs, err := Load(fsys, "a.encore")
	c.Assert(err, qt.IsNil)
	c.Assert(rs.Files, qt.HasLen, 3)

	// The shared file's rule appears exactly once.
	result := evalOK(c, rs, &Resource{Kind: "service"})
	c.Assert(result.Matched, qt.HasLen, 1)
}

func TestLoadMissingImport(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	fsys := fstest.MapFS{
		"main.encore": &fstest.MapFile{Data: []byte("import \"nope.encore\"\n")},
	}
	_, err := Load(fsys, "main.encore")
	assertErrContains(c, err,
		`main.encore:1:8: error: cannot find imported file "nope.encore"`,
		`import "nope.encore"`)
}

func TestLoadMissingEntrypoint(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	_, err := Load(fstest.MapFS{}, "main.encore")
	assertErrContains(c, err, `cannot read file "main.encore"`)
}

func TestLoadParseErrorsAcrossFiles(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	fsys := fstest.MapFS{
		"main.encore":  &fstest.MapFile{Data: []byte("import \"other.encore\"\nfor service { cpu = 1 }\n")},
		"other.encore": &fstest.MapFile{Data: []byte("for bucket { versioning == }\n")},
	}
	_, err := Load(fsys, "main.encore")
	assertErrContains(c, err, "main.encore:2:19", "other.encore:1:25")
}
