package secrets

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"secrets",
		"For more information about how to use secrets, see https://encore.dev/docs/primitives/secrets",

		errors.WithRangeSize(20),
	)

	errSecretsMustBeStruct = errRange.New(
		"Invalid secrets variable",
		"The \"secrets\" variable type must be an inline struct.",
	)

	errSecretsDefinedSeperately = errRange.New(
		"Invalid secrets variable",
		"The \"secrets\" variable must be declared separately and not in a var block.",
	)

	errSecretsGivenValue = errRange.New(
		"Invalid secrets variable",
		"The \"secrets\" variable must not be given a value. Encore will ensure that the secrets are loaded at runtime.",
	)

	errAnonymousFields = errRange.New(
		"Invalid secrets struct",
		"Anonymous fields are not allowed in the secrets struct.",
	)

	errSecretsMustBeString = errRange.New(
		"Invalid secrets struct",
		"Secrets must be of type string.",
	)
)
