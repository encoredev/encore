package cron

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"cron",
		"For more information, see https://encore.dev/docs/primitives/cron-jobs",

		errors.WithRangeSize(10),
	)

	errExpects2Arguments = errRange.Newf(
		"Invalid call to cron.NewJob",
		"Expected 2 arguments, got %d",
	)

	errScheduleSetTwice = errRange.New(
		"Invalid call to cron.NewJob",
		"The cron execution schedule was set twice, once in Every and once in Schedule. At least one of these must be set, but not both",
	)

	errInvalidSchedule = errRange.New(
		"Invalid call to cron.NewJob",
		"Schedule must be a valid cron expression",
	)

	errEveryMustBeInteger = errRange.Newf(
		"Invalid call to cron.NewJob",
		"Every must be an integer number of minutes, got %d seconds.",
	)

	errEveryMustBeOneOrGreater = errRange.Newf(
		"Invalid call to cron.NewJob",
		"Every must be between 1 minute and 24 hours, got %d seconds.",
	)

	errEveryMustBeLessThan24Hours = errRange.Newf(
		"Invalid call to cron.NewJob",
		"Every must be between 1 minute and 24 hours (1440 minutes), got %d minutes.",
	)

	errEveryMustBeMultipleOfMinute = errRange.Newf(
		"Invalid call to cron.NewJob",
		"Every 24 hour time range (from 00:00 to 23:59) needs to be evenly divided by the interval value (%s).",
	)
)
