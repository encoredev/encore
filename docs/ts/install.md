---
seotitle: Install Encore to start building
seodesc: See how you can install Encore on all platforms, and get started building your next backend application in minutes.
title: Installation
subtitle: Install the Encore CLI to get started with local development
lang: ts
---

If you are new to Encore, we recommend following the [quick start guide](/docs/ts/quick-start).

## Install the Encore CLI
To develop locally with Encore, you first need to install the Encore CLI.
This is what provisions your local development environment, and runs your Local Development Dashboard complete with logs, tracing, and API documentation.


<InstallInstructions />

### Prerequisites

- [Node.js](https://nodejs.org/en/download/) is required to run Encore.ts apps.
- [Docker](https://www.docker.com) is required for Encore to set up local databases.

### Optional: Add AI/LLM instructions

To help AI coding assistants (Cursor, Claude Code, GitHub Copilot, etc.) understand how to use Encore, run this from your app directory:

```bash
encore llm-rules init
```

This prompts you to select your tool and generates the appropriate config (e.g. `.cursorrules`, `CLAUDE.md`) and MCP setup where supported. For full details and other options, see [AI Tools Integration](/docs/ts/ai-integration).

### Build from source
If you prefer to build from source, [follow these instructions](https://github.com/encoredev/encore/blob/main/CONTRIBUTING.md).


## Update to the latest version
Check which version of Encore you have installed by running `encore version` in your terminal.
It should print something like:
```shell
encore version v1.28.0
```

If you think you're on an older version of Encore, you can easily update to the latest version by running
`encore version update` from your terminal.
