package app

import (
	"slices"
	"testing"

	"encr.dev/cli/internal/platform"
)

func TestPersonalOrgChoice(t *testing.T) {
	personal := &platform.Org{ID: "org_p", Name: "Personal", Personal: true}
	acme := &platform.Org{ID: "org_1", Name: "Acme"}
	zed := &platform.Org{ID: "org_2", Name: "Zed"}

	t.Run("only the personal org leaves nothing to choose", func(t *testing.T) {
		if hasChoice([]*platform.Org{personal}) {
			t.Error("hasChoice = true, want false")
		}
		if hasChoice(nil) {
			t.Error("hasChoice(nil) = true, want false")
		}
		if !hasChoice([]*platform.Org{personal, acme}) {
			t.Error("hasChoice = false, want true")
		}
	})

	t.Run("items", func(t *testing.T) {
		tests := []struct {
			name string
			orgs []*platform.Org
			want []orgItem
		}{
			{
				name: "no personal org",
				orgs: []*platform.Org{acme, zed},
				want: []orgItem{{id: "", name: "Personal account"}, {id: "org_1", name: "Acme"}, {id: "org_2", name: "Zed"}},
			},
			{
				name: "personal org",
				orgs: []*platform.Org{acme, zed, personal},
				want: []orgItem{{id: "org_p", name: "Personal"}, {id: "org_1", name: "Acme"}, {id: "org_2", name: "Zed"}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := orgItems(tt.orgs); !slices.Equal(got, tt.want) {
					t.Errorf("orgItems = %v, want %v", got, tt.want)
				}
			})
		}
	})
}

func TestMatchOrg(t *testing.T) {
	orgs := []*platform.Org{
		{ID: "org_1", Name: "Acme", Slug: "acme"},
		{ID: "org_2", Name: "No Slug"},
	}
	tests := []struct {
		key     string
		want    string
		wantErr bool
	}{
		{key: "org_1", want: "org_1"},
		{key: "acme", want: "org_1"},
		{key: "org_2", want: "org_2"},
		{key: "Acme", wantErr: true},
		{key: "missing", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got, err := matchOrg(orgs, tt.key)
			if (err != nil) != tt.wantErr {
				t.Fatalf("matchOrg(%q) err = %v, wantErr %v", tt.key, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("matchOrg(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}
