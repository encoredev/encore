---
seotitle: Encore Local MCP Server
seodesc: Encore's Local Model Context Protocol (MCP) server provides deep introspection of your application to AI development tools.
title: Local MCP Server
subtitle: The Model Context Protocol (MCP) exposes tools that provide application context to LLMs.
lang: ts
---

Encore provides an MCP server that implements the [Model Context Protocol](https://modelcontextprotocol.io/introduction), an open standard that enables large language models (LLMs) to access contextual information about your application. Think of MCP as a standardized interface—like a "USB-C port for AI applications"—that connects your Encore app's data and functionality to any LLM that supports the protocol.

You can connect to Encore's MCP server from any MCP host (such as Claude Desktop, IDEs, or other AI tools) using either Server-Sent Events (SSE) or stdio transport. To set up this connection, simply run:

```bash
cd my-encore-app
encore mcp start

  MCP Service is running!

  MCP SSE URL:        http://localhost:9900/sse?app=your-app-id
  MCP stdio Command:  encore mcp run --app=your-app-id
```

Copy the appropriate URL or command to your MCP host's configuration, and you're ready to give your AI assistants rich context about your application.

## Example: Integrating with Cursor

[Cursor](https://cursor.com) is one of the most popular AI powered IDE's, and it's simple to use Encore's MCP server together with Cursor. 

In order to add the Encore MCP server to Cursor, the fastest way is via the button below (make sure to update `your-app-id` in the configuration to your actual Encore app ID).

<a href="https://cursor.com/en/install-mcp?name=encore-local&config=eyJjb21tYW5kIjoiZW5jb3JlIG1jcCBydW4gLS1hcHA9eW91ci1hcHAtaWQifQ%3D%3D"><img src="https://cursor.com/deeplink/mcp-install-dark.svg" alt="Add encore-local MCP server to Cursor" height="32" class="noshadow" /></a>

If you prefer to configure it manually, create the file `.cursor/mcp.json` with the following settings:

```json
{
    "mcpServers": {
        "encore-local": {
             "command": "encore",
             "args": ["mcp", "run", "--app=your-app-id"]
        }
    }
}
```

Learn more in [Cursor's MCP docs](https://docs.cursor.com/context/model-context-protocol)

Now when using Cursor's Agent mode, you can ask it to do advanced actions, such as:

"Add an endpoint that publishes to a pub/sub topic, call it and verify that the publish is in the traces"

## Command Reference

#### Start

Starts an SSE-based MCP server and displays connection information.

```shell
$ encore mcp start [--app=<app-id>]
```

#### Run

Establishes an stdio-based MCP session. This command is typically used by MCP hosts to communicate with the server through standard input/output streams.

```shell
$ encore mcp run [--app=<app-id>]
```

## Exposed Tools

Encore's MCP server exposes the following tools that provide AI models with detailed context about your application. These tools enable LLMs to understand your application's structure, retrieve relevant information, and take actions within your system.

#### Database Tools

- **get_databases**: Retrieve metadata about all SQL databases defined in the application, including their schema, tables, and relationships.
- **query_database**: Execute SQL queries against one or more databases in the application.

#### API Tools

- **call_endpoint**: Make HTTP requests to any API endpoint in the application. Supports an optional `retry_until` parameter that polls the endpoint until a predicate matches (`status`, `body_path`, or `body_jq`) or the timeout elapses. On `matched: false`, the envelope includes `retry.last_response` and `retry.body_unchanged` so you can tell "predicate is racing with a slow update" apart from "upstream isn't moving" (typically: missing Pub/Sub subscription, wrong endpoint, or wrong predicate path).
- **get_services**: Retrieve comprehensive information about all services and their endpoints in the application.
- **get_middleware**: Retrieve detailed information about all middleware components in the application.
- **get_auth_handlers**: Retrieve information about all authentication handlers in the application.

#### Trace Tools

- **get_traces**: Retrieve a list of request traces from the application, including their timing, status, and associated metadata.
- **get_trace_spans**: Retrieve detailed information about one or more traces, including all spans, timing information, and associated metadata.

#### Source Code Tools

- **get_metadata**: Retrieve the complete application metadata, including service definitions, database schemas, API endpoints, and other infrastructure components.
- **get_src_files**: Retrieve the contents of one or more source files from the application.

#### PubSub Tools

- **get_pubsub**: Retrieve detailed information about all PubSub topics and their subscriptions in the application.
- **wait_for_subscription_message**: Block until a matching message has been processed by a subscription, then return the outcome (message ID, payload, delivery attempt, handler duration, handler error). Scans recent history (`lookback_ms`, default 5s, capped at 60s) before opening a forward wait window (`timeout_ms`, default 10s). On timeout, the response includes `subscriptions_on_topic` so a typo'd subscription name is self-diagnosable in a single turn.

##### Canonical Pub/Sub verify recipe

When verifying a Pub/Sub flow end-to-end, compose the two synchronous tools — `wait_for_subscription_message` first to confirm the handler ran, then `call_endpoint` with `retry_until` to confirm the read endpoint surfaces the result:

```jsonc
// 1. Trigger the publish.
call_endpoint { "service": "site", "endpoint": "add", "method": "POST", "path": "/site", "payload": "{\"url\":\"https://example.com\"}" }

// 2. Confirm the subscription handler ran (lookback catches it even after the publish).
wait_for_subscription_message { "topic": "site.added", "subscription": "audit-site-added", "lookback_ms": 5000, "timeout_ms": 10000 }

// 3. Confirm the read endpoint surfaces the new state.
call_endpoint {
  "service": "audit", "endpoint": "listEvents", "method": "GET", "path": "/events",
  "retry_until": { "predicate": { "body_jq": ".events | length >= 1" }, "timeout_ms": 5000, "interval_ms": 250 }
}
```

If step 2 times out with `subscriptions_on_topic` listing a name you didn't expect, the subscription argument is wrong. If step 3 returns `retry.matched: false` with `retry.body_unchanged: true`, the upstream isn't producing data — investigate the subscription or the read endpoint rather than retrying with a larger timeout.

#### Storage Tools

- **get_storage_buckets**: Retrieve comprehensive information about all storage buckets in the application.
- **get_objects**: List and retrieve metadata about objects stored in one or more storage buckets.

#### Cache Tools

- **get_cache_keyspaces**: Retrieve comprehensive information about all cache keyspaces in the application.

#### Metrics Tools

- **get_metrics**: Retrieve comprehensive information about all metrics defined in the application.

#### Cron Tools

- **get_cronjobs**: Retrieve detailed information about all scheduled cron jobs in the application.

#### Secret Tools

- **get_secrets**: Retrieve metadata about all secrets used in the application.

#### Documentation Tools

- **search_docs**: Search the Encore documentation using Algolia's search engine.
- **get_docs**: Retrieve the full content of specific documentation pages.

