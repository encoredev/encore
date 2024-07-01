---
seotitle: Use structured logging to understand your application
seodesc: Learn how to use structured logging, a combination of free-form log messages and type-safe key-value pairs, to understand your backend application's behavior.
title: Logging
subtitle: Structured logging helps you understand your application
infobox: {
  title: "Structured Logging",
  import: "encore.dev/rlog",
}
---

Encore offers built-in support for Structured Logging, which combines a free-form log message with structured and type-safe key-value pairs. This enables straightforward analysis of what your application is doing, in a way that is easy for a computer to parse, analyze, and index. This makes it simple to quickly filter and search through logs.

Encore’s logging is integrated with the built-in [Distributed Tracing](/docs/observability/tracing) functionality, and all logs are automatically included in the active trace. This dramatically simplifies debugging of your application.

## Usage
First, import `encore.dev/rlog` in your package. Then simply call one of the package methods `Info`, `Error`, or `Debug`. For example:

```go
rlog.Info("log message",
	"user_id", 12345,
	"is_subscriber", true)
rlog.Error("something went terribly wrong!",
	"err", err)
```

The first parameter is the log message. After that follows zero or more key-value pairs for structured logging for context.

If you’re logging many log messages with the same key-value pairs each time it can be a bit cumbersome. To help with that, use `rlog.With()` to group them into a context object, which then copies the key-value pairs into each log event:

```go
ctx := rlog.With("user_id", 12345)
ctx.Info("user logged in", "is_subscriber", true) // includes user_id=12345
```

For more information, see the [API Documentation](https://pkg.go.dev/encore.dev/rlog).

## Live-streaming logs

Encore also makes it simple to live-stream logs directly to your terminal, from any environment, by running:

```
$ encore logs --env=prod
```