---
seotitle: Infrastructure Namespaces
seodesc: Learn how Encore's infrastructure namespaces makes it easy to task switch. Stash your infrastructure state and switch to a different task with a single command.
title: Infrastructure Namespaces
subtitle: Task switching made easy
lang: ts
---

Encore's CLI allows you to create and switch between multiple, independent *infrastructure namespaces*.
Infrastructure namespaces are isolated from each other, and each namespace contains its own independent data.

This makes it trivial to switch tasks, confident your old state and data will be waiting for you when you return.

If you've ever worked on a new feature that involves making changes to the database schema,
only to context switch to reviewing a Pull Request and had to reset your database, you know the feeling.

With Encore's infrastructure namespaces, this is a problem of the past.
Run `encore namespace switch --create pr:123` (or `encore ns switch -c pr:123` for short) to create and switch to a new namespace.

The next `encore run` will run in the new namespace, with a completely fresh database.
When you're done, run `encore namespace switch -` to switch back to your previous namespace.

## Usage

Below are the commands for working with namespaces.
Note that you can use `encore ns` as a short form for `encore namespace`.

```shell
# List your namespaces (* indicates the current namespace)
$ encore namespace list

# Create a new namespace
$ encore namespace create my-ns

# Switch to a namespace
$ encore namespace switch my-ns

# Switch to a namespace, creating it if it doesn't exist
$ encore namespace switch --create my-ns

# Switch to the previous namespace
$ encore namespace switch -

# Delete a namespace (and all associated data)
$ encore namespace delete my-ns
```

Most other Encore commands that interact or use infrastructure take an optional
`--namespace` (`-n` for short) that overrides the current namespace. If left unspecified,
the current namespace is used.

For example:

```shell
# Run the app using the "my-ns" namespace
$ encore run --namespace my-ns

# Open a database shell to the "my-ns" namespace
$ encore db shell DATABASE_NAME --namespace my-ns

# Reset all databases within the "my-ns" namespace
$ encore db reset --all --namespace my-ns
```
