# Encore
https://encore.dev

The Go backend framework that understands what you're creating.

## Overview

Encore is a framework for building Go backends.
Unlike traditional frameworks Encore uses static analysis to understand what you're creating and helps you along the way.

**Read the documentation at [encore.dev/docs](https://encore.dev/docs).**

Features include:

- A state of the art developer experience with unmatched productivity
- Makes it super-easy to define services and APIs
- Adds compile time type-checking and auto-completion of your APIs
- Generates API docs and API clients for your app
- Instruments your app with distributed tracing, logs, and metrics – automatically
- Runs serverlessly on Encore's cloud, or deploys to your own favorite cloud
- Sets up a dedicated Preview Environments for your pull requests
- Supports flexible authentication 
- Manages your databases and migrates them automatically
- Provides an extremely simple yet secure secrets management.

### Installing

The easiest way to run Encore is to install the pre-built binaries.

#### macOS
`brew install encoredev/tap/encore`

#### Windows
Coming soon

#### Linux
Coming soon

### Building from source

See [HACKING.md](HACKING.md).

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