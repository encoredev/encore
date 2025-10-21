---
seotitle: Using Encore with AI Tools
seodesc: Learn how to set up Encore with AI-powered development tools like Cursor and Claude Code to supercharge your backend development workflow.
title: AI Tools Integration
subtitle: Supercharge your development with AI-powered coding assistants
lang: ts
---

Encore works seamlessly with AI-powered development tools like Cursor and Claude Code, giving AI assistants deep context about your application's structure, APIs, databases, and runtime behavior. This enables more accurate code suggestions and powerful development workflows.

## Quick Start

To get the most out of AI tools with Encore, you'll want to:

1. **Add LLM instructions** to help AI tools understand Encore's framework
2. **Set up the MCP server** to give AI assistants deep runtime context about your app

## LLM Instructions

LLM instructions help AI tools like Cursor and GitHub Copilot understand how to use Encore's framework and APIs correctly.

Download the [ts_llm_instructions.txt](https://github.com/encoredev/encore/blob/main/ts_llm_instructions.txt) file and add it to your Encore app.

**Setup for different tools:**

- **Cursor**: Rename the file to `.cursorrules` and place it in your app root
- **Claude Code**: Rename the file to `CLAUDE.MD` and place it in your app root
- **GitHub Copilot**: Paste the content into `.github/copilot-instructions.md`
- **Other tools**: Place the file in your app root

This helps the AI understand Encore-specific patterns like service definitions, API endpoints, database usage, and Pub/Sub topics.

## MCP Server Setup

Encore's [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction) server provides AI tools with deep introspection capabilities, allowing them to:

- Query your databases
- Call API endpoints
- Inspect service structures and middleware
- Analyze request traces
- Understand authentication handlers

Think of MCP as a "USB-C port for AI applications"—a standardized interface that connects your app's data and functionality to any AI tool that supports the protocol.

### Starting the MCP Server

From your Encore app directory, run:

```bash
encore mcp start
```

This will display connection information:

```
MCP Service is running!

MCP SSE URL:        http://localhost:9900/sse?app=your-app-id
MCP stdio Command:  encore mcp run --app=your-app-id
```

Keep this running while you use your AI tools.

### Integrating with Cursor

[Cursor](https://cursor.com) is an AI-powered IDE that works great with Encore's MCP server.

The fastest way to add Encore's MCP server to Cursor is via this button (make sure to update `your-app-id` to your actual Encore app ID):

<a href="https://cursor.com/en/install-mcp?name=encore-mcp&config=eyJjb21tYW5kIjoiZW5jb3JlIG1jcCBydW4gLS1hcHA9eW91ci1hcHAtaWQifQ%3D%3D"><img src="https://cursor.com/deeplink/mcp-install-dark.svg" alt="Add encore-mcp MCP server to Cursor" height="32" class="noshadow" /></a>

**Manual setup:**

Create the file `.cursor/mcp.json` in your app directory:

```json
{
  "mcpServers": {
    "encore-mcp": {
      "command": "encore",
      "args": ["mcp", "run", "--app=your-app-id"]
    }
  }
}
```

Replace `your-app-id` with your actual Encore app ID (you can find this by running `encore app info` or checking the Encore dashboard).

Learn more in [Cursor's MCP documentation](https://docs.cursor.com/context/model-context-protocol).

**What you can do:**

With the MCP server connected, you can ask Cursor's AI agent to perform advanced tasks like:

- "Add an endpoint that publishes to a pub/sub topic, call it and verify that the publish is in the traces"
- "Query the users database and show me all accounts created in the last week"
- "Create a new service with CRUD endpoints and connect it to a PostgreSQL database"

### Integrating with Claude Code

[Claude Code](https://docs.claude.com/en/docs/claude-code/mcp) is Anthropic's AI coding assistant that integrates directly into your terminal and IDE.

From your Encore app directory, run:

```bash
claude mcp add --transport stdio encore-mcp -- encore mcp run --app=your-app-id
```

Replace `your-app-id` with your actual Encore app ID (find it by running `encore app info` or checking the [Encore dashboard](https://app.encore.dev)).

**Verify the connection:**

List your configured MCP servers:

```bash
claude mcp list
```

You should see `encore-mcp` in the list of active servers.

**Manual configuration (alternative):**

Create or edit `.mcp.json` in your project directory:

```json
{
  "mcpServers": {
    "encore-mcp": {
      "type": "stdio",
      "command": "encore",
      "args": ["mcp", "run", "--app=your-app-id"],
      "env": {}
    }
  }
}
```

Learn more about [MCP configuration in Claude Code](https://docs.claude.com/en/docs/claude-code/mcp).

**What you can do:**

With the MCP server connected, Claude Code can perform tasks like:

- "Add an endpoint that publishes to a pub/sub topic, call it and verify that the publish is in the traces"
- "Query the users database and show me all accounts created in the last week"
- "Show me the schema for the orders table and suggest optimizations"
- "Analyze the recent traces for the /api/checkout endpoint and identify any performance issues"

### What the MCP Server Provides

The MCP server exposes powerful tools that give AI assistants comprehensive visibility into your application:

**Database Tools:**

- Get database schemas, tables, and relationships
- Execute SQL queries against your databases

**API Tools:**

- Call any API endpoint in your application
- Retrieve information about all services and endpoints
- Inspect middleware and authentication handlers

**Trace Tools:**

- View request traces with timing and metadata
- Analyze detailed span information for debugging

This deep integration allows AI tools to provide more accurate suggestions, understand your application's runtime behavior, and help you build features faster.

## Learn More

- [MCP Server Documentation](/docs/ts/cli/mcp) - Complete reference for Encore's MCP implementation
- [Encore Installation Guide](/docs/ts/install) - Install the Encore CLI
- [Quick Start Guide](/docs/ts/quick-start) - Build your first Encore app
