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

## Contributing

### Building and Testing

To build and test encore locally, npm is required. You can install node.js by following these architecture-specific [installation instructions](https://nodejs.org/en/download/package-manager/).

Then build the Dash app:
```
$ cd $GOPATH/src/github.com/encoredev/encore/cli/daemon/dash/dashapp && npm install && npm run build
```

> ⚠️ If you are using npm v7, there is a known bug running esbuild post-install scripts (see https://github.com/evanw/esbuild/issues/462 and https://github.com/npm/cli/issues/2606). A workaround is to run `node node_modules/esbuild/install.js` manually after the `npm install` step above.

Once the Dash app is built, you can build and test the encore packages using `go build` and `go test`:
```
$ go build ./...
$ go test [-short] ./...
```