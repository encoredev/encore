<p align="center" dir="auto">
<a href="https://encore.dev"><img src="https://user-images.githubusercontent.com/78424526/214602214-52e0483a-b5fc-4d4c-b03e-0b7b23e012df.svg" width="160px" alt="encore icon"></img></a>
<br/><br/>
<b>Encore: Infrastructure orchestration from local to your cloud</b><br/><br/>

[![License](https://img.shields.io/badge/license-MPL--2.0-blue.svg)](LICENSE)
[![Discord](https://img.shields.io/discord/814482502336905216?label=discord)](https://encore.dev/discord)
[![Go SDK](https://img.shields.io/badge/go-encore.dev-00ADD8)](https://pkg.go.dev/encore.dev)
[![TS SDK](https://img.shields.io/npm/v/encore.dev?label=npm)](https://www.npmjs.com/package/encore.dev)

</p>

## What is Encore?

Encore is the infrastructure platform for the intelligence era, where engineers and AI agents build production systems without waiting on infrastructure. Run and validate with real infrastructure locally and in preview environments, then deploy to your own AWS or GCP account.

It has two parts that work together:

- An open source **infrastructure SDK** for TypeScript and Go. Declare the resources your app needs (databases, Pub/Sub, object storage, caches, cron jobs, secrets) directly in your code, and `encore run` boots the whole system locally with real Postgres, real Pub/Sub semantics, and distributed tracing. No Docker Compose, no Localstack.
- **Encore Cloud**, an optional managed platform that uses the same model to spin up a per-PR preview environment in your own VPC and provision matching resources in your AWS or GCP account on deploy. No Terraform PR, no CI/CD pipeline to maintain, IAM and firewall rules generated from real code paths.

You can also use just the SDK and provision everything yourself with Terraform or any other tool.

### Example: Declaring resources

```typescript
import { SQLDatabase } from "encore.dev/storage/sqldb";
import { Topic } from "encore.dev/pubsub";
import { Bucket } from "encore.dev/storage/objects";

// Define a Postgres database
const db = new SQLDatabase("users", { migrations: "./migrations" });
// Define a Pub/Sub topic
const signups = new Topic<SignupEvent>("signups", {
  deliveryGuarantee: "at-least-once"
});
// Define an object storage bucket
const avatars = new Bucket("avatars", { versioned: true });
```

Each declaration becomes a node in the application graph. Encore runs the migrations and stands up local equivalents on `encore run`, and provisions the cloud equivalents on deploy:

| Resource       | Local              | AWS             | GCP             |
| -------------- | ------------------ | --------------- | --------------- |
| SQL Database   | Postgres           | RDS             | Cloud SQL       |
| Pub/Sub        | NSQ                | SNS + SQS       | Cloud Pub/Sub   |
| Object Storage | Local FS           | S3              | GCS             |
| Cache          | Redis              | ElastiCache     | Memorystore     |
| Cron           | Manually triggered | Encore Cloud    | Encore Cloud    |
| Secrets        | Encore vault       | Secrets Manager | Secret Manager  |
| Compute        | Local              | Fargate / EKS   | Cloud Run / GKE |

When you push a change that adds or modifies a resource, Encore diffs the application graph against the environment, provisions whatever is missing in your AWS or GCP account, and rolls out the new code against it.

### Configuration

Encore separates application semantics from environment-specific configuration. In code, you define what resources your app needs, not how each environment should configure them.
That keeps services portable across clouds, regions, accounts, scale profiles, and local development.

Encore provisions every resource with sane production defaults, then helps you manage configuration separately from your application code:

- **Encore Cloud dashboard:** easy to use knobs for all common settings like process allocation, instance sizes, replicas, etc. [See more in docs](https://encore.dev/docs/platform/infrastructure/configuration).
- **Your AWS or GCP console:** tweak anything directly in your cloud provider console. Encore picks up the changes on the next deploy.
- **IaC:** manage config for Encore-provisioned resources alongside the rest of your infrastructure via the [Terraform provider](https://encore.dev/docs/platform/integrations/terraform).

### Language SDKs

| Language   | Docs                                             |
| ---------- | ------------------------------------------------ |
| TypeScript | [encore.dev/docs/ts](https://encore.dev/docs/ts) |
| Go         | [encore.dev/docs/go](https://encore.dev/docs/go) |
| Python     | Coming soon                                      |

## The Development Workflow

The same infrastructure model runs locally, in per-PR preview environments, and in production:

1. **Local.** `encore run` boots the whole system on your laptop: real Postgres, real Pub/Sub semantics, type-safe service-to-service calls, plus a local dashboard with distributed tracing. No Docker Compose, no Localstack. [Infrastructure namespaces](https://encore.dev/docs/ts/cli/infra-namespaces) let multiple branches or agents work in parallel with isolated state.
2. **Per-PR preview environment.** Open a pull request and Encore Cloud spins up an ephemeral environment in your own VPC, with the same infrastructure model and (optionally) a [database branched from a seed environment](https://encore.dev/docs/platform/infrastructure/neon). End-to-end validation against real cloud services before merge.
3. **Production.** Push to deploy. Encore diffs the application graph against the environment, provisions whatever is missing in your AWS or GCP account, generates least-privilege IAM from real code paths, and rolls out the new code. No Terraform PR, no console wizard.

The fail-loop moves from "push, wait, fix" to "run locally, see it work, push." This tight loop is also what makes Encore particularly effective with AI coding agents, since every change can be validated end-to-end against real infrastructure rather than guessed at. See the [Development Workflow](https://encore.dev/docs/platform/workflow) docs for the full picture.

## Adopting Encore

You don't need a rewrite, and you don't need to use Encore for every resource in your stack.

- **Use any infrastructure as normal:** Encore doesn't try to own every resource. Import the AWS SDK, GCP client libraries, or any third-party API and provision the resource yourself, or wire it up alongside Encore-managed resources via the [Terraform provider](https://encore.dev/docs/platform/integrations/terraform).
- **Migrate service-by-service:** Build new services in Encore and run them next to your existing system, integrated over APIs.
- **Deploy where your stack already lives:** Encore can deploy into your existing Kubernetes cluster or into your VPC in AWS or GCP.

Start with a low-risk, frequently-changed service. See the [migration guide](https://encore.dev/docs/platform/migration/migrate-to-encore) for the full playbook.

### Migrating Away

Encore is designed to make leaving easy. 99% of your code is regular Go or TypeScript, so there's not much to rewrite.
See the [migrate-away guide](https://encore.dev/docs/ts/migration/migrate-away) for more.

## Limitations

Encore integrates at the application layer, which means a few constraints to be clear about up front:

- **Language:** Your services need to be written in a supported language: TypeScript (Node.js) or Go. Python is coming soon.
- **Infrastructure scope:** Encore is designed to solve the 99% use case, making it easier to work with the resources you use over and over again; services, databases, Pub/Sub, object storage, caches, cron jobs, and secrets. For the remaining 1% that is specific to your domain, you can still integrate any other service as you normally would. Encore doesn't prevent it or make it harder.
- **Cloud providers:** Encore Cloud's fully-automated provisioning currently supports AWS and GCP. Azure is on the roadmap. Self-hosting via `encore build docker` works on any provider.

## How Encore Compares

| Tool                                   | What it does                                            | How Encore differs                                                                                                                         |
| -------------------------------------- | ------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| **Pulumi / CDK / Terraform / SST**     | Infrastructure-as-Code for provisioning cloud resources | No separate IaC to write. Infrastructure is generated from your application code, so the same code runs locally and deploys to AWS or GCP. |
| **Convex / Supabase / Firebase**       | Managed backend-as-a-service platforms                  | Runs in your own AWS or GCP account. No vendor lock-in.                                                                                    |
| **Render / Fly.io / Railway / Vercel** | PaaS-style deployment platforms                         | Deploys to your own cloud account, not a managed runtime.                                                                                  |

## Quick Start

```bash
encore app create     # scaffold a project
cd myapp
encore run            # run locally with provisioned infra + dev dashboard
```

Install Encore:

- **macOS:** `brew install encoredev/tap/encore`
- **Linux:** `curl -L https://encore.dev/install.sh | bash`
- **Windows:** `iwr https://encore.dev/install.ps1 | iex`

Full walkthrough in the [Quickstart guide](https://encore.dev/docs/ts/quick-start).

## AI Integration

Encore is built for AI-assisted development. When you run `encore app create`, you can pick your AI tool (Cursor, Claude Code, etc.) and Encore generates the right rules files for it, plus configures an [MCP server](https://encore.dev/docs/ts/cli/mcp) that lets agents introspect your app: services, APIs, databases, traces. Combined with the fast local-to-preview-env iteration loop above, agents can validate their own changes end-to-end against real infrastructure instead of guessing.

See the [AI integration docs](https://encore.dev/docs/ts/ai-integration) for more details.

## Local Dev Dashboard

The Encore CLI ships a local dashboard for inspecting services, APIs, traces, databases, Pub/Sub messages, and architecture diagrams in real time. Run `encore run` and it is there at `localhost:9400`.

https://github.com/user-attachments/assets/461b902f-8fd3-46f1-a73c-0ebbfa789ce3

## Deployment Platform

[Encore Cloud](https://encore.dev/docs/platform) is the optional managed platform. It connects to your AWS or GCP account and provisions the resources your code declares in your own VPC. Other features:

- Preview environments for each PR
- Self-serve infrastructure provisioning, with least-privilege IAM and firewall rules derived from real code paths
- Distributed tracing, metrics, and logs across services
- Cost analytics and infrastructure approval workflows
- Auto-generated service catalog and architecture diagrams from your code

See [pricing](https://encore.dev/pricing) for details.

You can also skip the platform entirely. Run `encore build docker` to produce a standalone image and supply your own infra config (provisioned via Terraform, Pulumi, or any other tool). See the [self-hosting guide](https://encore.dev/docs/ts/self-host/build).

## Who's Using Encore

150+ Teams are already shipping production apps with Encore, including: Groupon, Echo.xyz (a Coinbase company), Bookshop.org, Gradient Labs, Ashby, Later.com, Pallet, Pave Bank, and Playwire.
Use cases span AI, fintech, logistics, commerce, web3, and more. See [case studies](https://encore.dev/customers) to learn more.

## Resources

- [Encore website](https://encore.dev)
- [Documentation](https://encore.dev/docs)
- [Migration guide](https://encore.dev/docs/platform/migration/migrate-to-encore)
- [Example apps](https://github.com/encoredev/examples/)
- [Discord](https://encore.dev/discord)
- [Book a 1:1 demo](https://encore.dev/book)
- [Public Roadmap](https://encore.dev/roadmap)
- [Contributing](CONTRIBUTING.md)

## License

Encore is licensed under the [Mozilla Public License 2.0](LICENSE).

The framework, parser, compiler, runtime, CLI, and everything needed to develop, build, and self-host an Encore application is Open Source. Encore Cloud, the optional managed deployment platform, is a commercial service.

See [CONTRIBUTING.md](CONTRIBUTING.md) for additional details.
