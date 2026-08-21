---
seotitle: "Coming from an IaC tool to Encore"
seodesc: How Terraform, Pulumi, OpenTofu and CDK concepts map onto Encore, what happens to infrastructure you have already provisioned, and how to run both alongside each other.
title: Coming from an IaC tool
subtitle: What changes, and what doesn't, when infrastructure moves into your application code
lang: platform
---

Infrastructure-as-code tools describe cloud resources in a codebase of their own. You write the resource, its properties and its dependencies, and the tool reconciles that description against the cloud provider. Encore starts from the other end: your application declares the resources it needs, and the properties that describe how to run them are settings on the environment rather than lines in a file.

That difference produces the same handful of questions no matter which tool you're coming from.

**What replaces my configuration?** A resource becomes an object you construct in the service that uses it. The properties that were in the configuration are either absent from the code, because they are environment settings, or checked when you build. [Primitives](/docs/ts/primitives) covers each resource type.

**Where did the knobs go?** Instance sizes, engines, compute platform and networking are [per-environment settings](/docs/platform/infrastructure/configuration), with defaults you override. Running without the platform, an [infra config file](/docs/ts/self-host/configure-infra) binds each logical resource to infrastructure you provisioned yourself.

**What happens to what I already run?** Existing resources can be imported rather than recreated, and the [Terraform Provider](/docs/platform/integrations/terraform) reads Encore-provisioned resources back into your configuration, so the two can coexist indefinitely.

**What do I give up?** Resources outside the six primitives, and any resource whose shape depends on runtime data, stay outside the model. The per-tool guides are explicit about this.

## Guides

- **[Coming from Terraform](/docs/platform/migration/from-terraform)** — concept mapping, coexistence paths, and the trade-offs, in full.

Guides for Pulumi, OpenTofu and AWS CDK are in progress. Until they land, the Terraform guide covers everything that isn't specific to a particular tool's syntax, since the questions are about the model rather than the language.

## The part that is genuinely different

An IaC tool can describe any resource its providers support. Encore covers six primitives and derives everything else from how your code uses them, which is what makes least-privilege IAM, generated clients and the service catalog possible without you writing them. That is a real trade: less reach, more derived from a single source. Whether it's the right trade depends on how much of your infrastructure falls inside those six, which is the first thing worth checking.

For how the model is built and what it contains, see the [application model](/docs/ts/concepts/application-model).
