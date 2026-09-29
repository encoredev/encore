package app

import (
	"testing"

	"encr.dev/cli/internal/platform"
)

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
