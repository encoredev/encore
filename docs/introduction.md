---
seotitle: Introduction to Encore – the backend development engine
seodesc: Learn how Encore works and how it helps backend developers build cloud based backend applications with a flow state developer experience.
title: What is Encore?
subtitle: Escape complexity and put the fun back into backend development
---

Many developers see programming as a creative expression, but the advent of the cloud has ruined this for backend developers. Cloud services help us build highly scalable systems, but have a major problem with developer experience.

These days, most of our time is spent hand-crafting boilerplate and orchestrating cloud services. This mind-numbingly repetitive work doesn't add anything unique to the applications we build and takes away the creative feeling of programming. This is what Encore is designed to solve.

## 1-minute introduction video

It's a long page, hit play if you can't be bothered reading.

<iframe width="360" height="202" src="https://www.youtube.com/embed/nDWMTw4LnyU?controls=0" title="Encore 1 minute introduction" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Common use cases

Encore is designed to give teams a productive and less complex experience when solving most backend use cases. There are many developers using Encore to build things like:

-   CRUD backends and REST APIs.
-   Microservices backends for advanced web and mobile apps.
-   Highly performant APIs providing advanced business logic to 3rd parties.
-   Services powering new features, in applications with existing backend systems.
-   And much more...

## Encore is a development engine for cloud backend applications

Taking inspiration from how game engines have helped game developers unlock creative productivity, Encore gives you the same sort of special-purpose tooling for backend development. It does this by integrating the development process:

-   Write business logic and use cloud infrastructure directly in code, with Encore's Open Source Go framework.
-   Run your local development environment and access development tools with the Open Source Encore CLI.
-   Use the Encore platform to automatically provision preview, development, and cloud environments in your own cloud account.

<img src="/assets/docs/encore-way.png" title="The Encore Way" className="noshadow"/>

## A developer experience designed to help you stay in the flow

### 1. Remove complexity with the Encore Framework

Building a modern backend application is normally very time-consuming. Often several services need to be integrated, complex configuration is required to connect all the pieces together, and there are many manual steps.

With Encore get a monolith-like developer experience when building cloud backends and distributed systems:

-   Write regular Go functions and [immediately get APIs](/docs/develop/services-and-apis#defining-apis).
-   Build microservices by [creating Go packages](/docs/develop/services-and-apis#defining-a-service).
-   Cloud infrastructure like [databases](/docs/develop/databases), [queues](/docs/develop/pubsub), and [scheduled tasks](/docs/develop/cron-jobs), are idiomatic concepts that you declare in your application code.
-   Use out-of-the-box solutions for common use cases like [managing secrets](docs/develop/secrets), [authentication](/docs/develop/auth), and observability.

### 2. Tools for easy collaboration and faster feedback loops

Encore comes with several tools to accelerate feedback loops and help teams collaborate:

-   [Development dashboard](/docs/observability/dev-dash) with API explorer, [distributed tracing](/docs/observability/tracing), and real-time [interactive architecture diagrams](/docs/develop/encore-flow).
-   Automatically generated [API documentation](/docs/develop/api-docs) for all your APIs.
-   [Integrate with GitHub](/docs/how-to/github) and get automatically provisioned preview environments for each pull request.
-   [Generate type-safe API clients](/docs/develop/client-generation) to simplify integrating with systems and frontends.

### 3. One workflow across all environments

With Encore you don't need to spend time on setting up build and deployment pipelines, provisioning Kubernetes clusters and databases, or other busy work. The Encore Platform can set it up for you:

-   Start your local environment with a single command: `encore run`
-   Built-in CI/CD, Run `git push encore` to build, test, provision necessary infrastructure, and deploy.
-   Automatic provisioning of all your infrastructure for local, preview, and cloud environments.
-   Built-in hosting on Encore Cloud for development and hobby use. (See [usage limits](/docs/about/usage))
-   [Connect your own cloud account](/docs/deploy/own-cloud) with AWS/Azure/GCP to have Encore deploy there.

### 4. Understand your system and debug faster

Your entire Encore application is automatically instrumented with [Distributed Tracing](/docs/observability/tracing). This dramatically simplifies finding the source of issues. Is a service slow to respond? What data was being passed in to cause a particular bug to surface? Tracing makes it easy to find the answer.

## Power through standardization

The reason we normally have to write boilerplate, and manually provision cloud services, is that the tools we use have no idea what we're trying to do. So it's up to the developer to do all the work.

Encore is designed to understand what you're building, in order to deliver powerful capabilities like [Encore Flow](/docs/observability/encore-flow) and automatic infrastructure provisioning. To facilitate this understanding Encore is deliberately opinionated about certain things, such as having a standardized way of expressing APIs, defining services, and declaring infrastructure.

We believe that Encore's opinionated approach gives developers the freedom to focus on creatively solving new problems, instead of having to re-solve the same commonplace problems over and over again.

### Congratulations – you don't have to decide!

Developers make dozens of decisions when creating a backend application. Deciding how to structure the codebase, defining API schemas, picking underlying infrastructure, etc. The decisions often come down to personal preferences, not technical rationale. This creates a huge problem in the form of fragmentation! When every stack looks different, all tools have to be general purpose.

When you adopt Encore, many of these stylistic decisions are already made for you. Encore's framework ensures your application follows modern best practices. And when you run your application, Encore's Open Source parser and compiler checks that you're sticking to the framework. This means you're free to focus your energy on what really matters: writing your application's business logic.

### Built ground up for Go, and only Go

Encore is deeply integrated with the [Go](https://golang.org/) programming language. This is not to say Encore is only for Go developers! Most backend developers will get incredible value from using Encore, and learning Go should not stop anyone from trying Encore.

Really, why is picking a programming language seen as the most important decision in a new project? When you set out to build a new backend, there are often very few rational arguments for why one language is better than another. The only real difference is personal taste.

## Meet the Encore Application Model

Encore uses static analysis to understand your application. This is a fancy term for parsing and analyzing the code you write and creating a graph of how your application works. This graph closely represents your own mental model about your system: boxes and arrows, representing systems and services that communicate with other systems, pass data and connect to infrastructure. We call it the Encore Application Model.

Using the model Encore is able to provide tools to solve problems that normally would be up to the developer to do manually, from creating architecture diagrams and API documentation, to provisioning cloud infrastructure.

We're continuously expanding on Encore's capabilities and are building a new generation of developer tools that are enabled by Encore's understanding of your application.

The framework, parser, and compiler that enable this are all [Open Source](https://github.com/encoredev/encore).

<img src="/assets/docs/flow-diagram.png" title="Encore Application Model" className="mx-auto md:max-w-lg"/>

## One workflow from prototype to production

Most frameworks and tools designed for rapid development are great for prototyping, but become a liability as your application grows. Encore is different! Building on our team's long experience from places like Google and Spotify, Encore is designed from the ground up to be suitable for large-scale software engineering.

When you build with the Encore framework, you express cloud infrastructure as logical statements directly in your application code. This means you can change the underlying infrastructure according to your scaling requirements, without needing to change application code.

This means you get the benefits of incredibly rapid prototyping and development using cheaply provisioned cloud infrastructure, while your application gracefully handles increased requirements and scale as you move to production and beyond.

## Where to go from here

You made it to the end of the page – we hope you are as excited as we are about making backend development fun again!

Next, we recommend following the [Quick Start Guide](/docs/quick-start). It gives you a taste of the Encore workflow and takes only around 5-10 minutes to complete, depending on your familiarity with Go.

After completing the guide, why not browse through the docs in your own areas of interest, or join [Slack](https://encore.dev/slack) to ask any questions you might have?

Finally, we recommend you follow and star the project on [GitHub](https://github.com/encoredev/encore) to stay up to date.
