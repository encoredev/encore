<p align="center" dir="auto">
<a href="https://encore.dev"><img src="https://user-images.githubusercontent.com/78424526/214602214-52e0483a-b5fc-4d4c-b03e-0b7b23e012df.svg" width="160px" alt="encore icon"></img></a><br/><br/><br/>
<b>Open Source Framework for creating type-safe distributed systems with declarative infrastructure</b><br/>
</p>

- **Framework:** The Encore framework, available for TypeScript and Go, lets you define APIs, services, and infrastructure (databases, Pub/Sub, caching, buckets, cron jobs) as type-safe objects in your code. Write your application once, then deploy it anywhere without code changes by [exporting a Docker image](https://encore.dev/docs/ts/self-host/build) and supplying the infra configuration.

- **Local Dev Tools:** The Encore CLI runs your app locally and automatically provisions local infrastructure. Encore's local dev dashboard provides tools for a productive workflow: Tracing, API Explorer, Service Catalog, Architecture Diagrams, and Database Explorer.

- **DevOps Platform (Optional):** [Encore Cloud](https://encore.cloud) parses your application and automatically provisions the required infrastructure in your own AWS/GCP account. Other tools include Preview Environments for each PR, Service Catalog, Distributed Tracing, Metrics, and Cost Analytics.

**⭐ Star this repository** to help spread the word and stay up to date.

### Get started

**Install Encore:**

- **macOS:** `brew install encoredev/tap/encore`
- **Linux:** `curl -L https://encore.dev/install.sh | bash`
- **Windows:** `iwr https://encore.dev/install.ps1 | iex`

**Create your first app:**

- **TypeScript:** `encore app create --example=ts/hello-world`
- **Go:** `encore app create --example=hello-world`

**Use with AI coding assistants:**

Add LLM instructions ([Encore.ts](./ts_llm_instructions.txt) / [Encore.go](./go_llm_instructions.txt)) to your project for better AI code suggestions. Use the [MCP server](https://encore.dev/docs/ts/ai-integration) to give your AI tools runtime context (query databases, call APIs, analyze traces). [Learn more](https://encore.dev/docs/ts/ai-integration)

https://github.com/encoredev/encore/assets/78424526/4d066c76-9e6c-4c0e-b4c7-6b2ba6161dc8

_Encore's local development dashboard_

## How it works

Encore's open source backend frameworks, [Encore.ts](https://encore.dev/docs/ts) and [Encore.go](https://encore.dev/docs/primitives/overview), enable you to define resources like services, databases, Pub/Sub, caches, buckets, and cron jobs, as type-safe objects in your application code.

You only define **infrastructure semantics** (_what matters for the behavior of the application_), not configuration for specific cloud services. Here's how you define a database in Encore.ts:

```typescript
const db = new SQLDatabase("users", { migrations: "./migrations" });
```

Encore parses your application to understand your infrastructure requirements, then sets up infrastructure in different environments:

- **Locally:** The Encore CLI sets up local infrastructure (microservices, Postgres, Pub/Sub, etc.) and provides a development dashboard with distributed tracing, API documentation, service catalog, architecture diagrams, and database explorer. Works offline, no Docker Compose needed.

- **AWS/GCP:** Encore Cloud deploys to your AWS/GCP account without any Terraform or YAML needed. It automatically sets up compute instances (serverless or Kubernetes), databases (RDS/Cloud SQL), Pub/Sub (SQS/GCP Pub/Sub), storage (S3/GCS), caching (ElastiCache/Memorystore), and all other required resources like security groups and IAM policies, according to best practices.

- **Self-hosted:** Use the Encore CLI to export your app as Docker images, then supply your infra config to host anywhere.

<img width="578" alt="Encore overview" src="https://github.com/user-attachments/assets/89fb83cf-2400-4b54-969d-6f7a5911e76c" />

_Encore orchestrates infrastructure from local development and testing, to production in your cloud._

#### Encore makes it simpler to build distributed systems

- **Microservices without boilerplate:** Call APIs in other services like regular functions. Encore handles service discovery, networking, and serialization. Get cross-service type-safety and auto-complete in your IDE.

- **Modular monolith to microservices:** Structure your application using independent services for clarity. Then deploy them colocated in a single process or as distributed microservices, without changing a single line of code. Encore handles service communication whether in-process or over the network.

- **Testing built-in:** Mock API calls, get dedicated test infrastructure, and use distributed tracing for tests.

### Example: Hello World

Defining microservices and API endpoints is very simple. With less than 10 lines of code, you can create a production-ready, deployable service.

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
Encore orchestrates the relevant Pub/Sub infrastructure for different environments:

- **NSQ** for local environments
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

### Need some infrastructure Encore doesn't provide?

Encore never prevents you from using arbitrary infrastructure.

You can use any external resource as you normally would, directly integrating standard SDKs (AWS SDK, GCP client libraries, third-party APIs, etc.). You then provision that resource yourself as you normally would.

### Want to use Encore in an existing system?

You don't need a complete rewrite, Encore supports incremental adoption.

**Service-by-service adoption (recommended):** Build new services with Encore and run them alongside your existing system, integrated via APIs. Then incrementally migrate existing services as needed. Each migrated service immediately gets Encore's full feature set: infrastructure provisioning, tracing, architecture diagrams.

#### Deployment options:

- **Your Kubernetes cluster:** Deploy directly to your existing Kubernetes infrastructure. Run Encore alongside legacy systems in the same environment.
- **Encore-managed infrastructure:** Encore provisions and manages infrastructure in your AWS/GCP account, deployed within your existing VPC and security setup.
- **Terraform provider:** Encore provides a [Terraform provider](https://encore.dev/docs/platform/integrations/terraform) to make it simple to integrate Encore managed infrastructure with your existing infrastructure landscape.

Start with low-risk, frequently-changed services to validate the approach. Learn more in our [migration guide](https://encore.dev/docs/platform/migration/migrate-to-encore).

## Learn more

- **Documentation:** [Encore Docs](https://encore.dev/docs)

- **See example apps:** [Example Apps Repo](https://github.com/encoredev/examples/)

- **See products built with Encore:** [Showcase](https://encore.cloud/showcase)

- **Hear from teams using Encore:** [Case studies](https://encore.cloud/customers)

- **Have questions?** Join the friendly developer community on [Discord](https://encore.dev/discord)

- **Talk to a human:** [Book a 1:1 demo](https://encore.dev/book) with one of our founders

- **Videos:**
  - <a href="https://youtu.be/vvqTGfoXVsw" alt="Intro video: Encore concepts & features" target="_blank">Intro: Encore concepts & features</a>
  - <a href="https://youtu.be/wiLDz-JUuqY" alt="Demo video: Getting started with Encore.ts" target="_blank">Demo video: Getting started with Encore.ts</a>
  - <a href="https://youtu.be/IwplIbwJtD0" alt="Demo video: Building and deploying a simple service" target="_blank">Demo: Building and deploying a simple Go service</a>
  - <a href="https://youtu.be/ipj1HdG4dWA" alt="Demo video: Building an event-driven system" target="_blank">Demo: Building an event-driven system in Go</a>
  - Find more videos in our [YouTube channel](youtube.com/channel/UCvqeAqMPotfuA6SPXa4VhNQ/).

### How Encore compares to other tools

| Tool                               | What it does                                                  | How Encore differs                                                                                                                                                                                                                                                           |
| ---------------------------------- | ------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Pulumi / CDK / Terraform / SST** | Infrastructure-as-Code tools for provisioning cloud resources | Other solutions define configuration for specific infra services, coupling your application to one set of infrastructure. Encore uses semantic infrastructure (services, databases, Pub/Sub), write once, deploy anywhere (AWS/GCP/local), colocated or distributed services |
| **Serverless Framework / Chalice** | Frameworks for deploying serverless functions to AWS          | Encore supports any architecture (monolith, microservices, serverless) and any cloud (AWS, GCP, self-hosted)                                                                                                                                                                 |
| **NestJS / Express / Fiber**       | Web frameworks for building APIs and services                 | Encore provides the same capabilities plus infrastructure primitives, local dev tooling, observability, and optional cloud deployment                                                                                                                                        |
| **Convex / Supabase / Firebase**   | Managed backend-as-a-service platforms                        | Encore gives you similar productivity but supports microservices and event-driven systems, and deploys to your own cloud account with no vendor lock-in                                                                                                                      |
| **Vercel / Netlify / Railway**     | Deployment platforms (primarily frontend/full-stack)          | Encore is backend-specialized with deeper primitives (Pub/Sub, cron, caching) and multi-cloud infrastructure automation                                                                                                                                                      |

### Why teams use Encore

- **Faster Development**: Encore streamlines the development process by providing guardrails, clear abstractions, and removing manual infrastructure tasks from development iterations.
- **Scalability & Performance**: Encore simplifies building large-scale microservices applications that can handle growing user bases and demands, without the normal boilerplate and complexity.
- **Control & Standardization**: Built-in tools like automated architecture diagrams, infrastructure tracking and approval workflows, make it easy for teams and leaders to get an overview of the entire application.
- **Security & Compliance**: Encore Cloud helps ensure your application is secure and compliant by enforcing security standards like least privilege IAM, and provisioning infrastructure according to best practices for each cloud provider.
- **Reduced Costs**: Encore Cloud's automatic infrastructure management removes common cloud expenses like overprovisioned test environments, and reduces DevOps workload.

## Open Source

Everything needed to develop and deploy Encore applications is Open Source, including the backend frameworks, parser, compiler, runtime, and CLI.

This includes all code needed for local development, everything that runs in your application when it is deployed, and everything needed to generate a Docker image for your application, so you can easily deploy your application anywhere. [Learn more in the docs](https://encore.dev/docs/ts/self-host/build).

## Join our growing developer community

Developers building with Encore are part of fast-moving teams that want to focus on creative programming and building great software to solve meaningful problems. It's a friendly place, great for exchanging ideas and learning new things!

**Join the community on [Discord](https://encore.dev/discord).**

We rely on your contributions and feedback to improve Encore for everyone who is using it.
Here's how you can contribute:

- ⭐ **Star and watch this repository to help spread the word and stay up to date.**
- Meet fellow Encore developers and chat on [Discord](https://encore.dev/discord).
- Follow Encore on [Twitter](https://twitter.com/encoredotdev).
- Share feedback or ask questions via [email](mailto:hello@encore.dev).
- Leave feedback on the [Public Roadmap](https://encore.dev/roadmap).
- Send a pull request here on GitHub with your contribution.

## Frequently Asked Questions (FAQ)

### Who's behind Encore?

Encore was founded by long-time engineers from Spotify and Google. We've lived through the challenges of building complex distributed systems with thousands of services, and scaling to hundreds of millions of users.

Encore grew out of these experiences and is a solution to the frustrations that came with them: unnecessary infrastructure complexity and tedious repetitive work that suffocates developers' productivity and creativity.

### Who is Encore for?

**Individual developers:** Build cloud applications without managing infrastructure configuration. Go from idea to deployed application in minutes instead of days.

**Startup teams:** Get a production-ready backend on AWS/GCP without dedicated DevOps engineers. Focus your time on your product instead of reinventing infrastructure patterns and building platform tooling.

**Large organizations:** Standardize backend development across teams. Reduce onboarding time and operational overhead. Spin up new services in minutes with consistent patterns, without needing days of back and forth between development and DevOps teams.

### Does defining infrastructure in code couple my app to infrastructure?

No. Encore keeps your application code cloud-agnostic by letting you refer only to **logical resources** (like "a Postgres database" or "a Pub/Sub topic"). A backend-agnostic interface means your code has no cloud-specific imports or configurations.

Encore's compiler and runtime handle the mapping of logical resources to actual infrastructure, which is configured per environment. Your code stays identical whether the environment uses e.g.:

- **AWS RDS** or **GCP Cloud SQL** for databases
- **SQS/SNS** or **GCP Pub/Sub** for messaging
- **AWS Fargate**, **Cloud Run**, or **Kubernetes** for compute

This **reduces coupling** compared to traditional Infrastructure-as-Code or cloud SDKs, which embed cloud-specific decisions directly in your codebase. With Encore, swapping cloud providers or infrastructure services requires no code changes.

### What kind of support does Encore offer?

Encore is fully open source and maintained by a dedicated full-time team. Support options include:

- **Community Support:** Free support via [Discord](https://encore.dev/discord)
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
