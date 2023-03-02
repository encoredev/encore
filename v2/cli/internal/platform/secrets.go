package platform

import (
	"context"

	"github.com/cockroachdb/errors"

	"encr.dev/v2/cli/internal/platform/gql"
)

func ListSecretGroups(ctx context.Context, appSlug string, keys []string) ([]*gql.Secret, error) {
	query := `
query ListSecretGroups($appSlug: String!, $keys: [String!]) {
	app(slug: $appSlug) {
		secrets(keys: $keys) {
			key
			groups {
				id, etag, description, archivedAt
				selector {
					__typename
					...on SecretSelectorEnvType {
						kind
					}
					...on SecretSelectorSpecificEnv {
						env { id, name }
					}
				}
				versions { id }
			}
		}
	}
}`
	var out struct {
		App struct {
			*gql.App
			Secrets []*gql.Secret
		}
	}

	in := graphqlRequest{Query: query, Variables: map[string]any{"appSlug": appSlug, "keys": keys}}
	if err := graphqlCall(ctx, in, &out, true); err != nil {
		return nil, err
	}
	return out.App.Secrets, nil
}

type CreateSecretGroupParams struct {
	AppID          string
	Key            string
	PlaintextValue string
	Description    string
	Selector       []gql.SecretSelector
}

func CreateSecretGroup(ctx context.Context, p CreateSecretGroupParams) error {
	query := `
mutation CreateSecretGroup($input: CreateSecretGroups!) {
	createSecretGroups(input: $input) { id }
}`
	envTypes, envIDs, err := mapSecretSelector(p.Selector)
	if err != nil {
		return err
	}

	in := graphqlRequest{Query: query, Variables: map[string]any{"input": map[string]any{
		"appID": p.AppID,
		"key":   p.Key,
		"entries": []map[string]any{
			{
				"plaintextValue": p.PlaintextValue,
				"envTypes":       envTypes,
				"envIDs":         envIDs,
				"description":    p.Description,
			},
		},
	}}}
	if err := graphqlCall(ctx, in, nil, true); err != nil {
		return errors.Wrap(err, "create secret group")
	}
	return nil
}

type CreateSecretVersionParams struct {
	GroupID        string
	PlaintextValue string
	Etag           string
}

func CreateSecretVersion(ctx context.Context, p CreateSecretVersionParams) error {
	query := `
mutation CreateSecretVersion($input: CreateSecretVersion!) {
	createSecretVersion(input: $input) { id }
}`
	in := graphqlRequest{Query: query, Variables: map[string]any{"input": map[string]any{
		"groupID":        p.GroupID,
		"plaintextValue": p.PlaintextValue,
		"etag":           p.Etag,
	}}}
	if err := graphqlCall(ctx, in, nil, true); err != nil {
		return errors.Wrap(err, "create secret version")
	}
	return nil
}

type UpdateSecretGroupParams struct {
	ID   string
	Etag *string

	// Nil fore ach field here means it's kept unchanged.
	Selector    []gql.SecretSelector // nil means no changes
	Archived    *bool
	Description *string
}

func UpdateSecretGroup(ctx context.Context, p UpdateSecretGroupParams) error {
	query := `
mutation UpdateSecretGroup($input: UpdateSecretGroup!) {
	updateSecretGroup(input: $input) { id }
}`

	var selector map[string]any
	if p.Selector != nil {
		envTypes, envIDs, err := mapSecretSelector(p.Selector)
		if err != nil {
			return err
		}
		selector = map[string]any{
			"envTypes": envTypes,
			"envIDs":   envIDs,
		}
	}

	in := graphqlRequest{Query: query, Variables: map[string]any{"input": map[string]any{
		"id":          p.ID,
		"etag":        p.Etag,
		"selector":    selector,
		"archived":    p.Archived,
		"description": p.Description,
	}}}
	if err := graphqlCall(ctx, in, nil, true); err != nil {
		return errors.Wrap(err, "update secret group")
	}
	return nil
}

func mapSecretSelector(selector []gql.SecretSelector) (envTypes, envIDs []string, err error) {
	envTypes, envIDs = []string{}, []string{}
	for _, sel := range selector {
		switch s := sel.(type) {
		case *gql.SecretSelectorEnvType:
			envTypes = append(envTypes, s.Kind)
		case *gql.SecretSelectorSpecificEnv:
			envIDs = append(envIDs, s.Env.ID)
		default:
			return nil, nil, errors.Newf("unknown secret selector type %T", s)
		}
	}
	return envTypes, envIDs, nil
}
