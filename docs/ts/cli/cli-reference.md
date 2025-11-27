---
seotitle: Encore CLI Reference
seodesc: The Encore CLI lets you run your local development environment, create apps, and much more. See all CLI commands in this reference guide.
title: CLI Reference
subtitle: The Encore CLI lets you run your local environment and much more.
lang: ts
---

## Running

#### Run

Runs your application.

```shell
$ encore run [--debug] [--watch=true] [--port NUMBER] [flags]
```

#### Test

Tests your application

Takes all the same flags as `go test`.

```shell
$ encore test ./... [go test flags]
```

#### Check

Checks your application for compile-time errors using Encore's compiler.

```shell
$ encore check
```

#### Exec

Runs executable scripts against the local Encore app.

Takes a command that it will execute with the local Encore app environment setup.

```
$ encore exec -- <command>
```

##### Example

Run a database seed script

```
$ encore exec -- npx tsx ./seed.ts
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
$ encore app create [name]
```

### Init

Create a new Encore app from an existing repository

```shell
$ encore app init [name]
```

#### Link

Link an Encore app with the server

```shell
$ encore app link [app-id]
```

## Auth

Commands to authenticate with Encore

#### Login

Log in to Encore

```shell
$ encore auth login
```

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

```shell
$ encore db shell <database-name> [--env=<name>]
```

`encore db shell` defaults to read-only permissions. Use `--write`, `--admin` and `--superuser` flags to modify which permissions you connect with.

#### Connection URI

Outputs a database connection string for `<database-name>`. Defaults to connecting to your local environment. Specify --env to connect to another environment.

```shell
$ encore db conn-uri <database-name> [--env=<name>] [flags]
```

#### Proxy

Sets up local proxy that forwards any incoming connection to the databases in the specified environment.

```shell
$ encore db proxy [--env=<name>] [flags]
```

#### Reset

Resets the databases for the given services. Use --all to reset all databases.

```shell
$ encore db reset [service-names...] [flags]
```

## Code Generation

Code generation commands

#### Generate client

Generates an API client for your app. For more information about the generated clients, see [this page](/docs/ts/cli/client-generation).

By default, `encore gen client` generates the client based on the version of your application currently running in your local environment.
You can change this using the `--env` flag and specifying the environment name.

Use `--lang=<lang>` to specify the language. Supported language codes are:
- `go`: A Go client using the net/http package
- `typescript`: A TypeScript client using the in-browser Fetch API
- `javascript`: A JavaScript client using the in-browser Fetch API
- `openapi`: An OpenAPI spec


```shell
$ encore gen client [<app-id>] [--env=<name>] [--services=foo,bar] [--excluded-services=baz,qux] [--lang=<lang>] [flags]
```

## Logs

Streams logs from your application

```shell
$ encore logs [--env=prod] [--json]
```

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

Sets a secret value

```shell
$ encore secret set --type <types> <secret-name>
```

Where `<types>` defines which environment types the secret value applies to. Use a comma-separated list of `production`, `development`, `preview`, and `local`. Shorthands: `prod`, `dev`, `pr`.

**Examples**


Entering a secret directly in terminal:

	$ encore secret set --type dev MySecret
	Enter secret value: ...
	Successfully created secret value for MySecret.

Piping a secret from a file:

	$ encore secret set --type dev,local MySecret < my-secret.txt
	Successfully created secret value for MySecret.

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
$  encore secret unarchive <id>
```


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

## VPN

VPN management commands

#### Start

Sets up a secure connection to private environments

```shell
$ encore vpn start
```

#### Status

Determines the status of the VPN connection

```shell
$ encore vpn status
```

#### Stop

Stops the VPN connection

```shell
$ encore vpn stop
```
## Build

Generates an image for your app, which can be used to [self-host](/docs/ts/self-host/build) your app.

#### Docker

Builds a portable Docker image of your Encore application.

```shell
$ encore build docker
```

**Flags**

`--base string` defines the base image to build from (default "scratch")
`--push` pushes image to remote repository

## LLM Rules

Generate llm rules in an existing app

#### Init

Initialize the llm rules files

```shell
$ encore llm-rules init
```
