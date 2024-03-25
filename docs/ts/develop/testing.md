---
seotitle: Automated testing for your backend application
seodesc: Learn how create automated tests for your microservices backend application, and run them automatically on deploy using Go and Encore.
title: Automated testing
subtitle: Confidence at speed
lang: ts
---

Encore provides a suite of built-in tooling to simplify testing your application.

To run your tests, configure the `test` command in your `package.json` to the test runner of your choice,
and then use `encore test` from the CLI. 
The `encore test` command sets up all the necessary infrastructure in test mode before handing over to
the test runner. 

## Test Runners

We recommend using [Vitest](https://vitest.dev) as the test runner. It's very fast, has native support for
ESM and TypeScript, and has a built-in compatibility layer for Jest's API.

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

## Testing from your IDE

### Visual Studio Code (VS Code)

If you're using Vitest, install the official Vitest VS Code extension and then add to the `.vscode/settings.json` file:

```
"vitest.commandLine": "encore test"
```

