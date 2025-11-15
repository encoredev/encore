<p align="center" dir="auto">
<a href="https://encore.dev"><img src="https://user-images.githubusercontent.com/78424526/214602214-52e0483a-b5fc-4d4c-b03e-0b7b23e012df.svg" width="160px" alt="encore icon"></img></a><br/><br/><br/>
<b>Open Source Framework for creating type-safe distributed systems with declarative infrastructure</b><br/>
</p>

**Encore consists of three parts:**

1. **Backend Frameworks:** [Encore.ts](https://encore.dev) and [Encore.go](https://encore.dev/go) let you define APIs, services, and infrastructure (databases, Pub/Sub, caching, buckets, cron jobs) as type-safe objects in your code. Write your application once, then deploy it anywhere without code changes by [exporting a Docker image](https://encore.dev/docs/ts/self-host/build) and supplying the infra configuration.

2. **Local Dev Tools:** The Encore CLI runs your app locally and automatically provisions the local infrastructure. Local dev tools like tracing, API documentation, service catalog, architecture diagrams, and database explorer are built-in.

3. **Optional DevOps Platform:** [Encore Cloud](https://encore.cloud) parses your application and automatically provisions the required infrastructure in your own AWS/GCP account. Other tools include Preview Environments for each PR, service catalog, distributed tracing, and metrics.

**⭐ Star this repository** to help spread the word.

**Install Encore:**

- **macOS:** `brew install encoredev/tap/encore`
- **Linux:** `curl -L https://encore.dev/install.sh | bash`
- **Windows:** `iwr https://encore.dev/install.ps1 | iex`

**Create your first app:**

- **TypeScript:** `encore app create --example=ts/hello-world`
- **Go:** `encore app create --example=hello-world`

**Use with AI coding assistants:**

Add LLM instructions ([Encore.ts](./ts_llm_instructions.txt) / [Encore.go](./go_llm_instructions.txt)) to your project for better AI code suggestions. Optionally use the [MCP server](https://encore.dev/docs/ts/ai-integration) to give AI tools runtime context (query databases, call APIs, analyze traces). [Learn more](https://encore.dev/docs/ts/ai-integration)

### How it works

Encore's open source backend frameworks [Encore.ts](https://encore.dev/docs/ts) and [Encore.go](https://encore.dev/docs/primitives/overview) enable you to define resources like services, databases, Pub/Sub, caches, buckets, and cron jobs, as type-safe objects in your application code.

You only define **infrastructure semantics** (_what matters for the behavior of the application_), not configuration for specific cloud services:

```typescript
const db = new SQLDatabase("users", { migrations: "./migrations" });
```

Encore parses your application to understand your infrastructure requirements. Then:

- **Locally:** The Encore CLI sets up local infrastructure (Postgres using Docker, Pub/Sub using NSQ, etc.)
- **AWS/GCP:** Encore Cloud automatically provisions managed services and deploys your application (RDS / Cloud SQL, SQS / Pub/Sub, S3 / Cloud Storage, etc.)
- **Self-hosted:** Export a Docker image and supply your infra config to host anywhere

The same application code works everywhere without changes and makes your application portable across cloud providers.

Encore is fully open source, there is **no proprietary code running in your application**.

#### Need some infrastructure Encore doesn't provide?

Encore never prevents you from using arbitrary infrastructure.

You can use any external resource as you normally would, directly integrating standard SDKs (AWS SDK, GCP client libraries, third-party APIs, etc.) using [Secrets](https://encore.dev/docs/primitives/secrets) to define per-environment settings or using Encore's environment configuration. You then provision that resource yourself as you normally would.

#### Want to integrate with existing infrastructure?

Encore Cloud supports multiple ways of integrating existing infrastructure and systems, including deploying to existing Kubernetes clusters, importing existing databases, and provides a [Terraform provider](https://encore.dev/docs/platform/integrations/terraform) to make it simple to integrate Encore managed infrastructure with your existing infrastructure landscape.

### Example: Hello World

Defining microservices and API endpoints is incredibly simple. With less than 10 lines of code, you can create a production-ready, deployable service.

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

---

For more info:

**See example apps:** [Example Apps Repo](https://github.com/encoredev/examples/)

**See products already being built with Encore:** [Showcase](https://encore.cloud/showcase)

**Hear from teams using Encore:** [Case studies](https://encore.cloud/customers)

**Have questions?** Join the friendly developer community on [Discord](https://encore.dev/discord)

**Talk to a human:** [Book a 1:1 demo](https://encore.dev/book) with one of our founders

## Documentation

**Framework Primitives:**

- **Services:** [Go](https://encore.dev/docs/go/primitives/services) / [TypeScript](https://encore.dev/docs/ts/primitives/services)
- **APIs:** [Go](https://encore.dev/docs/go/primitives/defining-apis) / [TypeScript](https://encore.dev/docs/ts/primitives/defining-apis)
- **Databases:** [Go](https://encore.dev/docs/go/primitives/databases) / [TypeScript](https://encore.dev/docs/ts/primitives/databases)
- **Cron Jobs:** [Go](https://encore.dev/docs/go/primitives/cron-jobs) / [TypeScript](https://encore.dev/docs/ts/primitives/cron-jobs)
- **Pub/Sub:** [Go](https://encore.dev/docs/go/primitives/pubsub) / [TypeScript](https://encore.dev/docs/ts/primitives/pubsub)
- **Object Storage:** [Go](https://encore.dev/docs/go/primitives/object-storage) / [TypeScript](https://encore.dev/docs/ts/primitives/object-storage)
- **Caching:** [Go](https://encore.dev/docs/go/primitives/caching) / TypeScript (Coming soon)

**Intro video:** [Watch on YouTube](https://youtu.be/vvqTGfoXVsw) for a quick introduction to Encore concepts & code examples.

<a href="https://youtu.be/vvqTGfoXVsw" target="_blank"><img width="589" alt="Encore Intro Video" src="https://github.com/encoredev/encore/assets/78424526/89737146-be48-429f-a83f-41bc8da37980"></a>

## Local Development & Testing

### Run locally with `encore run`

The CLI automatically provisions local infrastructure (Postgres, Pub/Sub, object storage, secrets) and provides a development dashboard with:

- **Distributed tracing** to understand application behavior and debug issues
- **API documentation** and client generation (Go, TypeScript, JavaScript, OpenAPI)
- **Service catalog** with architecture diagrams
- **Database explorer** to inspect and query your databases

Works fully offline. No Docker Compose configuration needed.

<p align="center">
<img width="578" alt="Local Development" src="https://github.com/encoredev/encore/assets/78424526/6bf682bb-f57e-4a02-9c92-ff83f7fb59d2">
</p>

### Type-safety across services

When building microservices with Encore, you get:

- **Cross-service type-safety:** Auto-complete in your IDE when making API calls between services
- **Type-aware infrastructure:** Infrastructure like Pub/Sub topics are type-safe objects, enabling end-to-end type-safety in event-driven applications

### Testing

- **Built-in mocking:** [Mock API calls](https://encore.dev/docs/go/develop/testing/mocking) and auto-generate mock objects for services
- **Test infrastructure:** Dedicated [test infrastructure](https://encore.dev/docs/go/develop/testing#test-only-infrastructure) provisioned automatically to isolate tests
- **Test tracing:** Distributed tracing for tests in the development dashboard
- **Preview Environments:** Encore Cloud provisions temporary [Preview Environments](https://encore.dev/docs/platform/deploy/preview-environments) for each PR

<p align="center">
<img width="573" alt="testing" src="https://github.com/encoredev/encore/assets/78424526/516a043c-66ac-464e-a4ca-f8ecd5642d54">
</p>

## Encore Cloud (Optional DevOps Platform)

Automates infrastructure provisioning and DevOps in your own AWS/GCP account. [Connect your account](https://encore.dev/docs/platform/deploy/own-cloud) and deploy, with no Terraform or YAML needed.

**Provisions infrastructure:**

- Compute (Cloud Run, Fargate, Kubernetes), databases (Cloud SQL, RDS), Pub/Sub (GCP Pub/Sub, SQS/SNS), caching (Memorystore, ElastiCache), storage (GCS, S3), etc.
- Conservative defaults: scalable instances, least-privilege IAM, secure networking

**Deployment flexibility (no code changes needed):**

- Deploy to AWS or GCP (or switch between them)
- Run on serverless (Cloud Run, Fargate) or Kubernetes (GKE/EKS)
- Colocate all services in one process, or deploy each service independently
- Different infrastructure sizing and scaling per environment

**Control & tools:**

- Override defaults via Encore Cloud dashboard or your AWS/GCP console
- Approval workflows and audit logs for infrastructure changes
- [Preview Environments](https://encore.dev/docs/platform/deploy/preview-environments) for each PR
- [Distributed tracing](https://encore.dev/docs/ts/observability/tracing), metrics, service catalog, architecture diagrams

<p align="center">
<img width="573" alt="DevOps" src="https://github.com/encoredev/encore/assets/78424526/e00d3e92-3301-4f3a-89cc-575c4a520aae">
</p>

## Why use Encore?

- **Faster Development**: Encore streamlines the development process by providing guardrails, clear abstractions, and removing manual infrastructure tasks from development interations.
- **Scalability & Performance**: Encore simplifies building large-scale microservices applications that can handle growing user bases and demands, without the normal boilerplate and complexity.
- **Control & Standardization**: Built-in tools like automated architecture diagrams, infrastructure tracking and approval workflows, make it easy for teams and leaders to get an overview of the entire application.
- **Security & Compliance**: Encore Cloud helps ensure your application is secure and compliant by enforcing security standards like least privilege IAM, and provisioning infrastructure according to best practices for each cloud provider.
- **Reduced Costs**: Encore Cloud's automatic infrastructure management removes common cloud expenses like overprovisioned test environments, and reduces DevOps workload.

## How does Encore compare to other tools?

| Tool                               | What it does                                                  | How Encore differs                                                                                                                                                                                           |
| ---------------------------------- | ------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Pulumi / CDK / Terraform / SST** | Infrastructure-as-Code tools for provisioning cloud resources | IaC defines cloud-specific resources (AWS Fargate, GCP Cloud Run). Encore uses semantic infrastructure (services, databases, Pub/Sub), write once, deploy anywhere (AWS/GCP/local), colocated or distributed |
| **Serverless Framework / Chalice** | Frameworks for deploying serverless functions to AWS          | Encore supports any architecture (monolith, microservices, serverless) and any cloud (AWS, GCP, self-hosted)                                                                                                 |
| **NestJS / Express / Fiber**       | Web frameworks for building APIs and services                 | Encore provides the same capabilities plus infrastructure primitives, local dev tooling, and optional cloud deployment                                                                                       |
| **Convex / Supabase / Firebase**   | Managed backend-as-a-service platforms                        | Encore gives you similar productivity but supports microservices and event-driven systems, and deploys to your own cloud account—no vendor lock-in                                                           |
| **Vercel / Netlify / Railway**     | Deployment platforms (primarily frontend/full-stack)          | Encore is backend-specialized with deeper primitives (Pub/Sub, cron, caching) and multi-cloud infrastructure automation                                                                                      |

## Common use cases

Encore is designed to give teams a productive and less complex experience when solving most backend use cases. Many teams use Encore to build things like:

- High-performance B2B Platforms
- Fintech & Consumer apps
- Global E-commerce marketplaces
- Microservices backends for SaaS applications and mobile apps
- And much more...

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

## Join our pioneering developer community

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

## Frequently Asked Questions (FAQ)

### Who's behind Encore?

Encore was founded by long-time backend engineers from Spotify, Google, and Monzo with over 50 years of collective experience. We've lived through the challenges of building complex distributed systems with thousands of services, and scaling to hundreds of millions of users.

Encore grew out of these experiences and is a solution to the frustrations that came with them: unnecessary crippling complexity and constant repetition of undifferentiated work that suffocates the developer's creativity. With Encore, we want to set developers free to achieve their creative potential.

### Who is Encore for?

**Individual developers:** Build cloud applications without managing infrastructure configuration. Go from idea to deployed application in minutes instead of days.

**Startup teams:** Get a production-ready backend on AWS/GCP without hiring DevOps. Focus engineering time on your product instead of reinventing infrastructure patterns and building platform tooling.

**Large organizations:** Standardize backend development across teams. Reduce onboarding time and operational overhead. Spin up new services in minutes with consistent patterns, without needing days of back and forth between development and DevOps teams.

### Does defining infrastructure in code couple my app to infrastructure?

No. Encore keeps your application code cloud-agnostic by letting you refer only to **logical resources** (like "a Postgres database" or "a Pub/Sub topic"). A backend-agnostic interface means your code has no cloud-specific imports or configurations.

Encore's compiler and runtime handle the mapping of logical resources to actual infrastructure, which is configured per environment. Your code stays identical whether the environment uses e.g.:

- **AWS RDS** or **GCP Cloud SQL** for databases
- **SQS/SNS** or **GCP Pub/Sub** for messaging
- **AWS Fargate**, **Cloud Run**, or **Kubernetes** for compute

This **reduces coupling** compared to traditional Infrastructure-as-Code or cloud SDKs, which embed cloud-specific decisions directly in your codebase. With Encore, swapping cloud providers or infrastructure services requires no code changes.

### What kind of support does Encore offer?

Encore is fully open source and maintained by a dedicated team. Support options include:

- **Community Support:** Free support via [GitHub Issues](https://github.com/encoredev/encore/issues) and [Discord](https://encore.dev/discord)
- **Documentation:** Comprehensive guides and API references at [encore.dev/docs](https://encore.dev/docs)
- **Paid Support:** For teams requiring guaranteed response times or dedicated support, [contact us](mailto:hello@encore.dev) about support plans

### What if I want to migrate away from Encore?

Encore is designed to let you go outside of the framework when you want to, and easily drop down in abstraction level when you need to, so you never run into any dead-ends.

Should you want to migrate away, it's straightforward and does not require a big rewrite. 99% of your code is regular Go or TypeScript.

Encore provides tools for [self-hosting](https://encore.dev/docs/ts/self-host/build) your application, by using the Open Source CLI to produce a standalone Docker image that can be deployed anywhere you'd like.

Learn more in the [migration guide](https://encore.dev/docs/ts/migration/migrate-away)

## Roadmap

We're actively expanding Encore's capabilities. Here's what's on the horizon:

**Languages**

- **Python**: Next on the roadmap for broader ecosystem support

**Cloud Providers**

- **Azure**: Planned to complement existing AWS and GCP support

**Infrastructure Primitives**

- Expanding storage, compute, and queue options

See the full [Public Roadmap](https://encore.dev/roadmap) and share your feedback on what you'd like to see next.

## Contributing to Encore and building from source

See [CONTRIBUTING.md](CONTRIBUTING.md).
