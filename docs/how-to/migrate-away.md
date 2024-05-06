---
title: Migrate away from Encore
subtitle: If you love someone, set them free.
---

_We realize most people read this page before even trying Encore, so we start with a perspective on how you might reason about adopting Encore. Read on to see what tools are available for migrating away._

Picking technologies for your project is an important decision. It's tricky because you don't know what the requirements are going to look like in the future. This uncertainty makes many teams opt for maximum flexibility, often without acknowledging this has a major negative impact on productivity.

When designing Encore, we've leaned on standardization to provide a well-integrated and incredibly productive development workflow. The design is based on the core team's experience building highly scalable distributed systems at Spotify and Google, complemented with loads of invaluable input from the developer community. 

In practise this means Encore is opinionated in certain areas, enabling static analysis used to create Encore's application model. This is core to how Encore can provide its powerful features, like automatically instrumenting distributed tracing, and provisioning and managing cloud infrastructure.

## But I'm a snowflake!

Many software projects end up having some novel requirements, highly specific to the problem they're addressing. To accommodate for this Encore is designed to let you go outside of the mold when you need to, by letting you drop down in abstraction level.

We believe that adopting Encore is a low-risk decision for several reasons:

- There's no upfront work to get the benefits
- Encore apps are normal programs where less than 5% of the code is Encore-specific
- All infrastructure, and data, lives in your own account on AWS/GCP
- It's simple to integrate with "unsupported" cloud services and other systems
- Key parts are Open Source, including the [parser](https://github.com/encoredev/encore/tree/main/v2/parser), [compiler](https://github.com/encoredev/encore/tree/main/v2/compiler), and [runtime](https://github.com/encoredev/encore/tree/main/runtimes)

## What to expect when migrating away

If you want to migrate away, we want to ensure this is as smooth as possible! Here are some of the ways Encore is designed to keep your app portable, with minimized lock-in, and the tools provided to aid in migrating away.

### Code changes

Building with Encore doesn't require writing your application in an Encore-specific way. Encore applications are normal programs where less than 5% of the code is specific to Encore.

This means that the changes required to migrate away will be almost exactly the same work you would have needed to do if you hadn't used Encore in the first place, e.g. writing infrastructure boilerplate. There's no added migration cost.

In practise, the code specific to Encore is limited to the use of Encore's Backend SDK. So if you wish to stop using Encore, you need to rework these interfaces to function in a traditional way. This normally means adding boilerplate that isn't necessary when using Encore.

### Ejecting your app as a Docker image

If you've decided to migrate away from Encore, Encore has built-in support for ejecting your application as a way of
removing the connection to Encore. Ejecting your app produces a standalone Docker image that can be
deployed anywhere you'd like. See `encore eject docker --help` for more information.

#### Configuring your ejected docker image

To run your app as an ejected image it needs to be configured. This configuration is normally handled by Encore,
but needs to be manually managed when ejecting. Two environment variables need to be set: `ENCORE_APP_SECRETS`
and `ENCORE_RUNTIME_CONFIG`.

`ENCORE_APP_SECRETS` provides the values of all of your application secrets. The value should be a comma-separated list
of key-value pairs, where the key is the secret name and the value is the secret value in base64 encoded form
(using Go's [RawURLEncoding](https://pkg.go.dev/encoding/base64#pkg-variables) scheme). For example, if you had two secrets
`Foo` and `Bar` with the values `Hello` and `World` respectively, the value of `ENCORE_APP_SECRETS` should be
`Foo=SGVsbG8,Bar=V29ybGQ`.

`ENCORE_RUNTIME_CONFIG` provides the runtime configuration Encore applications need. As the precise configuration changes
over time, please refer to the [current runtime config definition](https://github.com/encoredev/encore/blob/main/runtimes/go/appruntime/exported/config/config.go). The app, environment, and deployment related information powers the [encore.Meta](https://pkg.go.dev/encore.dev#AppMetadata) API
and can be set to arbitrary values. The SQL database and SQL server information is used to configure how Encore connects to SQL databases,
and should be configured according to your infrastructure setup. `AuthKeys` and `TraceEndpoint` must both be left unspecified as they determine how the application communicates with Encore, and leaving them empty disables that functionality.

### Tell us what you need

We're engineers ourselves and we understand the importance of not being constrained by a single technology.

We're working every single day on making it even easier to start, <i>and stop</i>, using Encore.
If you have specific concerns, questions, or requirements, we'd love to hear from you!

Please reach out on [Discord](https://encore.dev/discord) or [send an email](mailto:hello@encore.dev) with your thoughts.
