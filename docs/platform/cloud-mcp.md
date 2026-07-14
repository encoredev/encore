---
seotitle: Encore Cloud MCP Server
seodesc: Connect AI agents to your deployed environments on Encore Cloud using the Model Context Protocol (MCP) server.
title: Cloud MCP Server
subtitle: Give AI agents access to your deployed environments on Encore Cloud
lang: platform
---

The Encore Cloud MCP Server gives AI agents access to your deployed environments on Encore Cloud, including production traces, deployment status, environment configuration, and infrastructure metadata.

It implements the [Model Context Protocol](https://modelcontextprotocol.io/introduction), an open standard for connecting large language models to external context sources.

For AI access to your local development environment, see the [Local MCP Server](/docs/ts/cli/mcp).

## Endpoint

Connect your AI tool to the Cloud MCP Server at:

```
https://api.encore.dev/mcp
```

## Authentication

The Cloud MCP Server supports two authentication methods:

- **OAuth** — interactive flow that opens a browser to grant access. Best for local development.
- **API Key** — Bearer token sent as an `Authorization` header. Best for CI environments or shared configuration.

## Connect Claude Code

### With OAuth

Add the Cloud MCP Server to Claude Code:

```bash
claude mcp add --transport http encore-cloud https://api.encore.dev/mcp
```

The first time you use the server, Claude Code will open a browser window to complete the OAuth flow and grant access to your Encore Cloud account.

Verify with `claude mcp list`. You should see `encore-cloud` in the list.

### With an API Key

If you'd rather not use OAuth (for example, in CI environments or when sharing configuration), you can authenticate with a Bearer API token instead.

**1. Generate an API key**

In the [Encore Cloud dashboard](https://app.encore.dev), navigate to **Settings → API Keys** for your app and generate a new API key. Copy the token — you won't be able to see it again.

**2. Configure Claude Code with the API key**

Add the Cloud MCP Server with an `Authorization: Bearer` header:

```bash
claude mcp add --transport http encore-cloud https://api.encore.dev/mcp \
  --header "Authorization: Bearer <your-api-key>"
```

When authenticating with an API key, Claude Code skips the OAuth flow and uses the bearer token for every request. Treat the token like a password and avoid committing it to source control.
