---
seotitle: Use structured logging to understand your application
seodesc: Learn how to use structured logging, a combination of free-form log messages and type-safe key-value pairs, to understand your backend application's behavior.
title: Logging
subtitle: Structured logging helps you understand your application
lang: ts
infobox: {
  title: "Structured Logging",
  import: "encore.dev/log",
}
---

Encore offers built-in support for Structured Logging, which combines a free-form log message with structured and type-safe key-value pairs. This enables straightforward analysis of what your application is doing, in a way that is easy for a computer to parse, analyze, and index. This makes it simple to quickly filter and search through logs.

Encore’s logging is integrated with the built-in [Distributed Tracing](/docs/ts/observability/tracing) functionality, and all logs are automatically included in the active trace. This dramatically simplifies debugging of your application.

## Usage
First, add `import log  from "encore.dev/log";` to your module. Then call one of the logging functions `error`, `warn`, `info`, `debug`, or `trace` to emit a log message. For example:

```ts
log.info("log message", {is_subscriber: true})
log.error(err, "something went terribly wrong!")
```

The first parameter is the log message (or optionally an error for the error function) . After that follows a single object with key-value pairs for structured logging.

If you’re logging many log messages with the same key-value pairs each time it can be a bit cumbersome. To help with that, use `log.with()` to group them into a Logger object, which then copies the key-value pairs into each log event:

```ts
const logger = log.with({is_subscriber: true})
logger.info("user logged in", {login_method: "oauth"}) // includes is_subscriber=true
```

## Live-streaming logs

Encore also makes it simple to live-stream logs directly to your terminal, from any environment, by running:

```
$ encore logs --env=prod
```
