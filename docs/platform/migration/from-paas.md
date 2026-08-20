---
seotitle: "Coming from a PaaS to Encore"
seodesc: How Supabase, Railway, Heroku, Vercel and Fly.io concepts map onto Encore, and what changes when the infrastructure runs in your own AWS or GCP account.
title: Coming from a PaaS
subtitle: Keeping the parts of a platform you like, in an account you own
lang: platform
---

A managed platform gives you a database, a queue and a deploy pipeline without you configuring any of it, which is why teams reach for one and why they stay. What eventually pushes people off is rarely the developer experience. It is running out of room: a region you can't have, a compliance requirement you can't meet, a cost curve that stops making sense at your size, or a limit you can't raise.

Encore keeps the shape of that experience and changes where the resources live. You still declare a database and use it, without writing provisioning code. The resources are created in your own AWS or GCP account, so the ceiling is the cloud provider's rather than the platform's.

## What carries over, and what doesn't

**Your database carries over.** Postgres is Postgres. Migrations run the same way, and existing data can be moved with the same tools you would use between any two Postgres instances.

**Managed extras usually don't map one to one.** Hosted auth, generated REST layers over your tables, edge functions and realtime subscriptions are platform features rather than infrastructure primitives. Some have a direct equivalent, some become application code, and some you keep using as an external service. The per-platform guides are specific about which is which.

**Deploys stay one command.** You push, and a [preview environment](/docs/platform/deploy/preview-environments) comes up per pull request before production. The [development workflow](/docs/platform/workflow) covers the loop.

**The bill changes shape.** You pay your cloud provider for the resources and Encore for the control plane, rather than one blended platform price. Whether that is cheaper depends entirely on what you run.

## Guides

Per-platform guides for Supabase, Firebase, Railway, Heroku, Vercel and Fly.io are in progress.

In the meantime, [Primitives](/docs/ts/primitives) shows what you declare and what each one becomes in AWS and GCP, and the [Quick Start](/docs/ts/quick-start) is the fastest way to see whether the developer experience holds up against what you have now.

## Before you move

Two things are worth checking first, because they decide whether this is a small migration or a large one.

Take an inventory of the platform features you actually depend on, not the ones you provisioned. Teams are usually using less of a platform than they think, and the gap between those two lists is the real size of the move.

Then check where your data has to live and who is allowed to reach it. Owning the cloud account is the main thing you gain here, and it is worth being deliberate about it rather than discovering the constraint afterwards.
