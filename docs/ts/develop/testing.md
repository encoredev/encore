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

<GitHubLink 
    href="https://github.com/encoredev/examples/tree/main/ts/uptime" 
    desc="Uptime monitoring app with API endpoint unit tests written in Vitest." 
/>

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

```jsonc
"vitest.commandLine": "encore test"
```
As of Vitest plugin version 0.5 ([issue](https://github.com/vitest-dev/vscode/issues/306)), environment configuration requires an updated approach. The following configuration is required to ensure proper functionality:

```jsonc
"vitest.nodeEnv": {
    // generated with `encore daemon env | grep ENCORE_RUNTIME_LIB | cut -d'=' -f2`
    "ENCORE_RUNTIME_LIB": "/opt/homebrew/Cellar/encore/1.44.5/libexec/runtimes/js/encore-runtime.node"
}
```

When running tests within VSCode, file-level parallel execution must be disabled. Update your `vite.config.ts` as follows:

```typescript
// File vite.config.ts
export default defineConfig({
  resolve: {
    alias: {
      "~encore": path.resolve(__dirname, "./encore.gen"),
    },
  },
  test: {
    fileParallelism: false,
  },
});
```

To improve the performance in CI, you can re-enable the parallel execution by overwriting the config in cli `encore test --fileParallelism=true`.
