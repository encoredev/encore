# Developing Encore

Building an Encore application requires access to both the Encore runtime (the `encore.dev` package) as well as a custom-built
Go runtime ([encore-go](https://github.com/encoredev/encore-go)) to implement Encore's request semantics and automated instrumentation.

As a result the Encore Daemon must know where these two things exist on the filesystem in order to properly compile the Encore application.

This must be done in one of two ways: embedding the installation path at compile time (similar to `GOROOT`)
or by setting an environment variable at runtime.

The environment variables are:
- `ENCORE_GOROOT` – the path to encore-go on disk
- `ENCORE_RUNTIME_PATH` – the path to the `encore.dev` runtime implementation.

`ENCORE_RUNTIME_PATH` can be set to location of the `compiler/runtime` package in this repository,
while `ENCORE_GOROOT` must be pointed to where `encore-go` was built.

For more information on this see [cli/internal/env/env.go](cli/internal/env/env.go).