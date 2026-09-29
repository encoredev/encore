package platform

import (
	"context"
	"slices"

	"github.com/cockroachdb/errors"
)

type Org struct {
	ID           string
	Name         string
	Slug         string // may be empty
	CanCreateApp bool
}

// ListOrgs lists the orgs the user belongs to.
func ListOrgs(ctx context.Context) ([]*Org, error) {
	query := `
query ListOrgs {
	organizations {
		id, name, slug
		userScopes { name }
	}
}`
	var out struct {
		Organizations []struct {
			ID         string
			Name       string
			Slug       *string
			UserScopes []struct{ Name string }
		}
	}
	in := graphqlRequest{Query: query, OperationName: "ListOrgs"}
	if err := graphqlCall(ctx, in, &out, true); err != nil {
		return nil, errors.Wrap(err, "list orgs")
	}

	orgs := make([]*Org, 0, len(out.Organizations))
	for _, o := range out.Organizations {
		org := &Org{ID: o.ID, Name: o.Name}
		if o.Slug != nil {
			org.Slug = *o.Slug
		}
		org.CanCreateApp = slices.ContainsFunc(o.UserScopes, func(s struct{ Name string }) bool {
			return s.Name == "ORGS_CREATE_APP"
		})
		orgs = append(orgs, org)
	}
	return orgs, nil
}
