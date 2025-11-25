---
seotitle: Environment Variables Reference
seodesc: Learn how to configure Encore's development environment using environment.
title: Environment Variables
subtitle: Configure your development environment
lang: go
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

## Advanced Development

These variables are primarily useful for advanced development scenarios, such as contributing to Encore itself or using custom builds.

### ENCORE_RUNTIMES_PATH

Specifies the path to the Encore runtimes directory.

**Default:** Auto-detected relative to the Encore installation (`<install_root>/runtimes`)

**Example:**

```bash
export ENCORE_RUNTIMES_PATH=/path/to/custom/runtimes
```

### ENCORE_GOROOT

Specifies the path to the custom Encore Go runtime.

**Default:** Auto-detected relative to the Encore installation (`<install_root>/encore-go`)

**Example:**

```bash
export ENCORE_GOROOT=/path/to/custom/encore-go
```

<Callout type="info">

For most users, these paths are automatically detected and don't need to be set. They are primarily useful when contributing to Encore or testing custom builds.

</Callout>
