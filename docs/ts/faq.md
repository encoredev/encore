---
seotitle: Frequently Asked Questions
seodesc: See quick answers to common questions about Encore
title: FAQ
subtitle: Quick answers to common questions
lang: ts
---

## About the project

**Is Encore Open Source?**

Yes, check out the project on [GitHub](https://github.com/encoredev/encore).

**Is there a community?**

Yes, you're welcome to join the developer community on [Discord](https://encore.dev/discord).

## Can I use X with Encore?

**Can I use Python with Encore?**

Encore currently supports Go and TypeScript. Python support in on the [roadmap](https://encore.dev/roadmap) and will be available around Q1 2025.

**Can mix TypeScript and Go in one application?**

Support for mixing languages in coming. Currently, if you want to use both TypeScript and Go, you need to create a separate application per language and integrate using APIs.

**Can I use Azure / Digital Ocean?**

Encore Cloud currently supports automating deployments to AWS and GCP. Azure support in on the [roadmap](https://encore.dev/roadmap) and will be available in 2025.

If you want to use other cloud providers like Azure or Digital Ocean, you can follow the [self-hosting instructions](/docs/how-to/self-host).

**Can I use MongoDB / MySQL with Encore?**

Encore currently has built-in support for PostgreSQL. To use another type of database, like MongoDB and MySQL, you will need to set it up and integrate as you normally would when not using Encore.

**Can I use AWS lambda with Encore?**

Not right now. Encore currently supports AWS Fargate and EKS (along with CloudRun and GKE on Google Cloud Platform).

**Can I use Bun / Deno with Encore.ts?**

Right now Encore.ts officially supports Node and has experimental support for Bun. Deno support is on the way. Note that Encore.ts already provides performance improvements thanks to its Rust-based runtime. [Learn more](https://encore.dev/blog/event-loops).

To enable the Bun experiment, add `"experiments": ["bun-runtime"]` to your `encore.app` file.

## IDE Integrations

**Is there an Encore plugin for Goland / IntelliJ?**

Yes, Encore's official Goland plugin is available in the [JetBrains marketplace](https://plugins.jetbrains.com/plugin/20010-encore).

**Is there an Encore plugin for VS Code?**

Not yet, it's coming soon.

## Troubleshooting

**symlink creation error on Windows**

Encore currently relies on symbolic links, which may be disabled by default. A common fix for this issue is to enable "developer mode" in the Windows settings (Settings > System > For developers > Developer mode).

**`node` errors**

You might need to restart the Encore daemon, e.g. if your PATH has changed since installing nvm. Restart the daemon by running `encore daemon`.
