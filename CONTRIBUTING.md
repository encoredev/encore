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
To build from the source simply run `go build ./cli/cmd/encore` and `go install ./cli/cmd/git-remote-encore`.

Running an Encore application requires both the Encore runtime (the `encore.dev` package) as well as a custom-built
[Go runtime](https://github.com/encoredev/go) to implement Encore's request semantics and automated instrumentation.

As a result, the Encore Daemon must know where these two things exist on the filesystem to compile the Encore application properly.

This must be done in one of two ways: embedding the installation path at compile time (similar to `GOROOT`)
or by setting an environment variable at runtime.

The environment variables are:
- `ENCORE_RUNTIMES_PATH` – the path to the `encore.dev` runtime implementation.
- `ENCORE_GOROOT` – the path to encore-go on disk

**ENCORE_RUNTIMES_PATH**

This must be set to the location of the `encore.dev` runtime package.
It's located in this Git repository in the `runtimes` directory:

```bash
export ENCORE_RUNTIMES_PATH=/path/to/encore/runtimes
```

**ENCORE_GOROOT**

The `ENCORE_GOROOT` must be set to the path to the [Encore Go runtime](https://github.com/encoredev/go).
Unless you want to make changes to the Go runtime it's easiest to point this to an existing Encore installation.

To do that, run `encore daemon env` and grab the value of `ENCORE_GOROOT`. For example (yours is probably different):

```bash
export ENCORE_GOROOT=/opt/homebrew/Cellar/encore/0.16.2/libexec/encore-go
```

### Running applications when building from source
Once you've built your own `encore` binary and set the environment variables above, you're ready to go!

Start the daemon with the built binary: `./encore daemon -f`

Note that when you run commands like `encore run` must use the same `encore` binary the daemon is running.


### Testing the Daemon run logic
The codegen tests in the `internal/clientgen/client_test.go` file uses many auto generated files from the
`e2e-tests/testdata` directory. To generate the client files and other test files, run `go test -golden-update` from
the `e2e-tests` directory. This will generate client files for all the supported client generation languages.

Running `go test ./internal/clientgen` will now work and use the most recent client generated files. If
you change the client or content of the `testdata` folder, you may need to regenerate the client files again.

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
