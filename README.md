# Encore - The Go backend framework with superpowers

<img align="right" width="189px" src="https://encore.dev/assets/img/encore-gopher.svg">

https://encore.dev

Encore is a Go backend framework for rapidly creating APIs and distributed systems.

It uses static analysis and code generation to reduce the boilerplate you have to write,
resulting in an extremely productive developer experience.

The key features of Encore are:

* **No boilerplate**: Encore drastically reduces the boilerplate needed to set up
  a production ready backend application. Define backend services, API endpoints,
  and call APIs with a single line of Go code. 

* **Distributed Tracing**: Encore uses a combination of static analysis and code
  generation to automatically instrument your application for excellent observability.
  Automatically captures information about API calls, goroutines, HTTP requests,
  database queries, and more. Automatically works for local development as well
  as in production.

* **Infrastructure Provisioning**: Encore understands how your application works,
  and uses that understanding to provision and manage your cloud infrastructure.
  Automatically works with all the major cloud providers, as well as for local development.

* **Simple Secrets**: Encore makes it easy to store and securely use secrets and API keys. 
  Never worry about how to store and get access to secret values again.

* **API Documentation**: Encore parses your source code to understand the request/response
  schemas for all your APIs. Encore can automatically generate high-quality, interactive
  API Documentation for you. It can also automatically generate type-safe, documented
  clients for your frontends.

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

#### Setup Demo
[![Setup demo](https://asciinema.org/a/406681.svg)](https://asciinema.org/a/406681)

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

## Using Encore

Encore makes it super easy to create backend services and APIs.

### Creating a service with an API

In Encore, a backend service is just a regular Go package with one or more APIs defined.
The Go package name becomes the service name (which must be unique within your app).

```go
package greet

import (
    "context"
    "fmt"
)

type Params struct {
    Name string
}

type Response struct {
    Message string
}

//encore:api public
func Person(ctx context.Context, params *Params) (*Response, error) {
    msg := fmt.Sprintf("Hello, %s!", params.Name)
    return &Response{Message: msg}, nil
}
```

This creates a backend service named `greet`, with a single API endpoint named `Person`.

Calling it is easy:
```bash
$ encore run  # run the app in a separate terminal
$ curl http://localhost:4060/greet.Person -d '{"Name": "Jane"}'
# Outputs: {"Message": "Hello, Jane!"}
```

[Learn more in the Encore docs](https://encore.dev/docs/concepts/services-and-apis).

### Calling an API endpoint
Calling an API endpoint from another endpoint is easy.

Just import the service (with a regular Go import), and then call the function
as if it were a regular Go function:

```go
import "my.app/greet"

func MyAPI(ctx context.Context) error {
    resp, err := greet.Person(ctx, &greet.Params{Name: "John"})
    if err != nil {
        fmt.Println("The greeting message is:", resp.Message)
    }
    return err
}
```

Encore uses its static analysis and code generation to turn this into a proper API call.

[Learn more in the Encore docs](https://encore.dev/docs/concepts/services-and-apis).

### SQL Databases

Encore automatically provisions, connects to, and performs schema migrations of SQL databases for you.

All you have to do is define the SQL migrations:

```sql
-- greet/migrations/1_create_table.up.sql
CREATE TABLE person (
    name TEXT PRIMARY KEY,
    count INT NOT NULL
);
```

Then import `encore.dev/storage/sqldb` and just start querying:

```go
// genGreeting generates a personalized greeting for the given name.
func genGreeting(ctx context.Context, name string) (string, error) {
    var count int
    // Insert the row, and increment count if the row is already in the db.
    err := sqldb.QueryRow(ctx, `
        INSERT INTO "person" (name, count)
        VALUES ($1, 1)
        ON CONFLICT (name) DO UPDATE
        SET count = person.count + 1
        RETURNING count
    `, name).Scan(&count)
    if err != nil {
        return "", err
    }

    switch count {
    case 1:
        return fmt.Sprintf("Nice to meet you, %s!", name), nil
    case 2:
        return fmt.Sprintf("Hi again, %s!", name), nil
    default:
        return fmt.Sprintf("Good to see you, %s! We've met %d times before.", name, count-1), nil
    }
}
```

#### Database Demo
[![Setting up a database](https://asciinema.org/a/406695.svg)](https://asciinema.org/a/406695)

[Learn more in the Encore docs](https://encore.dev/docs/concepts/databases).

### API Documentation

Encore automatically generates API documentation for your app.

You can access it by viewing the local development dashboard by opening the API URL
in your browser when your app is running (normally [localhost:4060](http://localhost:4060)).

[![API Documentation](https://encore.dev/assets/img/api-docs-screenshot.png)](https://encore.dev/docs/concepts/api-docs)

### Distributed Tracing

Encore automatically instruments your app with Distributed Tracing.

For local development you can access it by viewing the local development dashboard by opening the API URL
in your browser when your app is running (normally [localhost:4060](http://localhost:4060)).

Any API calls to your app automatically produces traces.

![Automatic Tracing](https://encore.dev/assets/img/tracing.jpg)

## Developing Encore and building from source

See [DEVELOPING.md](DEVELOPING.md).

## Questions & Feedback

If you have questions, need help, or have any feedback: email [hello@encore.dev](mailto:hello@encore.dev) or join our [Slack channel](https://join.slack.com/t/encoredev/shared_invite/zt-c75mzbnb-kWCiGueYVJ4pUCIW45sb8A).
