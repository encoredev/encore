---
seotitle: API Calls with Encore.go
seodesc: Learn how to make type-safe API calls in Go with Encore.go
title: API Calls
subtitle: Making API calls is as simple as making function calls
lang: go
---

Calling an API endpoint looks like a regular function call with Encore.go. To call an endpoint you first import the other service as a Go package using `import "encore.app/package-name"` and then call the API endpoint like a regular function. Encore will automatically generate the necessary boilerplate at compile-time.

In the example below, we import the service package `hello` and call the `Ping` endpoint using a function call to `hello.Ping`.

```go
import "encore.app/hello" // import service

//encore:api public
func MyOtherAPI(ctx context.Context) error {
    resp, err := hello.Ping(ctx, &hello.PingParams{Name: "World"})
    if err == nil {
        log.Println(resp.Message) // "Hello, World!"
    }
    return err
}
```

<GitHubLink 
    href="https://github.com/encoredev/examples/tree/main/trello-clone" 
    desc="Simple microservices example application with service-to-service API calls." 
/>

This means your development workflow is as simple as building a monolith, even if you use multiple services.
You also get all the benefits of function calls, like compile-time checking of all the parameters and auto-completion in your editor, while still allowing the division of code into logical components, services, and systems.

Then when building your application, Encore uses [static analysis](/docs/go/concepts/application-model) to parse all API calls and compiles them to proper API calls.

## Current Request

By using Encore's [current request API](https://pkg.go.dev/encore.dev/#Request) you can get meta-information about the
current request. Including the type of request, the time the request started, the service and endpoint called and the path
which was called on the service.

For more information, see the [metadata documentation](/docs/go/develop/metadata).
