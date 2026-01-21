---
seotitle: Using Encore with AI Tools
seodesc: Learn how to set up Encore with AI-powered development tools like Cursor and Claude Code to supercharge your backend development workflow.
title: AI Tools Integration
subtitle: Supercharge your development with AI-powered coding assistants
lang: go
---

Encore gives AI coding assistants superpowers. With Encore-specific rules and MCP integration, AI understands your architecture, generates type-safe code that follows your patterns, and can provision infrastructure, whether [self-hosted](/docs/go/self-host/build) or via [Encore Cloud](https://encore.cloud) in AWS/GCP with automatic guardrails.

## What AI Enables

Encore's structured APIs and infrastructure primitives give AI agents a reliable framework. AI can provision databases, pub/sub topics, and other infrastructure with automatic guardrails, generate type-safe code that follows your existing patterns, and understand your architecture through MCP integration.

<video autoPlay playsInline loop controls muted className="w-full h-full">
  <source src="https://encore.cloud/assets/docs/claude-skills.mp4" type="video/mp4" />
</video>

## Enabling AI for Your Project

There are two ways to set up AI support:

- [Method 1: Using the CLI](#method-1-using-the-cli) (recommended)
- [Method 2: Using Encore Skills](#method-2-using-encore-skills)

### Method 1: Using the CLI

**New projects:** When you run `encore app create`, you'll be prompted to select an AI tool. Encore generates the appropriate configuration files for your chosen tool.

<img src="/assets/docs/initllm.png" />

**Existing projects:** Run `encore llm-rules init` to add AI support:

```bash
encore llm-rules init
```

This prompts you to select a tool and generates the appropriate configuration file (`.cursorrules`, `CLAUDE.md`, etc.).

Both commands also set up MCP server configuration for tools that support it (Cursor, Claude Code). If you want to set up MCP manually, see [MCP Server](#mcp-server) below.

Supported tools: Cursor, Claude Code, VS Code, AGENTS.md, and Zed.

### Method 2: Using Encore Skills

Use the [Encore skills package](https://github.com/encoredev/skills) which works with Cursor, Claude Code, GitHub Copilot, and 10+ other AI agents:

```bash
npx add-skill encoredev/skills
```

You can also install specific skills or target specific agents:

```bash
# List available skills
npx add-skill encoredev/skills --list

# Install to specific agents
npx add-skill encoredev/skills -a cursor -a claude-code
```

## MCP Server

Encore's [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction) server gives AI agents deep introspection into your application: querying databases, calling APIs, inspecting services, and analyzing traces.

### Start the Server

From your Encore app directory:

```bash
encore mcp start
```

This displays connection information. Keep it running while using your AI tools.

### Connect Cursor

**Quick setup:** Use this button (update `your-app-id` to your actual app ID):

<a href="https://cursor.com/en/install-mcp?name=encore-mcp&config=eyJjb21tYW5kIjoiZW5jb3JlIG1jcCBydW4gLS1hcHA9eW91ci1hcHAtaWQifQ%3D%3D"><img src="https://cursor.com/deeplink/mcp-install-dark.svg" alt="Add encore-mcp MCP server to Cursor" height="32" class="noshadow" /></a>

**Manual setup:** Create `.cursor/mcp.json`:

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

Find your app ID with `encore app info` or in the [Encore dashboard](https://app.encore.dev).

### Connect Claude Code

From your Encore app directory:

```bash
claude mcp add --transport stdio encore-mcp -- encore mcp run --app=your-app-id
```

Verify with `claude mcp list`. You should see `encore-mcp` in the list.

## What AI Can Do

With Encore skills and MCP connected, AI can:

- **Provision infrastructure** - databases, pub/sub, secrets, whether [self-hosted](/docs/go/self-host/build) or via [Encore Cloud](https://encore.cloud) in AWS/GCP
- **Generate type-safe APIs** - code that follows your patterns and passes validation
- **Understand architecture** - query databases, inspect services, analyze traces via MCP
- **Test instantly** - run `encore run` to test with real infrastructure, not mocks

Example prompts:

- "Add an endpoint that publishes to a pub/sub topic, call it and verify in traces"
- "Query the users database and show accounts created in the last week"
- "Create a new service with CRUD endpoints connected to PostgreSQL"

## Learn More

- [MCP Server Documentation](/docs/go/cli/mcp) - Complete MCP reference
- [Encore Skills Repository](https://github.com/encoredev/skills) - Available skills and installation
- [Quick Start Guide](/docs/go/quick-start) - Build your first Encore app
