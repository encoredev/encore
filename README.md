<p align="center" dir="auto">
<a href="https://encore.dev"><img src="https://user-images.githubusercontent.com/78424526/214602214-52e0483a-b5fc-4d4c-b03e-0b7b23e012df.svg" width="160px" alt="encore icon"></img></a><br/><br/>
<b>Encore – Backend Development Platform</b><br/><br/>
</p>

Encore provides a purpose-built workflow to help you create **event-driven and distributed systems** — from local development to your cloud on **AWS & GCP**.

It consists of a **Microservice Framework** & **Infrastructure SDK**, a **Local Development Environment** with tools like tracing, and a **Cloud Platform** for automating CI/CD and cloud infrastructure provisioning.

**🏁 Try Encore:** [Quick Start Guide](https://encore.dev/docs/quick-start)

**💻 See example apps:** [Example Apps Repo](https://github.com/encoredev/examples/)

**🚀 Discover products built with Encore:** [Showcase](https://encore.dev/showcase)

**⭐ Star this repository** to help spread the word.

**🍿 Intro video:** [Watch this video](https://youtu.be/LN8mQWho0Jc) for an introduction to Encore concepts & features

**👋 Have questions?** Join the friendly developer community on [Discord](https://encore.dev/discord).

**📞 See if Encore fits your project:** [Book a 1:1 demo](https://encore.dev/book).

## Introduction to Encore

Cloud services enable us to build highly scalable applications, but often lead to a poor developer experience — forcing developers to manage significant complexity during development and do a lot of repetitive manual work.

Encore is purpose-built to solve this problem and provides a complete toolset for backend development — from local development and testing, to cloud infrastructure management and DevOps.

<p align="center">
<img width="589" alt="Encore Overview" src="https://github.com/encoredev/encore/assets/78424526/ecb65a20-866c-449c-bf0e-e6d99c78430b">
</p>

### How it works

Encore's Open Source declarative Infrastructure SDK, available for [Go](https://encore.dev/docs/primitives/overview) and [TypeScript](https://encore.dev/docs/ts) (Beta), lets you define resources like services, databases, cron jobs, and Pub/Sub, as type-safe objects in your application code.

With the SDK you only define **infrastructure semantics** — _the things that matter to your application's behavior_ — not configuration for _specific_ cloud services. Encore parses your application and builds a graph of both its logical architecture and its infrastructure requirements, it then automatically generates boilerplate and orchestrates the relevant infrastructure for each environment. This means your application code can be used to run locally, test in preview environments, and provision and deploy to cloud environments on AWS and GCP.

This completely removes the need for separate infrastructure configuration like Terraform, increases standardization in both your codebase and infrastructure, and makes your application portable across cloud providers by default.

When your application is deployed to your cloud, there are **no runtime dependencies on Encore** and there is **no proprietary code running in your cloud**.

#### Example: Using Pub/Sub

If you want a Pub/Sub Topic, you declare it directly in your application code and Encore will automatically provision the infrastructure and generate the boilerplate code necessary for each environment:
- **NSQ** for local development
- **GCP Pub/Sub** for environments on GCP
- **SNS/SQS** for environments on AWS

Using the Go SDK, it looks like so:

```go
import "encore.dev/pubsub"
 
type User struct { /* fields... */ }
 
var Signup = pubsub.NewTopic[*User]("signup", pubsub.TopicConfig{
  DeliveryGuarantee: pubsub.AtLeastOnce,
})
 
// Publish messages by calling a method
Signup.Publish(ctx, &User{...})
```

Using the TypeScript SDK, it looks like so:

```typescript
import { Topic } "encore.dev/pubsub"

export interface SignupEvent {
    userID: string;
}

export const signups = new Topic<SignupEvent>("signups", {
    deliveryGuarantee: "at-least-once",
});
```

### Learn more in the docs

See how to use the Infrastructure SDK in the docs:

- **Services and APIs:** [Go](https://encore.dev/docs/primitives/services-and-apis) / [TypeScript](https://encore.dev/docs/ts/primitives/services-and-apis)
- **Databases:** [Go](https://encore.dev/docs/primitives/databases) / [TypeScript](https://encore.dev/docs/ts/primitives/databases)
- **Cron Jobs:** [Go](https://encore.dev/docs/primitives/cron-jobs) / [TypeScript](https://encore.dev/docs/ts/primitives/cron-jobs)
- **Pub/Sub:** [Go](https://encore.dev/docs/primitives/pubsub) / [TypeScript](https://encore.dev/docs/ts/primitives/pubsub)
- **Caching:** [Go](https://encore.dev/docs/primitives/caching) / TypeScript (Coming soon)

## Using Encore: An end-to-end workflow from local to cloud

Encore provides purpose-built tooling for each step in the development process, from local development and testing, to cloud DevOps. Here we'll cover the key features for each part of the process.

### Local Development

<p align="center">
<img width="578" alt="Local Development" src="https://github.com/encoredev/encore/assets/78424526/6bf682bb-f57e-4a02-9c92-ff83f7fb59d2">
</p>

When you run your app locally using the [Encore CLI](https://encore.dev/docs/install), Encore parses your code and automatically sets up the necessary local infrastructure on the fly. _No more messing around with Docker Compose!_

You also get built-in tools for an efficient workflow when creating distributed systems and event-driven applications:

- **Local environment matches cloud:** Encore automatically handles the semantics of service communication and interfacing with different types of infrastructure services, so that the local environment is a 1:1 representation of your cloud environment.
- **Cross-service type-safety:** When building microservices applications with Encore, you get type-safety and auto-complete in your IDE when making cross-service API calls.
- **Type-aware infrastructure:** With Encore, infrastructure like Pub/Sub queues are type-aware objects in your program. This enables full end-to-end type-safety when building event-driven applications.
- **Secrets management:** Built-in [secrets management](https://encore.dev/docs/primitives/secrets) for all environments.
- **Tracing:** The [local development dashboard](https://encore.dev/docs/observability/dev-dash) provides local tracing to help understand application behavior and find bugs.
- **Automatic API docs & clients:** Encore generates [API docs](https://encore.dev/docs/develop/api-docs) and [API clients](https://encore.dev/docs/develop/client-generation) in Go, TypeScript, JavaScript, and OpenAPI specification.

_Here's a video showing the local development dashboard:_

https://github.com/encoredev/encore/assets/78424526/4d066c76-9e6c-4c0e-b4c7-6b2ba6161dc8

### Testing

<p align="center">
<img width="573" alt="testing" src="https://github.com/encoredev/encore/assets/78424526/516a043c-66ac-464e-a4ca-f8ecd5642d54">
</p>

Encore comes with several built-in tools to help with testing:

- **Built-in service/API mocking:** Encore provides built-in support for [mocking API calls](https://encore.dev/docs/develop/testing/mocking), and interfaces for automatically generating mock objects for your services.
- **Local test infra:** When running tests locally, Encore automatically provides dedicated [test infrastructure](https://encore.dev/docs/develop/testing#test-only-infrastructure) to isolate individual tests.
- **Local test tracing:** The [local dev dashboard](https://encore.dev/docs/observability/dev-dash) provides distributed tracing for tests, providing great visibility into what's happening and making it easier to understand why a test failed.
- **Preview Environments:** Encore automatically provisions a [Preview Environment](https://encore.dev/docs/deploy/preview-environments) for each Pull Request, an effective tool when doing end-to-end testing.

### DevOps

<p align="center">
<img width="573" alt="DevOps" src="https://github.com/encoredev/encore/assets/78424526/e00d3e92-3301-4f3a-89cc-575c4a520aae">
</p>

With Encore you can focus your engineering effort on your product and avoid investing time in building a developer platform.

A core feature Encore provides is **automatic infrastructure provisioning** in your cloud. Because your application code is the source of truth for the application's infrastructure requirements, instead of writing Terraform, YAML, or clicking in cloud consoles, you [connect your cloud account](https://encore.dev/docs/deploy/own-cloud) and deploy. This approach also lets you swap out your infrastructure over time, without needing to make code changes or manually update infrastructure config files.

When you deploy, Encore automatically provisions [infrastructure](https://encore.dev/docs/deploy/infra) using battle-tested cloud services on AWS and GCP, such as:
- **Compute:** GCP Cloud Run, AWS Fargate, Kubernetes (GKE and EKS)
- **Databases:** GCP Cloud SQL, AWS RDS
- **Pub/Sub:** GCP Pub/Sub, AWS SQS/SNS
- **Caches:** GCP Memorystore,	Amazon ElastiCache
- **Secrets:**	GCP Secret Manager,	AWS Secrets Manager
- Etc.

Encore also provides built-in DevOps tools to help automate >90% of the day-to-day DevOps work:

- **Automatic least-privilege IAM:** Encore parses your application code and sets up least-privilege IAM to match the requirements of the application.
- **Infra tracking & approvals workflow:** Encore keeps track of all the [infrastructure](https://encore.dev/docs/deploy/infra) it provisions and provides an approval workflow as part of the deployment process, so Admins can verify and approve all infra changes.
- **Cloud config 2-way sync:** Encore provides [a simple UI to make configuration changes](https://encore.dev/docs/deploy/infra#configurability), and also supports syncing changes you make in your cloud console in AWS/GCP.
- **Cost analytics:** A simple overview to monitor costs for all infrastructure provisioned by Encore in your cloud.
- **Logging & Metrics:** Encore automatically provides [logging](https://encore.dev/docs/observability/logging), [metrics](https://encore.dev/docs/observability/metrics), and [integrates with 3rd party tools](https://encore.dev/docs/observability/metrics#integrations-with-third-party-observability-services) like Datadog and Grafana.
- **Service Catalog:**  Encore automatically generates a service catalog with complete [API documentation](https://encore.dev/docs/develop/api-docs).
- **Architecture diagrams:** To help with onboarding and collaboration, Encore generates [architecture diagrams](https://encore.dev/docs/observability/encore-flow) for your application, including infrastructure dependencies.
- **Extensible through Encore's Terraform Provider:** Extend your system with any infrastructure services you need, integration is simple because all infrastructure is provisioned in your cloud. Encore also has a [Terraform Provider](https://encore.dev/docs/deploy/terraform) to simplify this process.

_Here's a video showing the Cloud Platform Dashboard:_

https://github.com/encoredev/encore/assets/78424526/8116b387-d4d4-4e54-8768-3686ba0245f5

## Why use Encore?

- **Faster Development**: Encore streamlines the development process with its infrastructure SDK, clear abstractions, and built-in development tools, enabling you to build and deploy applications more quickly.
- **Reduced Costs**: Encore's automatic infrastructure management minimizes wasteful cloud expenses and reduces DevOps workload, allowing you to work more efficiently.
- **Scalability & Performance**: Encore simplifies building large-scale microservices applications that can handle growing user bases and demands, without the normal boilerplate and complexity.
- **Control & Standardization**: Built-in tools like automated architecture diagrams, infrastructure tracking and approval workflows, make it easy for teams and leaders to get an overview of the entire application.
- **Security & Compliance**: Encore helps ensure your application is secure and compliant by enforcing security standards and provisioning infrastructure according to best practices for each cloud provider.

## Common use cases

Encore is designed to give teams a productive and less complex experience when solving most backend use cases. Many teams use Encore to build things like:

-   High-performance B2B Platforms
-   Fintech & Consumer apps
-   Global E-commerce marketplaces
-   Microservices backends for SaaS applications and mobile apps
-   And much more...

## Getting started

- **1.** [Sign up and install the Encore CLI](https://encore.dev/signup)
- **2.** [Follow a tutorial and start building](https://encore.dev/docs/tutorials/)
- **3.** Follow and star the project on [GitHub](https://github.com/encoredev/encore) to stay up-to-date
- **4.** Explore the [Documentation](https://encore.dev/docs) to learn more about Encore's features
- **5.** [Book a 1:1](https://encore.dev/book) or [join Discord](https://encore.dev/discord) to discuss your use case or how to begin adopting Encore

## Open Source

Encore's Infrastructure SDK, parser, compiler, and CLI are all Open Source — this includes all code needed for local development and everything that runs in your cloud.
A free Encore account is needed to use features like distributed tracing, secrets management, and deploying to cloud environments, as this functionality is orchestrated by Encore's Cloud Platform.

The Open Source CLI also provides a mechanism to generate a standalone Docker image for your application, so you can deploy it without using the Cloud Platform. [Learn more in the docs](https://encore.dev/docs/how-to/migrate-away#ejecting-your-app-as-a-docker-image).

## Join the most pioneering developer community

Developers building with Encore are forward-thinkers who want to focus on creative programming and building great software to solve meaningful problems. It's a friendly place, great for exchanging ideas and learning new things! **Join the conversation on [Discord](https://encore.dev/discord).**

We rely on your contributions and feedback to improve Encore for everyone who is using it.
Here's how you can contribute:

- ⭐ **Star and watch this repository to help spread the word and stay up to date.**
- Meet fellow Encore developers and chat on [Discord](https://encore.dev/discord).
- Follow Encore on [Twitter](https://twitter.com/encoredotdev).
- Share feedback or ask questions via [email](mailto:hello@encore.dev).
- Leave feedback on the [Public Roadmap](https://encore.dev/roadmap).
- Send a pull request here on GitHub with your contribution.

## Videos

- <a href="https://youtu.be/LN8mQWho0Jc" alt="Intro video: Encore concepts & features" target="_blank">Intro: Encore concepts & features</a>
- <a href="https://youtu.be/IwplIbwJtD0" alt="Demo video: Building and deploying a simple service" target="_blank">Demo: Building and deploying a simple service</a>
- <a href="https://youtu.be/ipj1HdG4dWA" alt="Demo video: Building an event-driven system" target="_blank">Demo: Building an event-driven system</a>

## Visuals

### Code example (Go)

https://github.com/encoredev/encore/assets/78424526/f511b3fe-751f-4bb8-a1da-6c9e0765ac08

### Local Development Dashboard

https://github.com/encoredev/encore/assets/78424526/4c659fb8-e9ec-4f14-820b-c2b8d35e5359

### Generated Architecture Diagrams & Service Catalog

https://github.com/encoredev/encore/assets/78424526/a880ed2d-e9a6-4add-b5a8-a4b44b97587b

### Auto-Provisioning Infrastructure & Multi-cloud Deployments

https://github.com/encoredev/encore/assets/78424526/8116b387-d4d4-4e54-8768-3686ba0245f5

### Distributed Tracing & Metrics

https://github.com/encoredev/encore/assets/78424526/35189335-e3d7-4046-bab0-1af0f00d2504

## Frequently Asked Questions (FAQ)

### Who's behind Encore?

Encore was founded by long-time backend engineers from Spotify, Google, and Monzo with over 50 years of collective experience. We’ve lived through the challenges of building complex distributed systems with thousands of services, and scaling to hundreds of millions of users.

Encore grew out of these experiences and is a solution to the frustrations that came with them: unnecessary crippling complexity and constant repetition of undifferentiated work that suffocates the developer’s creativity. With Encore, we want to set developers free to achieve their creative potential.

### Who is Encore for?

**For individual developers** building for the cloud, Encore provides a radically improved experience. With Encore you’re able to stay in the flow state and experience the joy and creativity of building.

**For startup teams** who need to build a scalable backend to support the growth of their product, Encore lets them get up and running in the cloud within minutes. It lets them focus on solving the needs of their users, instead of spending most of their time re-solving the everyday challenges of building distributed systems in the cloud.

**For individual teams in large organizations** that want to focus on innovating and building new features, Encore lets them stop spending time on operations and onboarding new team members. Using Encore for new feature development is easy, just spin up a new backend service in a few minutes.

### How is Encore different?

Encore is the only tool that understands what you’re building. Encore uses static analysis to deeply understand the application you’re building. This enables a unique developer experience that helps you stay in the flow as you’re building. For instance, you don't need to bother with configuring and managing infrastructure, setting up environments and keeping them in sync, or writing documentation and drafting architecture diagrams. Encore does all of this automatically out of the box.

Unlike many tools that aim to only make cloud deployment easier, Encore is not a cloud hosting provider. With Encore, you can use your cloud account with AWS and GCP. This means you’re in control of your data and can maintain your trust relationship with your cloud provider. You can also use Encore's development cloud for free, with pretty generous "fair use" limits.

### Why is the Encore SDK integrated with a cloud platform?

We've found that to meaningfully improve the developer experience, you have to operate across the full stack. Unless you understand how an application is deployed, there are a large number of things in the development process that you can't simplify. That's why so many other developer tools have such a limited impact. With Encore's more integrated approach, we're able to unlock a radically better experience for developers.

### What if I want to migrate away from Encore?

Encore has been designed to let you go outside of the SDK when you want to, and easily drop down in abstraction level when you need to, so you never need to run into any dead-ends.

Should you want to migrate away, it's straightforward and does not require a big rewrite. 95% of your code is regular Go or TypeScript and the code specific to Encore is limited to using the Infrastructure SDK.

Encore has built-in support for [ejecting](https://encore.dev/docs/how-to/migrate-away#ejecting) your application as a way of removing the connection to the Encore Platform. Ejecting your app produces a standalone Docker image that can be deployed anywhere you'd like, and can help facilitate the migration away according to the process above.

Migrating away is low risk since Encore deploys to your cloud account from the start, which means there's never any data to migrate.

Open Source also plays a role. Encore's code generation, compiler, and parser are all open source and can be used however you want. So if you run into something unforeseen down the line, you have free access to the tools you might need.

And since Encore is designed for building distributed systems, it's straightforward to use it in combination with other backends that aren't built with Encore. So if you come across a use case where Encore for some reason doesn't fit, you won't need to tear everything up and start from scratch. You can just build that specific part without Encore.

We believe that adopting Encore is a low-risk decision, given it needs no initial investment in foundational work. The ambition is to simply add a lot of value to your everyday development process, from day one.

## Contributing to Encore and building from source

See [CONTRIBUTING.md](CONTRIBUTING.md).
