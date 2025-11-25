---
seotitle: Environment Variables Reference
seodesc: Learn how to configure Encore's development environment using environment variables.
title: Environment Variables
subtitle: Configure your development environment
lang: ts
---

Encore works out of the box without configuration, but provides several environment variables for advanced use cases such as debugging, testing, or adapting Encore to specific workflow requirements.

## Daemon & Development Dashboard

These variables control how the Encore daemon operates and where it exposes its services.

### ENCORE_DAEMON_LOG_PATH

Controls the location of the Encore daemon log file.

**Default:** `<user_cache_dir>/encore/daemon.log`

**Example:**

```bash
export ENCORE_DAEMON_LOG_PATH=/var/log/encore/daemon.log
```

### ENCORE_DEVDASH_LISTEN_ADDR

Overrides the listen address for the local development dashboard.

**Default:** Automatically assigned by the daemon

**Format:** Network address (e.g., `localhost:9400`)

**Example:**

```bash
export ENCORE_DEVDASH_LISTEN_ADDR=localhost:8080
encore run
```

### ENCORE_MCPSSE_LISTEN_ADDR

Overrides the listen address for the MCP SSE (Model Context Protocol Server-Sent Events) endpoint.

**Default:** Automatically assigned by the daemon

**Format:** Network address

**Example:**

```bash
export ENCORE_MCPSSE_LISTEN_ADDR=localhost:9401
```

### ENCORE_OBJECTSTORAGE_LISTEN_ADDR

Overrides the listen address for the object storage service endpoint.

**Default:** Automatically assigned by the daemon

**Format:** Network address

**Example:**

```bash
export ENCORE_OBJECTSTORAGE_LISTEN_ADDR=localhost:9402
```

## Logging Configuration

These variables control the logging behavior for TypeScript applications.

### ENCORE_RUNTIME_LOG

Sets the log level for Encore's internal runtime operations (written in Rust).

**Default:** `debug` (automatically set to `error` during `encore run`)

**Valid values:** `trace`, `debug`, `info`, `warn`, `error`

**Example:**

```bash
# See detailed runtime logs
export ENCORE_RUNTIME_LOG=trace
encore run
```

<Callout type="info">

If `RUST_LOG` is set, it takes precedence over `ENCORE_RUNTIME_LOG`. The runtime log controls logging for internal Encore modules.

</Callout>

### ENCORE_LOG

Sets the log level for your application code.

**Default:** `Trace` (log everything)

**Valid values:** `Off`, `Error`, `Warn`, `Info`, `Debug`, `Trace`

**Example:**

```typescript
import log from "encore.dev/log";

log.info("This message respects ENCORE_LOG level");
```

```bash
# Only show errors and warnings
export ENCORE_LOG=Warn
encore run
```

### ENCORE_NOLOG

Disables all logging when set to any non-empty value.

**Default:** Not set

**Example:**

```bash
# Disable all logs
export ENCORE_NOLOG=1
encore run
```

## Advanced Development

These variables are primarily useful for advanced development scenarios, such as contributing to Encore itself or using custom builds.

### ENCORE_RUNTIMES_PATH

Specifies the path to the Encore runtimes directory.

**Default:** Auto-detected relative to the Encore installation (`<install_root>/runtimes`)

**Example:**

```bash
export ENCORE_RUNTIMES_PATH=/path/to/custom/runtimes
```

### ENCORE_RUNTIME_LIB

Specifies the path to the native Node.js runtime library used by TypeScript applications.

**Default:** `<runtimes_path>/js/encore-runtime.node`

**Example:**

```bash
export ENCORE_RUNTIME_LIB=/path/to/custom/encore-runtime.node
```

### ENCORE_TSPARSER_PATH

Specifies the path to the TypeScript parser binary.

**Default:** Auto-detected from `encore` binary location or system `PATH`

**Example:**

```bash
export ENCORE_TSPARSER_PATH=/path/to/custom/tsparser-encore
```

<Callout type="info">

For most users, these paths are automatically detected and don't need to be set. They are primarily useful when contributing to Encore or testing custom builds.

</Callout>

## Debugging

### ENCORE_API_INCLUDE_INTERNAL_MESSAGE

Controls whether internal error messages are included in API error responses.

**Default:** automatically set to `1` during local development with `encore run`

**Format:** Any non-empty, non-"0" value is considered `true`

**Example:**

```bash
# Manually enable for debugging
export ENCORE_API_INCLUDE_INTERNAL_MESSAGE=1
```

### RUST_LOG

Controls Rust-level logging for the Encore runtime. This provides more granular control than `ENCORE_RUNTIME_LOG`.

**Default:** Not set

**Format:** Standard Rust `env_logger` format (see [env_logger documentation](https://docs.rs/env_logger))

**Example:**

```bash
# Enable info logs for all modules in the runtime
export RUST_LOG=info
encore run
```

<Callout type="info">

`RUST_LOG` takes precedence over `ENCORE_RUNTIME_LOG`. Use `RUST_LOG` for fine-grained control over specific runtime modules.

</Callout>
