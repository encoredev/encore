---
title: Installation
subtitle: Install the Encore CLI to get started with local development
---

If you are new to Encore, we recommend following the [quick start guide](/docs/quick-start).

## Install the Encore CLI
To develop locally with Encore, you first need to install the Encore CLI.
This is what provisions your local development environment, and runs your local development dashboard complete with logs, tracing, and API documentation.


<InstallInstructions />

<Callout type="info">

To locally run Encore apps with databases, you also need to have [Docker](https://www.docker.com) installed and running.

</Callout>

### Build from source
If you prefer to build from source, [follow these instructions](https://github.com/encoredev/encore/blob/main/CONTRIBUTING.md).


## Update to the latest version
Check which version of Encore you have installed by running `encore version` in your terminal.
It should print something like:
```bash
encore version v1.0.0
```

If you think you're on an older version of Encore, you can easily update to the latest version by running
`encore version update` from your terminal.
