package sqldb

import (
	"encr.dev/pkg/errors"
)

var (
	ErrDuplicateNames = errRange.New(
		"Duplicate Databases",
		"Multiple databases with the same name were found. Database names must be unique.",
	)

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
	errNewDatabaseArgCount = errRange.Newf(
		"Invalid sqldb.NewDatabase call",
		"A call to sqldb.NewDatabase requires 2 arguments: the database name and the config object, got %d arguments.",
	)
	errNewDatabaseAbsPath = errRange.New(
		"Invalid sqldb.NewDatabase call",
		"The migration path must be a relative path rooted within the package directory, got an absolute path.",
	)
	errNewDatabaseNonLocalPath = errRange.New(
		"Invalid sqldb.NewDatabase call",
		"The migration path must be a relative path rooted within the package directory, got a non-local path.",
	)
	errNewDatabaseMigrationDirNotFound = errRange.New(
		"Invalid sqldb.NewDatabase call",
		"The migration directory does not exist.",
	)
	errMigrationsNotInMainModule = errRange.New(
		"Invalid database migration directory",
		"The migration path must be within the application's main module.",
	)
)
