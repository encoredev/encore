---
seotitle: Automatic API Client Generation
seodesc: Learn how you can use automatic API client generation to get clients for your backend. See how to integrate with your frontend using a type-safe generated client.
title: Client Library Generation
subtitle: Stop writing the same types everywhere
lang: ts
---

Encore makes it simple to write scalable distributed backends by allowing you to make function calls that Encore translates into RPC calls. Encore also generates API clients with interfaces that look like the original Go functions, with the same parameters and response signature as the server.

The generated clients are single files that use only the standard functionality of the target language, with full type safety. This allow anyone to look at the generated client and understand exactly how it works.

The structure of the generated code varies by language, to ensure it's idiomatic and easy to use, but always includes all publicly accessible endpoints, data structures, and documentation strings.

Encore currently supports generating the following clients:
- **Go** - Using `net/http` for the underlying HTTP transport.
- **TypeScript** - Using the browser `fetch` API for the underlying HTTP client.
- **JavaScript** - Using the browser `fetch` API for the underlying HTTP client.
- **OpenAPI** - Using the OpenAPI Specification's language-agnostic interface to HTTP APIs. (Experimental)

If there's a language you think should be added, please submit a pull request or create a feature
request on [GitHub](https://github.com/encoredev/encore/issues/new), or [reach out on Discord](/discord).

<Callout type="important">

If you ship the generated client to end customers, keep in mind that old clients will continue to be used after you make changes. To prevent issues with the generated clients, avoid making breaking changes in APIs that your clients access.

</Callout>

<br />

## Generating a Client

To generate a client, use the `encore gen client` command. It generates a type-safe client using the most recent API metadata
running in a particular environment for the given Encore application. For example:

```shell
# Generate a TypeScript client for calling the hello-a8bc application based on the primary environment
encore gen client hello-a8bc --output=./client.ts

# Generate a Go client for the hello-a8bc application based on the locally running code
encore gen client hello-a8bc --output=./client.go --env=local

# Generate an OpenAPI client for the hello-a8bc application based on the primary environment
encore gen client hello-a8bc --lang=openapi --output=./openapi.json
```

### Environment Selection

By default, `encore gen client` generates the client based on the version of your application currently running in your local environment.
You can change this using the `--env` flag and specifying the environment name.

<Callout type="info">

The generated client can be used with any environment, not just the one it was generated for. However, the APIs, data structures
and marshalling logic will be based on whatever is present and running in that environment at the point in time the client is generated.

</Callout>

### Service filtering

By default `encore gen client` outputs code for all services with at least one publicly accessible (or authenticated) API.
You can narrow down this set of services by specifying the `--services` (or `-s`) flag. It takes a comma-separated list
of service names.

For example, to generate a typescript client for the `email` and `users` services, run:
```shell
encore gen client --services=email,users -o client.ts
```

### Output Mode

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

## Using the Client

The generated client has all the data structures required as parameters or returned as response values as needed by any
of the public or authenticated API's of your Encore application. Each service is exposed as object on the client, with
each public or authenticated API exposed as a function on those objects.

For instance, if you had a service called `email` with a function `Send`, on the generated client you would call this
using; `client.email.Send(...)`.

### Creating an instance

When constructing a client, you need to pass a `BaseURL` as the first parameter; this is the URL at which the API can
be accessed. The client provides two helpers:

- `Local` - This is a constant provided, which will always point at your locally running instance environment.
- `Environment("name")` - This is a function which allows you to specify an environment by name

However, BaseURL is a string, so if the two helpers do not provide enough flexibility you can pass any valid URL to be
used as the BaseURL.

### Authentication

If your application has any API's which require [authentication](/docs/ts/develop/auth), then additional options will generated
into the client, which can be used when constructing the client. Just like with API's schemas, the data type required by
your application's `auth handler` will be part of the client library, allowing you to set it in two ways:

If your credentials won't change during the lifetime of the client, simply passing the authentication data to the client
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

### Structured Errors

Errors created or wrapped using Encore's [`errs package`](/docs/ts/primitives/errors) will be returned to the client and deserialized
as an `APIError`, allowing the client to perform adaptive error handling based on the type of error returned. You can perform
a type check on errors caused by calling an API to see if it is an `APIError`, and once cast as an `APIError` you can access
the `Code`, `Message` and `Details` fields. For TypeScript Encore generates a `isAPIError` type guard which can be used.

The `Code` field is an enum with all the possible values generated in the library, alone with description of when we
would expect them to be returned by your API. See the [errors documentation](/docs/ts/primitives/errors#error-codes) for
an online reference of this list.
