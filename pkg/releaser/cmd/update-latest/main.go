// Command update-latest points latest/COMMIT in encore-releases2 at the
// commit whose build was just finalized, so tooling can find the newest
// build of main without knowing its sha: read latest/COMMIT, then fetch
// latest/<sha>/encore-<os>_<arch>.tar.gz.
//
// It only ever writes the pointer for a build stored under latest/<commit>,
// and refuses any other prefix, so a proper release can't end up behind it.
// Runs of the workflow are serialized by its concurrency group, so on push
// the pointer advances in commit order; re-running an older run's finalize
// job moves it back to that (complete) build until the next push.
//
// Configuration (environment variables):
//
//	R_COMMIT             the commit sha this build was made from
//	R_VERSION            release version, e.g. "v0.0.0-develop+<commit>"
//	R_RELEASE_PREFIX     object prefix in the bucket; must be "latest/<commit>"
//	RUNNER_TEMP          scratch directory
package main

import (
	"context"
	"regexp"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"encr.dev/pkg/releaser/config"
)

var commitRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

func main() {
	log.Logger = zerolog.New(zerolog.NewConsoleWriter()).With().Caller().Timestamp().Stack().Logger()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse environment variables")
	}
	commit, err := config.Required("R_COMMIT")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse environment variables")
	} else if !commitRe.MatchString(commit) {
		log.Fatal().Msgf("R_COMMIT %q is not a full commit sha", commit)
	}
	if want := config.LatestPrefix + "/" + commit; string(cfg.Prefix) != want {
		log.Fatal().Msgf("refusing to point %s at a build under %q (expected %q)", config.LatestCommitObject, cfg.Prefix, want)
	}

	if err := updateLatest(context.Background(), commit); err != nil {
		log.Fatal().Err(err).Msg("failed to update latest commit pointer")
	}
	log.Info().Msgf("%s now points at %s", config.LatestCommitObject, commit)
}

func updateLatest(ctx context.Context, commit string) error {
	client, err := config.NewClient(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to instantiate GCS client")
	}
	obj := client.Bucket(config.ReleasesBucket).Object(config.LatestCommitObject)

	w := obj.NewWriter(ctx)
	w.ContentType = "text/plain"
	// The pointer is small and mutable; don't let anything cache it.
	w.CacheControl = "no-cache"
	if _, err := w.Write([]byte(commit + "\n")); err != nil {
		_ = w.Close()
		return errors.Wrapf(err, "write %s", config.LatestCommitObject)
	}
	if err := w.Close(); err != nil {
		return errors.Wrapf(err, "write %s", config.LatestCommitObject)
	}
	return nil
}
