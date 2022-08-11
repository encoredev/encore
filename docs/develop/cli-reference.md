## Running

#### Run

Runs your application.

```bash
encore run [--debug] [--watch=true] [flags]
```

#### Test

Tests your application

Takes all the same flags as `go test`.

```bash
encore test ./... [go test flags]
```

#### Check

Checks your application for compile-time errors using Encore's compiler.

```bash
encore check
```

## App

Commands to create and link Encore apps

#### Clone

Clone an Encore app to your computer

```bash
encore app clone [app-id] [directory]
```

#### Create

Create a new Encore app

```bash
encore app create [name]
```

#### Link

Link an Encore app with the server

```bash
encore app link [app-id]
```

## Auth

Commands to authenticate with Encore

#### Login

Log in to Encore

```bash
encore auth login
```

#### Logout

Logs out the currently logged in user

```bash
encore auth logout
```

#### Signup

Create a new Encore account

```bash
encore auth signup
```

#### Whoami

Show the current logged in user

```bash
encore auth whoami
```

## Database Management

Database management commands

#### Connection URI

Outputs the database connection string

```bash
encore db conn-uri [servicename] [flags]
```

#### Proxy

Sets up a proxy tunnel to the database

```bash
encore db proxy [--env=<name>] [flags]
```

#### Reset

Resets the databases for the given services. Use --all to reset all databases.

```bash
encore db reset [service-names...] [flags]
```

#### Shell

Connects to the database via psql shell

Defaults to connecting to your local environment. Specify --env to connect to another environment.

```bash
encore db shell [service-name] [--env=local]
```

## Code Generation

Code generation commands

#### Generate client

Generates an API client for your app. For more information about the generated clients, see [this page](/docs/develop/client-generation).

By default generates the API based on your primary production environment.
Use '--env=local' to generate it based on your local development version of the app.

Supported language codes are:
- go: A Go client using the net/http package
- typescript: A TypeScript-client using the in-browser Fetch API


```bash
encore gen client <app-id> [--env=prod] [flags]
```

## Logs

Streams logs from your application

```bash
encore logs [--env=prod] [--json]
```

## Secrets Management

Secret management commands

#### Set

Sets a secret value

```bash
encore secret set --dev|prod <key>
```

**Examples**


Entering a secret directly in terminal:

	$ encore secret set --dev MySecret
	Enter secret value: ...
	Successfully created development secret MySecret.

Piping a secret from a file:

	$ encore secret set --dev MySecret < my-secret.txt
	Successfully created development secret MySecret.

Note that this strips trailing newlines from the secret value.

## Version

Reports the current version of the encore application

```bash
encore version
```

#### Update

Checks for an update of encore and, if one is available, runs the appropriate command to update it.

```bash
encore version update
```

## VPN

VPN management commands

#### Start

Sets up a secure connection to private environments

```bash
encore vpn start
```

#### Status

Determines the status of the VPN connection

```bash
encore vpn status
```

#### Stop

Stops the VPN connection

```bash
encore vpn stop
```
