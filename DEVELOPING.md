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

## Building from source

To build from source, simply run `go build ./cli/cmd/encore`.

To start the Encore daemon with the built binary, you must set the env variables as described above.
*(The below is just an example; your disk locations will differ.)*

```bash
export ENCORE_RUNTIME_PATH=$HOME/src/encore.build/encr.dev/compiler/runtime # or whatever
export ENCORE_GOROOT=$HOME/src/encore.build/encore-go/dist/$(go env GOOS)_$(go env GOARCH)/encore-go
```

Finally start the daemon with the built binary: `./encore daemon -f`.

Note that running commands like `encore run` must use the same `encore` binary the daemon is running.

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

## Dashapp

To run the dashapp in development mode (using `npm run dev`), the dashapp needs
to know the url to the daemon. To achieve this, you must set an environmental 
variable with the correct address. Add a file in the [dashapp directory](cli/daemon/dash/dashapp)
called `.env.development.local` with the following content:
```
VITE_DAEMON_ADDRESS=localhost:12345
```
The actual dash port can be found when running `./encore daemon -f` or when
running `encore run` where it's the same as the Dev Dashboard Url. 

> **Note:** If you restart the daemon, the port will likely change. This means that
> you have to set the new port in `.env.development.local` and restart the
> dashapp.