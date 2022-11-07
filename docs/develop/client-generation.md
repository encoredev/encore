---
seotitle: Automatic API Client Generation
seodesc: Learn how you can use automatic API client generation to get clients for your backend. See how to integrate with your frontend using a type-safe generated client.
title: Client Library Generation
subtitle: Stop writing the same types everywhere
---

One of Encore's core principles is writing a scalable distributed backend should be no more difficult than writing a
normal application, where you simply make function calls between your various packages which Encore translates into RPC calls.
The same approach applies to how Encore generates API clients; it generates an API interface which looks like the original
Go function, with the same parameters and response signature as the server.

The generated clients are all designed to be single files written in the same way you would write them without using
any additional dependencies apart from the standard functionality of the target language with full type safety.
This is to allow anybody to look at the generated client and understand exactly how it works.

The precise structure of the generated code depends on the language, to make sure it's idiomatic and easy to use,
but always includes all the publicly accessible endpoints, data structures, and documentation strings.

Currently, Encore supports generating clients for:
- `go` - using `net/http` for the underlying HTTP transport.
- `typescript` - using the browser `fetch` API for the underlying HTTP client.
- `javascript` - using the browser `fetch` API for the underlying HTTP client.


If there's another language you think Encore should support, please either submit a pull request or create a feature
request on [GitHub](https://github.com/encoredev/encore/issues/new), or [reach out on Slack](/slack).

<Callout type="warning">

If you ship the generated client to end customers, it's important to note that with any public API, old clients will
continue to be used for some time after you've made changes. So it's always best to avoid making breaking changes in any
API's that your clients access otherwise your generated clients could stop working or cause hard to debug issues.

</Callout>

<br />

# Generating Clients

To generate a client, download the [Encore CLI](/docs/install#install-the-encore-cli) and run
```shell
$ encore gen client <app-id> --lang=<lang>
```

**Environment Selection**

By default, this command will generate the client for the version of the application currently deployed on the primary [environment](/docs/deploy/environments)
of your application. You can change this using the `--env` flag and specifying the environment name.

If you want to generate the client for the version of your application you have local running, then you can use the
special environment name `local` (you'll need to be running the application first).

<Callout type="info">

The generated client can be used with any environment, not just the one it was generated for. However, the APIs, data structures
and marshalling logic will be based on whatever is present and running in that environment at the point in time the client is generated.

</Callout>


**Output Mode**

By default the client's code will be output to stdout, allowing you to pipe it into your clipboard, or another tool. However,
using `--output` you can specify a file location to write the client to. If output is specified, you do not need to specify
the language as Encore will detect the language based on the file extension.


### Example Script
You could combine this into a `package.json` file for your Typescript frontend, to allow you to run `npm run gen` in that
project to update the client to match the code running in your staging environment.
```json
{
  "scripts": {
    // ...
    "gen": "encore gen client hello-a8bc --output=./client.ts --env=staging"
    // ...
  }
}
```

# Using the Client

The generated client has all the data structures required as parameters or returned as response values as needed by any
of the public or authenticated API's of your Encore application. Each service is exposed as object on the client, with
each public or authenticated API exposed as a function on those objects.

For instance, if you had a service called `email` with a function `Send`, on the generated client you would call this
using; `client.email.Send(...)`.

## Creating an instance

When constructing a client, you need to pass a `BaseURL` as the first parameter; this is the URL at which the API can
be accessed. The client provides two helpers:

- `Local` - This is a constant provided, which will always point at your locally running instance environment.
- `Environment("name")` - This is a function which allows you to specify an environment by name

However, BaseURL is a string, so if the two helpers do not provide enough flexibility you can pass any valid URL to be
used as the BaseURL.

### Authentication

If your application has any API's which require [authentication](/docs/develop/auth), then additional options will generated
into the client, which can be used when constructing the client. Just like with API's schemas, the data type required by
your application's `auth handler` will be part of the client library, allowing you to set it in two ways:

If your credentials wont change during the lifetime of the client, simply passing the authentication data to the client
through the `WithAuth` (Go) or `auth` (TypeScript) options.

However, if the authentication credentials can change, you can also pass a function which will be called before each request
and can return a new instance of the authentication data structure or return the existing instance.


### HTTP Client Override

If required, you can override the underlying HTTP implementation with your own implementation. This is useful if you want
to perform logging of the requests being made, or route the traffic over a secured tunnel such as a VPN.

In Go this can be configured using the `WithHTTPClient` option. You are required to provide an implementation of the
`HTTPDoer` interface, which the [http.Client](https://pkg.go.dev/net/http#Client) implements. For TypeScript clients,
this can be configured using the `fetcher` option and must conform to the same prototype as the browsers inbuilt [fetch
API](https://developer.mozilla.org/en-US/docs/Web/API/fetch).

## Structured Errors

Errors created or wrapped using Encore's [`errs package`](/docs/develop/errors) will be returned to the client and deserialized
as an `APIError`, allowing the client to perform adaptive error handling based on the type of error returned. You can perform
a type check on errors caused by calling an API to see if it is an `APIError`, and once cast as an `APIError` you can access
the `Code`, `Message` and `Details` fields. For TypeScript Encore generates a `isAPIError` type guard which can be used.

The `Code` field is an enum with all the possible values generated in the library, alone with description of when we
would expect them to be returned by your API. See the [errors documentation](/docs/develop/errors#error-codes) for
an online reference of this list.

## Example CLI Tool

For instance, we could build a simple CLI application to use our [url shortener](/docs/tutorials/rest-api), and handle
any structured errors in a way which makes sense for that error code.

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "shorten_cli/client"
)

func main() {
    // Create a new client with the default BaseURL
    client, err := client.New(
        client.Environment("production"),
        client.WithAuth(os.Getenv("SHORTEN_API_KEY")),
    )
    if err != nil {
        panic(err)
    }

    // Timeout if the request takes more than 5 seconds
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Call the Shorten function in the URL service
    resp, err := client.Url.Shorten(
        ctx,
        client.UrlShortenParams{ URL: os.Args[1] },
    )
    if err != nil {
        // Check the error returned
        if err, ok := err.(*client.APIError); ok {
            switch err.Code {
            case client.ErrUnauthenticated:
                fmt.Println("SHORTEN_API_KEY was invalid, please check your environment")
                os.Exit(1)
            case client.ErrAlreadyExists:
                fmt.Println("The URL you provided was already shortened")
                os.Exit(0)
            }
        }
        panic(err) // if here then something has gone wrong in an unexpected way
    }
    fmt.Printf("https://short.encr.app/%s", resp.ID)
}
```
