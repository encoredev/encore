---
seotitle: Submit a Template to Encore's Templates repo
seodesc: Learn how to contribute to Encore's Templates repository and get features in the Encore Templates marketplace.
title: Submit a Template
subtitle: Your contributions help other developers build
---

[Templates](/templates) help and inspire developers to build applications using Encore together with other tools.

You are welcome to contribute your own Templates!

There are two primary types of contributions that are especially useful:
- **Starters:** Runnable Encore applications that others  can use as is, or take inspiration from.
- **Pre-mades:** Re-usable code samples to solve common development patterns, or for integrating Encore applications with third party tools and services.

## Submit a contribution

Contribute a template by submitting a Pull Request to the [Open Source Examples Repo](https://github.com/encoredev/examples): `https://github.com/encoredev/examples`

**Step by step instructions**
1. Fork the repo.
2. Create a new folder inside the root directory of the repo, this is where you will place your template. — Use a short folder name as your template will be installable via the CLI, like so: `encore app create APP-NAME --example=<TEMPLATE_FOLDER_NAME>`
3. Include a `README.md` with instructions for how to use the template. We recommend following [this format](https://github.com/encoredev/examples/blob/8c7e33243f6bfb1b2654839e996e9a924dcd309e/uptime/README.md).

Once your Pull Request has been approved, it may be featured on the [Templates page](/templates) on the Encore website.

## Contribute from your own repo

If you don't want to contribute code to the examples repo, but still want to be featured on the [Templates page](/templates), please contact us at [hello@encore.dev](mailto:hello@encore.dev).

## Dynamic Encore AppID

In most cases you should avoid hardcoding an `AppID` in your template's source code. Instead, use the notation `{{ENCORE_APP_ID}}`.

When a developer creates an app using the template, `{{ENCORE_APP_ID}}` will be dymically replaced with their new and unique `AppID`, meaning they will not need to make any manual code adjustments.