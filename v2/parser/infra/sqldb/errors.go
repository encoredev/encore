package sqldb

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"sqldb",
		"For more information about how to use databases in Encore, see https://encore.dev/docs/primitives/databases",

		errors.WithRangeSize(20),
	)

	errUnableToParseMigrations = errRange.New(
		"Unable to parse migrations",
		"Encore was unable to parse the database migrations. Please ensure that the migrations are in the correct format.",
	)

	errNamedRequiresDatabaseName = errRange.Newf(
		"Invalid call to sqldb.Named",
		"sqldb.Named requires a database name to be passed as the only argument, got %d arguments.",
	)

	errNamedRequiresDatabaseNameString = errRange.New(
		"Invalid call to sqldb.Named",
		"sqldb.Named requires a database name to be passed as a string literal.",
	)
	errMigrationsNotInMainModule = errRange.New(
		"Invalid database migration directory",
		"The migration path must be within the application's main module.",
	)
)
