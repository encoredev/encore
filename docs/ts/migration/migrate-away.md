---
title: Migrate away from Encore
subtitle: If you love someone, set them free.
lang: ts
---

_We realize most people read this page before even trying Encore, so we start with a perspective on how you might reason about adopting Encore. Read on to see what tools are available for migrating away._

Picking technologies for your project is an important decision. It's tricky because you don't know what the requirements are going to look like in the future. This uncertainty makes many teams opt for maximum flexibility, often without acknowledging this has a significant negative effect on productivity.

When designing Encore, we've leaned on standardization to provide a well-integrated and highly productive development workflow. The design is based on the core team's experience building scalable distributed systems at Spotify and Google, complemented with loads of invaluable input from the developer community. 

In practise Encore is opinionated only in certain areas which are critical for enabling the static analysis used to create Encore's application model. This is fundamental to how Encore can provide its powerful features, like automatically instrumenting distributed tracing, and provisioning and managing cloud infrastructure.

## Accommodating for your unique requirements

Many software projects end up having a few novel requirements, which are highly specific to the problem domain. To accommodate for this, Encore is designed to let you go outside of the standardized Backend Framework when you need to, for example:
- You can drop down in abstraction level in the API framework using [raw endpoints](/docs/ts/primitives/defining-apis#raw-endpoints)
- You can use tools like the [Terraform provider](/docs/platform/integrations/terraform) to integrate infrastructure that is not managed by Encore

## Mitigating risk through Open Source and efficiency

We believe that adopting Encore is a low-risk decision for several reasons:

- There's no upfront investment needed to get the benefits
- Encore apps are normal programs where less than 1% of the code is Encore-specific
- All infrastructure and data is in your own cloud
- It's simple to integrate with cloud services and systems not natively supported by Encore
- Everything you need to develop your application is Open Source, including the [parser](https://github.com/encoredev/encore/tree/main/v2/parser), [compiler](https://github.com/encoredev/encore/tree/main/v2/compiler), [runtime](https://github.com/encoredev/encore/tree/main/runtimes)
- Everything you need to self-host your application is [Open Source and documented](/docs/ts/self-host/build)

## What to expect when migrating away

If you want to migrate away, we want to ensure this is as smooth as possible! Here are some of the ways Encore is designed to keep your app portable, with minimized lock-in, and the tools provided to aid in migrating away.

### Code changes

Building with Encore doesn't require writing your entire application in an Encore-specific way. Encore applications are normal programs where only 1% of the code is specific to Encore's Open Source Backend Framework.

This means that the changes required to stop using the Backend Framework is almost exactly the same work you would have needed to do if you hadn't used Encore in the first place, e.g. writing infrastructure boilerplate. There is no added migration cost.

### Deployment

If you are self-hosting your application, then you're already done.

If you are using Encore Cloud to manage deployments and want to migrate to your own solution, you can use the [self-hosting instructions](/docs/ts/self-host/build) and Open Source CLI tooling. The `encore build docker` command produces a Docker image, containing the compiled application, using exactly the same code path as Encore's CI system to ensure compatibility.

Learn more in the [self-hosting docs](/docs/ts/self-host/build).

### Tell us what you need

We're engineers ourselves and we understand the importance of not being constrained by a single technology.

We're working every single day on making it even easier to start, <i>and stop</i>, using Encore.
If you have specific concerns, questions, or requirements, we'd love to hear from you!

Please reach out on [Discord](https://encore.dev/discord) or [send an email](mailto:hello@encore.dev) with your thoughts.
