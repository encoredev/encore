package platform

import (
	"context"
	"slices"

	"github.com/cockroachdb/errors"

	"encr.dev/internal/conf"
)

type Org struct {
	ID           string
	Name         string
	Slug         string // may be empty
	Personal     bool   // the user's own org
	CanCreateApp bool
}

// ListOrgs lists the orgs the user belongs to.
func ListOrgs(ctx context.Context) ([]*Org, error) {
	query := `
query ListOrgs {
	organizations {
		id, name, slug
		personalUser { id }
		userScopes { name }
	}
}`
	var out struct {
		Organizations []struct {
			ID           string
			Name         string
			Slug         *string
			PersonalUser *struct{ ID string }
			UserScopes   []struct{ Name string }
		}
	}
	in := graphqlRequest{Query: query, OperationName: "ListOrgs"}
	if err := graphqlCall(ctx, in, &out, true); err != nil {
		return nil, errors.Wrap(err, "list orgs")
	}

	// The logged-in user's ID; if unknown, no org is their personal org.
	var userID string
	if cfg, err := conf.CurrentUser(); err == nil {
		userID = cfg.Actor
	}

	orgs := make([]*Org, 0, len(out.Organizations))
	for _, o := range out.Organizations {
		org := &Org{ID: o.ID, Name: o.Name}
		org.Personal = o.PersonalUser != nil && o.PersonalUser.ID == userID
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
