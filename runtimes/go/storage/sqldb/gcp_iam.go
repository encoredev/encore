package sqldb

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// gcpIAMScope is the scope Cloud SQL accepts an access token under as a
// database password.
const gcpIAMScope = "https://www.googleapis.com/auth/sqlservice.login"

// gcpIAMTokens is the token source of the Google Cloud identity the process
// runs as (on Cloud Run, its service account, through the metadata server). It
// caches a token until shortly before it expires, so connections opened in the
// meantime share it.
var gcpIAMTokens = sync.OnceValues(func() (oauth2.TokenSource, error) {
	return google.DefaultTokenSource(context.Background(), gcpIAMScope)
})

// gcpIAMBeforeConnect sets the password of a connection about to be opened for
// a user that logs in with Cloud SQL's IAM database authentication to a
// current access token. A token only has to be valid when the connection
// authenticates; the connection outlives it.
func gcpIAMBeforeConnect(ctx context.Context, cfg *pgx.ConnConfig) error {
	ts, err := gcpIAMTokens()
	if err != nil {
		return fmt.Errorf("sqldb: find Google Cloud credentials for IAM database authentication: %w", err)
	}
	tok, err := ts.Token()
	if err != nil {
		return fmt.Errorf("sqldb: get access token for IAM database authentication: %w", err)
	}
	cfg.Password = tok.AccessToken
	return nil
}
