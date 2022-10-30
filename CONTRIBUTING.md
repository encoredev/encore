# Contributing to Encore

We're so excited that you are interested in contributing to Encore!
All contributions are welcome, and there are several valuable ways to contribute.

Below is a technical walkthrough of developing the `encore` command for contributing code
to the Encore project. Head over to the community section for [more ways to contribute](https://encore.dev/docs/community/contribute)!

## GitHub Codespaces / VS Code Remote Containers
The easiest way to get started with developing Encore is using
GitHub Codespaces. Simply open this repository in a new Codespace
and your development environment will be set up with everything preconfigured for building the `encore` CLI and running applications with it.

This also works just as well with [Visual Studio Code's Remote Development](https://code.visualstudio.com/docs/remote/remote-overview).


## Building the encore command from source
To build from source, [build the dashboard](#Building-the-Development-Dashboard) and simply run `go build ./cli/cmd/encore`.

Running an Encore application requires both the Encore runtime (the `encore.dev` package) as well as a custom-built
[Go runtime](https://github.com/encoredev/go) to implement Encore's request semantics and automated instrumentation.

As a result the Encore Daemon must know where these two things exist on the filesystem in order to properly compile the Encore application.

This must be done in one of two ways: embedding the installation path at compile time (similar to `GOROOT`)
or by setting an environment variable at runtime.

The environment variables are:
- `ENCORE_RUNTIME_PATH` – the path to the `encore.dev` runtime implementation.
- `ENCORE_GOROOT` – the path to encore-go on disk

**ENCORE_RUNTIME_PATH**

This must be set to the location of the `encore.dev` runtime package.
It's located in this Git repository in the `runtime` directory:

```bash
export ENCORE_RUNTIME_PATH=/path/to/encore/runtime
```

**ENCORE_GOROOT**

The `ENCORE_GOROOT` must be set to the path to the [Encore Go runtime](https://github.com/encoredev/go).
Unless you want to make changes to the Go runtime it's easiest to point this to an existing Encore installation.

To do that, run `encore daemon env` and grab the value of `ENCORE_GOROOT`. For example (yours is probably different):

```bash
export ENCORE_GOROOT=/opt/homebrew/Cellar/encore/0.16.2/libexec/encore-go`
```

### Running applications when building from source
Once you've built your own `encore` binary and set the environment variables above, you're ready to go!

Start the daemon with the built binary: `./encore daemon -f`

Note that when you run commands like `encore run` must use the same `encore` binary the daemon is running.

## Developing the Development Dashboard

Encore comes with a development dashboard, located at `cli/daemon/dash/dashapp`.
It's a client-side React application built with [Vite](https://vitejs.dev/).

To run it from source:

```bash
cd cli/daemon/dash/dashapp
npm install
npm run dev
```

The dashboard application talks to the Encore daemon, and therefore needs to know its network address.
Set `ENCORE_DAEMON_DEV=1` before running `encore daemon -f` in order to force Encore to use a fixed port
for development purposes. The dashboard application assumes this is done when you run Vite in development mode.

### Building the Development Dashboard
The dashboard is embedded in the encore command and the build artifact needs to be available for the command to compile.

To create the artifact run:

```bash
cd cli/daemon/dash/dashapp
npm install
npm run build
```

### Testing the Development Dashboard
Tests are written using [Jest](https://jestjs.io/) and the [React Testing Library](https://testing-library.com/docs/react-testing-library/intro/).

To run the front-end unit tests:

```bash
cd cli/daemon/dash/dashapp
npm test #run all tests 
npm test -- --watch #re-run tests on file change
```

### Testing the Daemon run logic
The codegen tests in the `internal/clientgen/client_test.go` file uses many auto generated files from the
`e2e-tests/testdata` directory. To generate the client files and other test files, run `go test -golden-update` from
the `e2e-tests` directory. This will generate client files for all the supported client generation languages.

Running `go test ./internal/clientgen/client_test.go` will now work and use the most recent client generate files. If
you change the client or content of the `testdata` folder, you may need to regenerate the client files again.

## Architecture

The code base is divided into several parts:

### cli
The `encore` command line interface. The encore background daemon
is located at `cli/daemon` and is responsible for managing processes,
setting up databases and talking with the Encore servers for operations like
fetching production logs.

### cli/daemon/dash/dashapp
The Encore development dashboard frontend application. It renders
the generated API Documentation and API Explorer, traces, logs, and so on.

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
