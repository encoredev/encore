# End to End Tests

This folder contains end to end tests for Encore, which test everything in Encore
as an end user would use it.

The tests will:
1. Effectively run `encore run` on the [echo test app](./testdata/echo)
2. Perform some basic requests against the running app to verify behaviour
3. Generate the front end clients for the app
4. Run tests against using generated clients against the running app
5. Shutdown the running app

The TypeScript app's `client_imports.test.ts` covers lazy generated test clients:
importing `~encore/clients` must not initialize service definitions or middleware.
The first endpoint call loads its service's test module; the module loader owns
caching so Vitest mocks and `vi.resetModules()` remain effective. Handler lookup
and registration still happen on every call. Keep the test import inside the
`ENCORE_DROP_TESTS` guard in the client template so production bundles omit it.
