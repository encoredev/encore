package parser

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"encr.dev/v2/internals/testutil"
	"encr.dev/v2/parser/infra/sqldb"
	"encr.dev/v2/parser/resource"
)

func Test_deduplicateSQLDBResources(t *testing.T) {
	fooDB := &sqldb.Database{Name: "foo", MigrationDir: "./foo/migrations"}
	barDB := &sqldb.Database{Name: "bar", MigrationDir: "./bar/migrations"}
	quxDB := &sqldb.Database{Name: "qux", MigrationDir: "./bar/migrations"} // same migrations as bar

	fooImplicitBind := &resource.ImplicitBind{Resource: resource.ResourceOrPath{Resource: fooDB}}
	fooExplicitBind := &resource.PkgDeclBind{Resource: resource.ResourceOrPath{Resource: fooDB}}
	barImplicitBind := &resource.ImplicitBind{Resource: resource.ResourceOrPath{Resource: barDB}}
	quxExplicitBind := &resource.PkgDeclBind{Resource: resource.ResourceOrPath{Resource: quxDB}}

	foo2DB := &sqldb.Database{Name: "foo"}
	foo2ImplicitBind := &resource.ImplicitBind{Resource: resource.ResourceOrPath{Resource: foo2DB}}

	tests := []struct {
		name      string
		res       []resource.Resource
		binds     []resource.Bind
		wantRes   []resource.Resource
		wantBinds []resource.Bind
	}{
		{
			name:      "implicit_only",
			res:       []resource.Resource{fooDB},
			binds:     []resource.Bind{fooImplicitBind},
			wantRes:   []resource.Resource{fooDB},
			wantBinds: []resource.Bind{fooImplicitBind},
		},
		{
			name:    "resource_only",
			res:     []resource.Resource{fooDB},
			wantRes: []resource.Resource{fooDB},
		},
		{
			name:      "explicit_only",
			res:       []resource.Resource{fooDB},
			binds:     []resource.Bind{fooExplicitBind},
			wantRes:   []resource.Resource{fooDB},
			wantBinds: []resource.Bind{fooExplicitBind},
		},
		{
			name:      "deduplicate_simple",
			res:       []resource.Resource{fooDB},
			binds:     []resource.Bind{fooExplicitBind, fooImplicitBind},
			wantRes:   []resource.Resource{fooDB},
			wantBinds: []resource.Bind{fooExplicitBind},
		},
		{
			name:      "deduplicate_keep_other_db",
			res:       []resource.Resource{fooDB, barDB},
			binds:     []resource.Bind{fooExplicitBind, barImplicitBind, fooImplicitBind},
			wantRes:   []resource.Resource{fooDB, barDB},
			wantBinds: []resource.Bind{fooExplicitBind, barImplicitBind},
		},
		{
			name:      "deduplicate_multiple_identical_resources",
			res:       []resource.Resource{fooDB, foo2DB},
			binds:     []resource.Bind{fooExplicitBind, foo2ImplicitBind},
			wantRes:   []resource.Resource{fooDB},
			wantBinds: []resource.Bind{fooExplicitBind},
		},
		{
			name:      "deduplicate_different_db",
			res:       []resource.Resource{barDB, quxDB},
			binds:     []resource.Bind{barImplicitBind, quxExplicitBind},
			wantRes:   []resource.Resource{quxDB},
			wantBinds: []resource.Bind{quxExplicitBind},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			gotRes, gotBinds := deduplicateSQLDBResources(tt.res, tt.binds)
			c.Assert(gotRes, testutil.ResourceDeepEquals, tt.wantRes)
			c.Assert(gotBinds, testutil.ResourceDeepEquals, tt.wantBinds)
		})
	}
}
