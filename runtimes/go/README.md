# Encore Runtime

This is the Encore runtime module that provides APIs and internal runtime components
for running Encore applications.

Using [the Public API generator tool](../tools/publicapigen) the [encore.dev Public API](https://github.com/encoredev/encore.dev)
is generated from this package. However, the following are not included in the public API:
- The [runtime](./runtime) package, as that is considered to be an internal implementation detail of Encore and it's API is considered unstable.
- Any internal packages, such as [pubsub/internal](./pubsub/internal), as those can't be accessed outside the Encore runtime.
- Any files with the suffix of `_internal.go` as those are implementation details and not expected to be called from outside an Encore application
- Any files with the suffix of `_test.go` as those simply form the testsuite for the runtime's implementation
- Any functions, types or variables which are not exported from the package (unless the comment `//publicapigen:keep` is present)
- The body of any functions, each body is replaced with a `panic()` call.
