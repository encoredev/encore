---
seotitle: How to use `encore exec` for running scripts
seodesc: Learn how to use the `encore exec` command to run scripts like database seeding in your Encore app.
title: Running Scripts
subtitle: Run scripts with your application's infrastructure and runtime configured and initialized
lang: ts
---
In local development, you may need to run scripts or commands, such as seeding a database with initial data.
For that to work the database needs to be started, and the Encore runtime needs to be configured and initialized.

## Using `encore exec`

The `encore exec` command allows you to execute custom commands while leveraging Encore's infrastructure setup. This is particularly useful for tasks like database seeding, running scripts, or other one-off commands that require the app's environment to be initialized.

### How it works

The `encore exec` command initializes the required infrastructure for your Encore app and executes the specified command.
This ensures that your commands run in the correct context with all dependencies properly configured.

### Example: Database Seeding

In this example, `npx tsx ./seed.ts` runs a TypeScript script (`seed.ts`) to populate the database with initial data

```bash
encore exec -- npx tsx ./seed.ts
```

Here’s what happens:
1. Encore initializes the app infrastructure.
2. The `npx tsx ./seed.ts` command is executed in the context of the initialized app.

### General Syntax

```bash
encore exec -- <your-command>
```

Substitute `<your-command>` with the specific command you wish to run.

### Use Cases

- **Database Seeding**: Populate your database with initial data using a script.
- **Client Generation**: Generate a client for interacting with an external dependency.
- **Custom Scripts**: Run any script that depends on the app's initialized environment.

### Notes

- Ensure that the command you provide is executable in your environment.
- Use `--` to separate `encore exec` options from the command you want to run.

