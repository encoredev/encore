---
seotitle: Encore CLI Configuration Options
seodesc: Configuration options to customize the behavior of the Encore CLI.
title: Configuration Reference
subtitle: Configuration options to customize the behavior of the Encore CLI.
lang: ts
---


The Encore CLI has a number of configuration options to customize its behavior.

Configuration options can be set both for individual Encore applications, as well as
globally for the local user.

Configuration options can be set using `encore config <key> <value>`,
and options can similarly be read using `encore config <key>`.

When running `encore config` within an Encore application, it automatically
sets and gets configuration for that application.

To set or get global configuration, use the `--global` flag.

## Configuration files

The configuration is stored in one ore more TOML files on the filesystem.

The configuration is read from the following files, in order:

### Global configuration
* `$XDG_CONFIG_HOME/encore/config`
* `$HOME/.config/encore/config`
* `$HOME/.encoreconfig`

### Application-specific configuration
* `$APP_ROOT/.encore/config`

Where `$APP_ROOT` is the directory containing the `encore.app` file.

The files are read and merged, in the order defined above, with latter files taking precedence over earlier files.

## Configuration options

#### run.browser
Type: string<br/>
Default: auto<br/>
Must be one of: always, never, or auto

Whether to open the Local Development Dashboard in the browser on `encore run`.
If set to "auto", the browser will be opened if the dashboard is not already open.

