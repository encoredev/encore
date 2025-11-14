<p align="center" dir="auto">
<a href="https://encore.dev"><img src="https://user-images.githubusercontent.com/78424526/214602214-52e0483a-b5fc-4d4c-b03e-0b7b23e012df.svg" width="160px" alt="encore icon"></img></a><br/><br/><br/>
<b>Open Source Framework for creating type-safe distributed systems with declarative infrastructure</b><br/>
</p>

- **Backend Frameworks:** [Encore.ts](https://encore.dev) and [Encore.go](https://encore.dev/go) simplify creating microservices and type-safe APIs, and provide an AI-ready declarative approach to define infrastructure in code.
- **Local Development:** Encore's CLI automatically manages local infrastructure and provides a development dashboard with tracing, service catalog, and architecture diagrams.
- **Infrastructure Integration:** Simplified integration with cloud infrastructure using the open source CLI ([learn more](https://encore.dev/docs/ts/self-host/build)), or using the optional [Encore Cloud](https://encore.cloud) platform to automate DevOps and infrastructure provisioning in your own cloud on AWS and GCP.

**⭐ Star this repository** to help spread the word.

**Install Encore:**
- **macOS:** `brew install encoredev/tap/encore`
- **Linux:** `curl -L https://encore.dev/install.sh | bash`
- **Windows:** `iwr https://encore.dev/install.ps1 | iex`

**Create your first app:**
- **TypeScript:** `encore app create --example=ts/hello-world`
- **Go:** `encore app create --example=hello-world`

**Add Encore LLM instructions to your app:**
- **Encore.ts:** [ts_llm_instructions.txt](./ts_llm_instructions.txt)
- **Encore.go:** [go_llm_instructions.txt](./go_llm_instructions.txt)
- **How to use:**
  - Cursor: Rename the file to `.cursorrules`.
  - GitHub Copilot: Paste content in `.github/copilot-instructions.md`.
  - For other tools, place the file in your app root.

### How it works

Encore's open source backend frameworks [Encore.ts](https://encore.dev/docs/ts) and [Encore.go](https://encore.dev/docs/primitives/overview) enable you to define resources like services, databases, cron jobs, and Pub/Sub, as type-safe objects in your application code.

With the frameworks you only define **infrastructure semantics** — _the things that matter to your application's behavior_ — not configuration for _specific_ cloud services. Encore parses your application and builds a graph of both its logical architecture and its infrastructure requirements, it then automatically generates boilerplate and helps orchestrate the relevant infrastructure for each environment. This means the same application code can be used to run locally, test in preview environments, and deploy to cloud environments on e.g. AWS and GCP.

This often removes the need for separate infrastructure configuration like Terraform, increases standardization in both your codebase and infrastructure, and makes your application highly portable across cloud providers.

Encore is fully open source, there is **no proprietary code running in your application**.

### Example: Hello World

Defining microservices and API endpoints is incredibly simple—with less than 10 lines of code, you can create a production-ready, deployable service.

**Hello World in Encore.ts**

```typescript
import { api } from "encore.dev/api";

export const get = api(
  { expose: true, method: "GET", path: "/hello/:name" },
  async ({ name }: { name: string }): Promise<Response> => {
    const msg = `Hello ${name}!`;
    return { message: msg };
  }
);

interface Response {
  message: string;
}
```

**Hello World in Encore.go**

```go
package hello

//encore:api public path=/hello/:name
func World(ctx context.Context, name string) (*Response, error) {
	msg := fmt.Sprintf("Hello, %s!", name)
	return &Response{Message: msg}, nil
}

type Response struct {
	Message string
}
```

### Example: Using Pub/Sub

If you want a Pub/Sub Topic, you declare it directly in your application code and Encore will integrate the infrastructure and generate the boilerplate code necessary.
Encore supports the following Pub/Sub infrastructure:
- **NSQ** for local environments (automatically provisioned by Encore's CLI)
- **GCP Pub/Sub** for environments on GCP
- **SNS/SQS** for environments on AWS

**Using Pub/Sub in Encore.ts**

```typescript
import { Topic } "encore.dev/pubsub"

export interface SignupEvent {
    userID: string;
}

export const signups = new Topic<SignupEvent>("signups", {
    deliveryGuarantee: "at-least-once",
});
```

**Using Pub/Sub in Encore.go**

```go
import "encore.dev/pubsub"

type User struct { /* fields... */ }

var Signup = pubsub.NewTopic[*User]("signup", pubsub.TopicConfig{
  DeliveryGuarantee: pubsub.AtLeastOnce,
})

// Publish messages by calling a method
Signup.Publish(ctx, &User{...})
```

--

For more info:
**See example apps:** [Example Apps Repo](https://github.com/encoredev/examples/)

**See products being build with Encore:** [Showcase](https://encore.cloud/showcase)

**Have questions?** Join the friendly developer community on [Discord](https://encore.dev/discord).

**Talk to a human:** [Book a 1:1 demo](https://encore.dev/book) with one of our founders.

## Intro video
[Watch the intro video](https://youtu.be/vvqTGfoXVsw) for a quick introduction to Encore concepts & code examples.

<a href="https://youtu.be/vvqTGfoXVsw" target="_blank"><img width="589" alt="Encore Intro Video" src="https://github.com/encoredev/encore/assets/78424526/89737146-be48-429f-a83f-41bc8da37980"></a>

## Introduction to Encore

Building scalable applications with cloud services is powerful but often frustrating. Developers face complex setups and repetitive tasks that slow them down.

Encore solves this with an all-in-one backend development toolkit, streamlining everything from local testing to cloud integration and DevOps.

<p align="center">
<img width="589" alt="Encore Overview" src="https://github.com/encoredev/encore/assets/78424526/ecb65a20-866c-449c-bf0e-e6d99c78430b">
</p>

### Learn more in the docs

See how to use the backend frameworks in the docs:

- **Services:** [Go](https://encore.dev/docs/go/primitives/services) / [TypeScript](https://encore.dev/docs/ts/primitives/services)
- **APIs:** [Go](https://encore.dev/docs/go/primitives/defining-apis) / [TypeScript](https://encore.dev/docs/ts/primitives/defining-apis)
- **Databases:** [Go](https://encore.dev/docs/go/primitives/databases) / [TypeScript](https://encore.dev/docs/ts/primitives/databases)
- **Cron Jobs:** [Go](https://encore.dev/docs/go/primitives/cron-jobs) / [TypeScript](https://encore.dev/docs/ts/primitives/cron-jobs)
- **Pub/Sub:** [Go](https://encore.dev/docs/go/primitives/pubsub) / [TypeScript](https://encore.dev/docs/ts/primitives/pubsub)
- **Object Storage:** [Go](https://encore.dev/docs/go/primitives/object-storage) / [TypeScript](https://encore.dev/docs/ts/primitives/object-storage)
- **Caching:** [Go](https://encore.dev/docs/go/primitives/caching) / TypeScript (Coming soon)


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
- **Secrets management:** Built-in [secrets management](https://encore.dev/docs/ts/primitives/secrets) for all environments.
- **Tracing:** The [local development dashboard](https://encore.dev/docs/ts/observability/dev-dash) provides local tracing to help understand application behavior and find bugs.
- **Automatic API docs & clients:** Encore generates [API docs](https://encore.dev/docs/ts/obsevability/service-catalog) and [API clients](https://encore.dev/docs/ts/cli/client-generation) in Go, TypeScript, JavaScript, and OpenAPI specification.

_Here's a video showing the local development dashboard:_

https://github.com/encoredev/encore/assets/78424526/4d066c76-9e6c-4c0e-b4c7-6b2ba6161dc8

### Testing

<p align="center">
<img width="573" alt="testing" src="https://github.com/encoredev/encore/assets/78424526/516a043c-66ac-464e-a4ca-f8ecd5642d54">
</p>

Encore comes with several built-in tools to help with testing:

- **Built-in service/API mocking:** Encore provides built-in support for [mocking API calls](https://encore.dev/docs/go/develop/testing/mocking), and interfaces for automatically generating mock objects for your services.
- **Local test infra:** When running tests locally, Encore automatically provides dedicated [test infrastructure](https://encore.dev/docs/go/develop/testing#test-only-infrastructure) to isolate individual tests.
- **Local test tracing:** The [Local Development Dashboard](https://encore.dev/docs/ts/observability/dev-dash) provides distributed tracing for tests, providing great visibility into what's happening and making it easier to understand why a test failed.
- **Preview Environments:** When using Encore Cloud (optional), it automatically provisions a temporary cloud-based [Preview Environment](https://encore.dev/docs/platform/deploy/preview-environments) for each Pull Request, an effective tool when doing end-to-end testing.

### Optional: Automate your AWS/GCP with Encore Cloud

<p align="center">
<img width="573" alt="DevOps" src="https://github.com/encoredev/encore/assets/78424526/e00d3e92-3301-4f3a-89cc-575c4a520aae">
</p>

Encore Cloud is Encore's managed service offering for teams wanting to focus their engineering effort on their product development, avoiding investing time in platformization and DevOps.

Encore Cloud provides **automatic infrastructure provisioning in your cloud on AWS & GCP**. So instead of writing Terraform, YAML, or clicking in cloud consoles, you [connect your cloud account](https://encore.dev/docs/platform/deploy/own-cloud) and simply deploy your application. Since using Encore's open source backend frameworks means your application code is cloud agnostic and not tied to any specific infrastructure services, Encore Cloud enables you to change your infrastructure depending on your evolving needs, without needing to make code changes or manually update infrastructure config files.

When you deploy, Encore Cloud automatically provisions [infrastructure](https://encore.dev/docs/platform/infrastructure/infra) using battle-tested cloud services on AWS and GCP, such as:
- **Compute:** GCP Cloud Run, AWS Fargate, Kubernetes (GKE and EKS)
- **Databases:** GCP Cloud SQL, AWS RDS
- **Pub/Sub:** GCP Pub/Sub, AWS SQS/SNS
- **Caches:** GCP Memorystore, Amazon ElastiCache
- **Object Storage:** GCS, Amazon S3
- **Secrets:**	GCP Secret Manager, AWS Secrets Manager
- Etc.

Encore Cloud also includes cloud versions of Encore's built-in development tools:

- [Service Catalog & API Docs](https://encore.dev/docs/ts/observability/service-catalog)
- [Architecture Diagrams](https://encore.dev/docs/ts/observability/flow)
- [Tracing](https://encore.dev/docs/ts/observability/tracing)

_Here's a video showing the Encore Cloud dashboard:_

https://github.com/encoredev/encore/assets/78424526/8116b387-d4d4-4e54-8768-3686ba0245f5

## Why use Encore?

- **Faster Development**: Encore streamlines the development process with its backend frameworks, clear abstractions, and built-in local development tools.
- **Scalability & Performance**: Encore simplifies building large-scale microservices applications that can handle growing user bases and demands, without the normal boilerplate and complexity.
- **Control & Standardization**: Built-in tools like automated architecture diagrams, infrastructure tracking and approval workflows, make it easy for teams and leaders to get an overview of the entire application.
- **Security & Compliance**: Encore Cloud helps ensure your application is secure and compliant by enforcing security standards like least privilege IAM, and provisioning infrastructure according to best practices for each cloud provider.
- **Reduced Costs**: Encore Cloud's automatic infrastructure management minimizes wasteful cloud expenses and reduces DevOps workload, allowing you to work more efficiently.

## Common use cases

Encore is designed to give teams a productive and less complex experience when solving most backend use cases. Many teams use Encore to build things like:

-   High-performance B2B Platforms
-   Fintech & Consumer apps
-   Global E-commerce marketplaces
-   Microservices backends for SaaS applications and mobile apps
-   And much more...

## Getting started

- **1. Install Encore:**
  - **macOS:** `brew install encoredev/tap/encore`
  - **Linux:** `curl -L https://encore.dev/install.sh | bash`
  - **Windows:** `iwr https://encore.dev/install.ps1 | iex`
- **2. Create your first app:**
  - **TypeScript:** `encore app create --example=ts/introduction`
  - **Go:** `encore app create --example=hello-world`
- **3. Star the project** on [GitHub](https://github.com/encoredev/encore) to stay up-to-date
- **4. Explore the [Documentation](https://encore.dev/docs)** to learn more about Encore's features
- **5. [Join Discord](https://encore.dev/discord)** to ask questions and meet other Encore developers

## Open Source

Everything needed to develop and deploy Encore applications is Open Source, including the backend frameworks, parser, compiler, runtime, and CLI.
This includes all code needed for local development and everything that runs in your application when it is deployed.

The Open Source CLI also provides a mechanism to generate a Docker images for your application, so you easily self-host your application. [Learn more in the docs](https://encore.dev/docs/ts/self-host/build).

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

- <a href="https://youtu.be/vvqTGfoXVsw" alt="Intro video: Encore concepts & features" target="_blank">Intro: Encore concepts & features</a>
- <a href="https://youtu.be/wiLDz-JUuqY" alt="Demo video: Getting started with Encore.ts" target="_blank">Demo video: Getting started with Encore.ts</a>
- <a href="https://youtu.be/IwplIbwJtD0" alt="Demo video: Building and deploying a simple service" target="_blank">Demo: Building and deploying a simple Go service</a>
- <a href="https://youtu.be/ipj1HdG4dWA" alt="Demo video: Building an event-driven system" target="_blank">Demo: Building an event-driven system in Go</a>

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

### Why does Encore provide infrastructure integrations through Encore Cloud?

We've found that to meaningfully improve the developer experience, you have to operate across the full stack. Unless you understand how an application is deployed, there are a large number of things in the development process that you can't simplify. That's why so many other developer tools have such a limited impact. With Encore's more integrated approach, we're able to unlock a radically better experience for developers.

### What if I want to migrate away from Encore?

Encore is designed to let you go outside of the framework when you want to, and easily drop down in abstraction level when you need to, so you never run into any dead-ends.

Should you want to migrate away, it's straightforward and does not require a big rewrite. 99% of your code is regular Go or TypeScript.

Encore provides tools for [self-hosting](https://encore.dev/docs/ts/self-host/build) your application, by using the Open Source CLI to produce a standalone Docker image that can be deployed anywhere you'd like.

## Contributing to Encore and building from source

See [CONTRIBUTING.md](CONTRIBUTING.md).
