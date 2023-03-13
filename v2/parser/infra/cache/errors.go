package cache

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"cache",
		`For more information see https://encore.dev/docs/primitives/caching`,
	)

	errExpectsTwoArgs = errRange.Newf("Invalid cache construction", "%s expects two arguments, got %d.)")

	errCouldNotResolveCacheCluster = errRange.New(
		"Invalid Cache Keyspace Construction",
		`Could not resolve the cache cluster: must refer to a package-level variable.`,
	)

	errPrefixReserved = errRange.New(
		"Invalid Cache Key Pattern",
		"The prefix `__encore` is reserved for internal use by Encore.",
	)

	errInvalidKeyspacePattern = errRange.New(
		"Invalid Cache Key Pattern",
		"Got an invalid keyspace pattern.",
	)

	errKeyPatternMustBeNamedKey = errRange.New(
		"Invalid Cache Key Pattern",
		"KeyPattern parameter must be named ':key' for basic (non-struct) key types",
	)

	errInvalidKeyTypeParameter = errRange.Newf(
		"Invalid Cache Key Type",
		"%s has an invalid key type parameter: %s",
	)

	errKeyContainsAnonymousFields = errRange.New(
		"Invalid Cache Key Type",
		"The key type must not contain anonymous fields.",
	)

	errKeyContainsUnexportedFields = errRange.New(
		"Invalid Cache Key Type",
		"The key type must not contain unexported fields.",
	)

	errFieldNotUsedInKeyPattern = errRange.Newf(
		"Invalid Cache Key Type",
		"Invalid use of the key type, the field %s was not used in the KeyPattern",
	)

	errFieldIsInvalid = errRange.Newf(
		"Invalid Cache Key Type",
		"The field %s is invalid: %s",
	)

	errFieldDoesntExist = errRange.Newf(
		"Invalid Cache Key Pattern",
		"Field %s does not existing in key type %s",
	)

	errMustBeANamedStructType = errRange.New(
		"Invalid Cache Value Type",
		"Must be a named struct type.",
	)

	errStructMustNotBePointer = errRange.New(
		"Invalid Cache Value Type",
		"Must not be a pointer type.",
	)

	errInvalidEvictionPolicy = errRange.New(
		"Invalid Cache Eviction Policy",
		"Must be one of of the constants defined in the cache package.",
	)
)
