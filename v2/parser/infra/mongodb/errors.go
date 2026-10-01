package mongodb

import (
	"encr.dev/pkg/errors"
)

const (
	mongodbNewDatabaseHelp = "For example `mongodb.NewDatabase(\"my_database\", mongodb.DatabaseConfig{})`"
)

var (
	ErrDuplicateNames = errRange.New(
		"Duplicate MongoDB Databases",
		"Multiple MongoDB databases with the same name were found. Database names must be unique.",
	)

	errRange = errors.Range(
		"mongodb",
		"For more information about Encore, see https://encore.dev/docs",

		errors.WithRangeSize(20),
	)

	errNewDatabaseArgCount = errRange.Newf(
		"Invalid mongodb.NewDatabase call",
		"A call to mongodb.NewDatabase requires 2 arguments: the database name and the config object, got %d arguments.",
		errors.PrependDetails(mongodbNewDatabaseHelp),
	)
)
