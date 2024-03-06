---
seotitle: Automated testing for your backend application
seodesc: Learn how create automated tests for your microservices backend application, and run them automatically on deploy using Go and Encore.
title: Automated testing
subtitle: Confidence at speed
infobox: {
  title: "Testing",
  import: "encore.dev/et",
}
lang: go
---

Go comes with excellent built-in support for automated tests.
Encore builds on top of this foundation, and lets you write tests in exactly the same way.
We won't cover the basics of how to write tests here, see [the official Go docs](https://golang.org/pkg/testing/) for that.
Let's instead focus on the difference between testing in Encore compared to a standard Go application.

The main difference is that since Encore requires an extra compilation step,
you must run your tests using `encore test` instead of `go test`. This is
a wrapper that compiles the Encore app and then runs `go test`. It supports
all the same flags that the `go test` command does.

For example, use `encore test ./...` to run tests in all sub-directories,
or just `encore test` for the current directory.

## Test tracing

Encore comes with built-in test tracing for local development.

You only need to open Encore's local development dashboard at [localhost:9400](http://localhost:9400) to see traces for all your tests.
This makes it very simple to understand the root cause for why a test is failing.

<img className="w-full d:w-3/4 h-auto" src="/assets/docs/test_trace.png" title="Test tracing" />


## Integration testing

Since Encore removes almost all boilerplate, most of the code you write
is business logic that involves databases and calling APIs between services.
Such behavior is most easily tested with integration tests.

When running tests, Encore automatically sets up the databases you need
in a separate database cluster. They are additionally configured to skip `fsync`
and to use an in-memory filesystem since durability is not a concern for automated tests.

This drastically reduces the speed overhead of writing integration tests.

In general, Encore applications tend to focus more on integration tests
compared to traditional applications that are heavier on unit tests.
This is nothing to worry about and is the recommended best practice.

### Service Structs

In tests, [service structs](/docs/primitives/services-and-apis/service-structs) are initialised on demand when the first
API call is made to that service and then that instance of the service struct for all future tests. This means your tests
can run faster as they don't have to each initialise all the service struct's each time a new test starts.

However, in some situations you might be storing state in the service struct that would interfere with other tests. When
you have a test you want to have it's own instance of the service struct, you can use the `et.EnableServiceInstanceIsolation()` function within the test to enable this for just that test, while the rest of your tests will continue to use the shared instance.

## Test-only infrastructure

Encore allows tests to define infrastructure resources specifically for testing.
This can be useful for testing library code that interacts with infrastructure.

For example, the [x.encore.dev/pubsub/outbox](https://pkg.go.dev/x.encore.dev/infra/pubsub/outbox) package
defines a test-only database that is used to do integration testing of the outbox functionality.

## Testing from your IDE

### GoLand / IntelliJ

Encore has an officially supported plugin [available in the JetBrains marketplace](https://plugins.jetbrains.com/plugin/20010-encore).

It lets you run unit tests directly from within your IDE with support for debug mode and breakpoints.

### Visual Studio Code (VS Code)

There's no official VS Code plugin available yet, but we are happy to include your contribution if you  build one. Reach out on [Slack](/slack) if you need help to get started.

For advice on debugging when using VS Code, see the [Debugging docs](/docs/how-to/debug).
