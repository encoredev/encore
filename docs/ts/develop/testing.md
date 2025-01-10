---
seotitle: Automated testing for your backend application
seodesc: Learn how create automated tests for your microservices backend application, and run them automatically on deploy using Go and Encore.
title: Automated testing
subtitle: Confidence at speed
lang: ts
---

Encore provides built-in testing tools that make it simple to test your application using a variety of test runners.

To run tests with Encore:

1. Configure the `test` command in your `package.json` to use the test runner of your choice.
2. Configure your test runner.
3. Run `encore test` from the CLI.

The `encore test` command automatically sets up all necessary infrastructure in test mode before running your tests.

<GitHubLink 
    href="https://github.com/encoredev/examples/tree/main/ts/uptime" 
    desc="Uptime monitoring app with API endpoint unit tests written in Vitest." 
/>

## Recommended Setup: Vitest

We recommend [Vitest](https://vitest.dev) as your test runner because it offers:
- Fast execution
- Native ESM and TypeScript support
- Jest API compatibility

### Setting up Vitest

1. Create `vite.config.json` in your application's root directory:

```ts
/// <reference types="vitest" />
import { defineConfig } from "vite";
import path from "path";

export default defineConfig({
  resolve: {
    alias: {
      "~encore": path.resolve(__dirname, "./encore.gen"),
    },
  },
});
```

2. Update your `package.json` to include:

```json
{
  "scripts": {
    "test": "vitest"
  }
}
```

You're done! Now you can run your tests with `encore test`.

### Optional: IDE Integration

#### VS Code Setup

If using Vitest, follow these steps:
1. Install the official Vitest VS Code extension
2. Add to `.vscode/settings.json`:

```json
{
  "vitest.commandLine": "encore test"
}
```

## Integration Testing Best Practices

Encore applications typically focus on integration tests rather than unit tests because:

- Encore eliminates most boilerplate code
- Your code primarily consists of business logic involving databases and inter-service API calls
- Integration tests better verify this type of functionality

### Test Environment Benefits

When running tests, Encore automatically:
- Sets up separate test databases
- Configures databases for optimal test performance by:
  - Skipping `fsync`
  - Using in-memory filesystems
  - Removing durability overhead

These optimizations make integration tests nearly as fast as unit tests.

