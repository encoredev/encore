# Encore - The Go backend framework with superpowers

https://encore.dev

## Overview

Encore is a Go backend framework for rapidly creating
APIs and distributed systems.

Encore works by using source code analysis to understand how your application fits together, and then uses that to provide many [superpowers](#superpowers) that radically improve the developer experience.

**Read the complete documentation at [encore.dev/docs](https://encore.dev/docs).**

## Quick Start

### Install
```bash
# macOS
brew install encoredev/tap/encore
# Linux
curl -L https://encore.dev/install.sh | bash
# Windows
iwr https://encore.dev/install.ps1 | iex
```

### Create your app
```bash
encore app create my-app
cd my-app
encore run
```

### Deploy
```bash
git push encore
```

## Superpowers

Encore comes with tons of superpowers that radically simplify backend development compared to traditional frameworks:

- A state of the art developer experience with unmatched productivity
- Define services, APIs, and make API calls with a single line of Go code
- Autocomplete and get compile-time checks for API calls 
- Generates beautiful API docs and API clients automatically
- Instruments your app with Distributed Tracing, logs, and metrics – automatically
- Runs serverlessly on Encore's cloud, or deploys to your own favorite cloud
- Sets up dedicated Preview Environments for your pull requests
- Supports flexible authentication 
- Manages your databases and migrates them automatically
- Provides an extremely simple yet secure secrets management
- And lots more...

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

## Contributing to Encore and building from source

See [HACKING.md](HACKING.md).