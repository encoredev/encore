# Developing Encore

Building an Encore application requires access to both the Encore runtime (the `encore.dev` package) as well as a custom-built
([Go runtime](https://github.com/encoredev/go)) to implement Encore's request semantics and automated instrumentation.

As a result the Encore Daemon must know where these two things exist on the filesystem in order to properly compile the Encore application.

This must be done in one of two ways: embedding the installation path at compile time (similar to `GOROOT`)
or by setting an environment variable at runtime.

The environment variables are:
- `ENCORE_GOROOT` – the path to encore-go on disk
- `ENCORE_RUNTIME_PATH` – the path to the `encore.dev` runtime implementation.

`ENCORE_RUNTIME_PATH` can be set to location of the `compiler/runtime` package in this repository,
while `ENCORE_GOROOT` must be pointed to where `encore-go` was built.

For more information on this see [cli/daemon/internal/env/env.go](cli/daemon/internal/env/env.go).

## Architecture

The code base is divided into several parts:

### cli
The `encore` command line interface. The encore background daemon
is located at `cli/daemon` and is responsible for managing processes,
setting up databases and talking with the Encore servers for operations like
fetching production logs.

### parser
The Encore Parser statically analyzes Encore apps to build up a model
of the application dubbed the Encore Syntax Tree (EST) that lives in
`parser/est`.

For speed the parser does not perform traditional type-checking; it does
limited type-checking for enforcing Encore-specific rules but otherwise
relies on the underlying Go compiler to perform type-checking as part of
building the application.

### compiler
The Encore Compiler rewrites the source code based on the parsed
Encore Syntax Tree to create a fully functioning application.
It rewrites API calls & API handlers, injects instrumentation
and secret values, and more.