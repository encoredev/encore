---
seotitle: Encore CLI Reference
seodesc: The Encore CLI lets you run your local development environment, create apps, and much more. See all CLI commands in this reference guide.
title: CLI Reference
subtitle: The Encore CLI lets you run your local environment and much more.
lang: go
---

## Running

#### Run

Runs your application.

```shell
$ encore run [--debug] [--watch=true] [--port=4000] [--listen=<addr>] [flags]
```

**Flags**

| Flag | Description | Default |
| --- | --- | --- |
| `-w, --watch` | Watch for changes and live-reload | `true` |
| `--listen` | Address to listen on (e.g. `0.0.0.0:4000`) | |
| `-p, --port` | Port to listen on | `4000` |
| `--json` | Display logs in JSON format | `false` |
| `-n, --namespace` | Namespace to use (defaults to active namespace) | |
| `--color` | Whether to display colorized output | auto-detected |
| `--redact` | Redact sensitive data in traces when running locally | `false` |
| `-l, --level` | Minimum log level to display (`trace\|debug\|info\|warn\|error`) | |
| `--debug` | Compile for debugging (`enabled\|break`) | |
| `--browser` | Open local dev dashboard in browser on startup (`auto\|never\|always`) | `auto` |

#### Test

Tests your application

Takes all the same flags as `go test`.

```shell
$ encore test ./... [go test flags]
```

Additional flags recognized by `encore test`:

| Flag | Description |
| --- | --- |
| `--codegen-debug` | Dump generated code (for debugging Encore's code generation) |
| `--prepare` | Prepare for running tests without running them |
| `--trace` | Write trace information about the parse and compilation process to a file |
| `--no-color` | Disable colorized output |

#### Check

Checks your application for compile-time errors using Encore's compiler.

```shell
$ encore check [flags]
```

**Flags**

| Flag | Description |
| --- | --- |
| `--codegen-debug` | Dump generated code (for debugging Encore's code generation) |
| `--tests` | Parse tests as well |

#### Exec

Runs executable scripts against the local Encore app.

Compiles and runs a Go script with the local Encore app environment setup.

```shell
$ encore exec <path/to/command> [...args]
```

The command directory should contain Go files with package main with a main function.

The additional arguments are passed directly to the built binary.

**Flags**

| Flag | Description |
| --- | --- |
| `-n, --namespace` | Namespace to use (defaults to active namespace) |

##### Example

Run a database seed script

```shell
$ encore exec cmd/seed
```

## App

Commands to create and link Encore apps

#### Clone

Clone an Encore app to your computer

```shell
$ encore app clone [app-id] [directory]
```

#### Create

Create a new Encore app

```shell
$ encore app create [name] [flags]
```

**Flags**

| Flag | Description | Default |
| --- | --- | --- |
| `--example` | URL to example code to use | |
| `-l, --lang` | Programming language to use for the app | |
| `-r, --llm-rules` | Initialize the app with LLM rules for a specific tool | |
| `--platform` | Whether to create the app with the Encore Platform | `true` |

#### Init

Create a new Encore app from an existing repository

```shell
$ encore app init [name] [flags]
```

**Flags**

| Flag | Description |
| --- | --- |
| `-l, --lang` | Programming language to use for the app |

#### Link

Link an Encore app with the server

```shell
$ encore app link [app-id] [flags]
```

**Flags**

| Flag | Description |
| --- | --- |
| `-f, --force` | Force link even if the app is already linked |

## Auth

Commands to authenticate with Encore

#### Login

Log in to Encore

```shell
$ encore auth login [flags]
```

**Flags**

| Flag | Description |
| --- | --- |
| `-k, --auth-key` | Auth Key to use for login |

#### Logout

Logs out the currently logged in user

```shell
$ encore auth logout
```

#### Signup

Create a new Encore account

```shell
$ encore auth signup
```

#### Whoami

Show the current logged in user

```shell
$ encore auth whoami
```

## Daemon

Encore CLI daemon commands

#### Restart

If you experience unexpected behavior, try restarting the daemon using:

```shell
$ encore daemon
```

#### Env

Outputs Encore environment information

```shell
$ encore daemon env
```

## Database Management

Database management commands

#### Connect to database via shell

Connects to the database via psql shell

Defaults to connecting to your local environment. Specify --env to connect to another environment.

Use `--test` to connect to databases used for integration testing.
Use `--shadow` to connect to the shadow database, used for database drift detection when using tools like Prisma.

`--test` and `--shadow` imply `--env=local`.

```shell
$ encore db shell [DATABASE_NAME] [--env=<name>] [flags]
```

`encore db shell` defaults to read-only permissions. Use `--write`, `--admin` and `--superuser` flags to modify which permissions you connect with.

**Flags**

| Flag | Description | Default |
| --- | --- | --- |
| `-n, --namespace` | Namespace to use (defaults to active namespace) | |
| `-e, --env` | Environment name to connect to | `local` |
| `-t, --test` | Connect to the integration test database (implies --env=local) | `false` |
| `--shadow` | Connect to the shadow database (implies --env=local) | `false` |
| `--write` | Connect with write privileges | `false` |
| `--admin` | Connect with admin privileges | `false` |
| `--superuser` | Connect as a superuser | `false` |

#### Connection URI

Outputs a database connection string. Defaults to connecting to your local environment. Specify --env to connect to another environment.

```shell
$ encore db conn-uri [<db-name>] [--env=<name>] [flags]
```

**Flags**

| Flag | Description | Default |
| --- | --- | --- |
| `-n, --namespace` | Namespace to use (defaults to active namespace) | |
| `-e, --env` | Environment name to connect to | `local` |
| `-t, --test` | Connect to the integration test database (implies --env=local) | `false` |
| `--shadow` | Connect to the shadow database (implies --env=local) | `false` |
| `--write` | Connect with write privileges | `false` |
| `--admin` | Connect with admin privileges | `false` |
| `--superuser` | Connect as a superuser | `false` |

#### Proxy

Sets up local proxy that forwards any incoming connection to the databases in the specified environment.

```shell
$ encore db proxy [--env=<name>] [flags]
```

**Flags**

| Flag | Description | Default |
| --- | --- | --- |
| `-n, --namespace` | Namespace to use (defaults to active namespace) | |
| `-e, --env` | Environment name to connect to | `local` |
| `-p, --port` | Port to listen on (defaults to a random port) | `0` |
| `-t, --test` | Connect to the integration test database (implies --env=local) | `false` |
| `--shadow` | Connect to the shadow database (implies --env=local) | `false` |
| `--write` | Connect with write privileges | `false` |
| `--admin` | Connect with admin privileges | `false` |
| `--superuser` | Connect as a superuser | `false` |

#### Reset

Resets the databases for the given services. Use --all to reset all databases.

```shell
$ encore db reset <database-names...|--all> [flags]
```

**Flags**

| Flag | Description | Default |
| --- | --- | --- |
| `-n, --namespace` | Namespace to use (defaults to active namespace) | |
| `--all` | Reset all services in the application | `false` |
| `-t, --test` | Reset databases in the test cluster instead | `false` |
| `--shadow` | Reset databases in the shadow cluster instead | `false` |

## Code Generation

Code generation commands

#### Generate client

Generates an API client for your app. For more information about the generated clients, see [this page](/docs/go/cli/client-generation).

By default, `encore gen client` generates the client based on the version of your application currently running in your local environment.
You can change this using the `--env` flag and specifying the environment name.

Use `--lang=<lang>` to specify the language. Supported language codes are:

- `go`: A Go client using the net/http package
- `typescript`: A TypeScript client using the in-browser Fetch API
- `javascript`: A JavaScript client using the in-browser Fetch API
- `openapi`: An OpenAPI spec

```shell
$ encore gen client [<app-id>] [--env=<name>] [--lang=<lang>] [flags]
```

**Flags**

| Flag | Description | Default |
| --- | --- | --- |
| `-l, --lang` | Language to generate code for | |
| `-o, --output` | Filename to write the generated client code to | |
| `-e, --env` | Environment to fetch the API for | `local` |
| `-s, --services` | Names of the services to include in the output | |
| `-x, --excluded-services` | Names of the services to exclude in the output | |
| `-t, --tags` | Names of endpoint tags to include in the output | |
| `--excluded-tags` | Names of endpoint tags to exclude in the output | |
| `--openapi-exclude-private-endpoints` | Exclude private endpoints from the OpenAPI spec | `false` |
| `--ts:shared-types` | Import types from ~backend instead of re-generating them | `false` |
| `--target` | An optional target for the client (`leap`) | |

## Logs

Streams logs from your application

```shell
$ encore logs [--env=prod] [--json] [flags]
```

**Flags**

| Flag | Description |
| --- | --- |
| `-e, --env` | Environment name to stream logs from (defaults to the primary environment) |
| `--json` | Whether to print logs in raw JSON format |
| `-q, --quiet` | Whether to print initial message when the command is waiting for logs |

## Kubernetes

Kubernetes management commands

#### Configure

Updates your kubectl config to point to the Kubernetes cluster(s) for the specified environment

```shell
$ encore k8s configure --env=ENV_NAME
```

## Secrets Management

Secret management commands

#### Set

Set a secret value for a specific environment:

```shell
$ encore secret set --env <env-name> <secret-name>
```

Set a secret value for an environment type:

```shell
$ encore secret set --type <types> <secret-name>
```

Where `<types>` defines which environment types the secret value applies to. Use a comma-separated list of `production`, `development`, `preview`, and `local`. Shorthands: `prod`, `dev`, `pr`.

**Examples**

Entering a secret directly in terminal:

```shell
$ encore secret set --type dev MySecret
Enter secret value: ...
Successfully created secret value for MySecret.
```

Piping a secret from a file:

```shell
$ encore secret set --type dev,local MySecret < my-secret.txt
Successfully created secret value for MySecret.
```

Note that this strips trailing newlines from the secret value.

#### List

Lists secrets, optionally for a specific key

```shell
$ encore secret list [keys...]
```

#### Archive

Archives a secret value

```shell
$ encore secret archive <id>
```

#### Unarchive

Unarchives a secret value

```shell
$ encore secret unarchive <id>
```

## Namespaces

Manage infrastructure namespaces for isolating local infrastructure. See [Infrastructure Namespaces](/docs/go/cli/infra-namespaces) for more details.

#### List

List infrastructure namespaces

```shell
$ encore namespace list [--output=columns|json]
```

#### Create

Create a new infrastructure namespace

```shell
$ encore namespace create NAME
```

#### Delete

Delete an infrastructure namespace

```shell
$ encore namespace delete NAME
```

#### Switch

Switch to a different infrastructure namespace. Subsequent commands will use the given namespace by default.

Use `-` as the namespace name to switch back to the previously active namespace.

```shell
$ encore namespace switch [--create] NAME
```

**Flags**

| Flag | Description |
| --- | --- |
| `-c, --create` | Create the namespace before switching |

## Config

Gets or sets configuration values for customizing the behavior of the Encore CLI.

Configuration options can be set both for individual Encore applications, as well as globally for the local user.

```shell
$ encore config <key> [<value>] [flags]
```

When running `encore config` within an Encore application, it automatically sets and gets configuration for that application. To set or get global configuration, use the `--global` flag.

**Flags**

| Flag | Description |
| --- | --- |
| `--all` | View all settings |
| `--app` | Set the value for the current app |
| `--global` | Set the value at the global level |

## Telemetry

Reports the current telemetry status

```shell
$ encore telemetry
```

#### Enable

Enables telemetry reporting

```shell
$ encore telemetry enable
```

#### Disable

Disables telemetry reporting

```shell
$ encore telemetry disable
```

## MCP

MCP (Model Context Protocol) commands for integrating with AI assistants. See [MCP](/docs/go/cli/mcp) for more details.

#### Start

Starts an SSE-based MCP session and prints the SSE URL

```shell
$ encore mcp start [--app=<app-id>]
```

#### Run

Runs a stdio-based MCP session

```shell
$ encore mcp run [--app=<app-id>]
```

## Random

Utilities for generating cryptographically secure random data.

#### UUID

Generates a random UUID (defaults to version 4)

```shell
$ encore rand uuid [-1|-4|-6|-7]
```

**Flags**

| Flag | Description |
| --- | --- |
| `-1, --v1` | Generate a version 1 UUID |
| `-4, --v4` | Generate a version 4 UUID (default) |
| `-6, --v6` | Generate a version 6 UUID |
| `-7, --v7` | Generate a version 7 UUID |

#### Bytes

Generates random bytes and outputs them in the specified format

```shell
$ encore rand bytes BYTES [-f <format>]
```

**Flags**

| Flag | Description | Default |
| --- | --- | --- |
| `-f, --format` | Output format (`hex\|base32\|base32hex\|base32crockford\|base64\|base64url\|raw`) | `hex` |
| `--no-padding` | Omit padding characters from base32/base64 output | `false` |

#### Words

Generates random 4-5 letter words for memorable passphrases

```shell
$ encore rand words [--sep=SEPARATOR] NUM
```

**Flags**

| Flag | Description | Default |
| --- | --- | --- |
| `-s, --sep` | Separator between words | ` ` (space) |

## Deploy

Deploy an Encore app to a cloud environment.

Requires either `--commit` or `--branch` to be specified.

```shell
$ encore deploy --env=<env-name> (--commit=<sha> | --branch=<name>) [flags]
```

**Flags**

| Flag | Description | Default |
| --- | --- | --- |
| `--app` | App slug to deploy to (defaults to current app) | |
| `-e, --env` | Environment to deploy to (required) | |
| `--commit` | Commit SHA to deploy | |
| `--branch` | Branch to deploy | |
| `-f, --format` | Output format (`text\|json`) | `text` |

## Version

Reports the current version of the encore application

```shell
$ encore version
```

#### Update

Checks for an update of encore and, if one is available, runs the appropriate command to update it.

```shell
$ encore version update
```

## Build

Generates an image for your app, which can be used to [self-host](/docs/go/self-host/docker-build) your app.

#### Docker

Builds a portable Docker image of your Encore application.

```shell
$ encore build docker IMAGE_TAG [flags]
```

**Flags**

| Flag | Description | Default |
| --- | --- | --- |
| `--base` | Base image to build from | `scratch` |
| `-p, --push` | Push image to remote repository | `false` |
| `--cgo` | Enable cgo | `false` |
| `--config` | Infra configuration file path | |
| `--skip-config` | Do not read or generate an infra configuration file | `false` |
| `--services` | Services to include in the image | |
| `--gateways` | Gateways to include in the image | |
| `--os` | Target operating system | `linux` |
| `--arch` | Target architecture (`amd64\|arm64`) | `amd64` |

## LLM Rules

Generate LLM rules in an existing app

#### Init

Initialize the LLM rules files

```shell
$ encore llm-rules init [flags]
```

**Flags**

| Flag | Description |
| --- | --- |
| `-r, --llm-rules` | Initialize the app with LLM rules for a specific tool (`cursor\|claudecode\|vscode\|agentsmd\|zed`) |
