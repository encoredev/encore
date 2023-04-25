package parser

import (
	"reflect"
	"testing"

	"encr.dev/v2/parser/infra/sqldb"
	"encr.dev/v2/parser/resource"
)

func Test_deduplicateSQLDBResources(t *testing.T) {
	fooDB := &sqldb.Database{Name: "foo"}
	barDB := &sqldb.Database{Name: "bar"}

	fooImplicitBind := &resource.ImplicitBind{Resource: resource.ResourceOrPath{Resource: fooDB}}
	fooExplicitBind := &resource.PkgDeclBind{Resource: resource.ResourceOrPath{Resource: fooDB}}
	barImplicitBind := &resource.ImplicitBind{Resource: resource.ResourceOrPath{Resource: barDB}}

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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRes, gotBinds := deduplicateSQLDBResources(tt.res, tt.binds)
			if !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("deduplicateSQLDBResources() got resources = %v, want %v", gotRes, tt.wantRes)
			}
			if !reflect.DeepEqual(gotBinds, tt.wantBinds) {
				t.Errorf("deduplicateSQLDBResources() got binds = %v, want %v", gotBinds, tt.wantBinds)
			}
		})
	}
}
