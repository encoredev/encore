---
title: Migrate away from Encore
subtitle: If you love someone, set them free.
---

Picking which technologies to use for your project can feel like a big decision. It's tricky, because you never really know where things might end up, and what your future needs will look like.

Why is it that we need to make the big decisions at the start, when we have the least information?
We've been thinking about this a lot when designing Encore!

### Opinionated, not restrictive
The Encore framework is opinionated in places to enable using static analysis to create an application model for what you're building. This is core to how Encore delivers all its powerful features, like automatically instrumenting distributed tracing, and provisioning and managing infrastructure.

The framework is designed by the core team, with long-term experience building highly scalable distributed systems at Spotify and Google, with lots of invaluable input coming from the community.

However, most interesting software projects end up having some novel requirements that are highly specific to the problem they're addressing. To accommodate for this, Encore is designed to let you go outside of the framework, and makes it easy to drop down in abstraction level when you need to; for instance using [raw endpoints](/docs/how-to/webhooks).

### Open Source, so you can do it yourself
Rest assured that the key parts of Encore are Open Source. Including the [parser](https://github.com/encoredev/encore/tree/main/parser), [compiler](https://github.com/encoredev/encore/tree/main/compiler), and [runtime](https://github.com/encoredev/encore/tree/main/runtime). They can all be used however you want. So if you run into something unforeseen down the line, you have free access to the tools you might need.

### Plain Go code, no rewrites
If you really do want to fully migrate away, it's relatively easy to do. Because when you build an Encore application, the vast majority of code is just plain Go. So in reality, the amount of code specific to Encore is very small and there's very little to rewrite.

With Encore, much of the value comes from letting you <i>avoid</i> doing the normally required foundational work. Which means the work required to migrate away is exactly the same as you'd have needed to do without Encore in the first place.

### Your data in your own cloud account, from day one
Because you can deploy to your own cloud account from the start, there's never any data to migrate if you stop using Encore. This makes migrating away very low risk.

### Plays well with others
Encore is all about building distributed systems, and it's straightforward to use your Encore application in combination with other backends, and [databases](/docs/how-to/connect-existing-db), that aren't built with Encore. So if you come across a use case where Encore for some reason doesn't fit, you won't need to tear everything up and start from scratch. You can just build that specific part without Encore.

### Tell us what you need
We're engineers ourselves and we understand the importance of not being tied to a specific technology choice.
It's our belief that adopting Encore is a low-risk decision, given it needs no initial investment in foundational work, it's been designed to avoid lock-in, and you use your own cloud account. Our ambition is simply to add a lot of value to your every-day development process, from day one.

We're working every single day on making it even easier to start, <i>and stop</i>, using Encore.
If you have specific concerns or questions, we'd love to hear from you!

Please reach out on [Slack](https://encore.dev/slack) or [send an email](mailto:hello@encore.dev) with your thoughts.

## Ejecting
If you've decided to migrate away from Encore, Encore has built-in support for ejecting your application as a way of
removing the connection to the Encore Platform. Ejecting your app produces a standalone Docker image that can be
deployed any where you'd like, and can help facilitating the migration away according to the process above.
See `encore eject docker --help` for more information.

### Configuring your ejected docker image
To run your app as an ejected image it needs to be configured. This configuration is normally handled by the Encore Platform,
but needs to be manually managed when ejecting. There are two environment variables that need to be set: `ENCORE_APP_SECRETS`
and `ENCORE_RUNTIME_CONFIG`.

`ENCORE_APP_SECRETS` provides the values of all of your application secrets. The value should be a comma-separated list
of key-value pairs, where the key is the secret name and the value is the secret value in base64 encoded form
(using Go's [RawURLEncoding](https://pkg.go.dev/encoding/base64#pkg-variables) scheme). For example, if you had two secrets
`Foo` and `Bar` with the values `Hello` and `World` respectively, the value of `ENCORE_APP_SECRETS` should be
`Foo=SGVsbG8,Bar=V29ybGQ`.

`ENCORE_RUNTIME_CONFIG` provides the runtime configuration Encore applications need. As the precise configuration changes
over time, please refer to the [current runtime config definition](https://github.com/encoredev/encore/blob/main/runtime/appruntime/config/config.go). The app, environment, and deployment related information powers the [encore.Meta](https://pkg.go.dev/encore.dev#AppMetadata) API
and can be set to arbitrary values. The SQL database and SQL server information is used to configure how Encore connects to SQL databases,
and should be configured according to your own infrastructure setup. `AuthKeys` and `TraceEndpoint` must both be left unspecified as they
determine how the application communicates with the Encore Platform, and leaving them empty disables that functionality.
