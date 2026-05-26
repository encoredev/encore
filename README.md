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

Encore compiles infrastructure from your application code and manages it across local and cloud environments.

Build a backend that runs the same way on your laptop and in production: type-safe, traceable, and deployable to your own AWS or GCP account without Terraform or YAML.

You declare resources semantically in code ("I need a Postgres database") and Encore parses them into an application graph it can compile for each environment: local sandbox, managed cloud on AWS/GCP, or self-hosted Docker.

Because infrastructure is derived from the code that uses it, it stays in sync as your application evolves.

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

When you push a change that adds or modifies a resource, Encore diffs the application graph against the environment, provisions whatever is missing in your AWS or GCP account, and rolls out the new code against it. No Terraform module, no console wizard, no YAML.

### Language SDKs

| Language   | Docs                                             |
| ---------- | ------------------------------------------------ |
| TypeScript | [encore.dev/docs/ts](https://encore.dev/docs/ts) |
| Go         | [encore.dev/docs/go](https://encore.dev/docs/go) |
| Python     | Coming soon                                      |

## What Changes

Before Encore:

- Configure Localstack and docker compose to run app locally while developing.
- Write Terraform to go to prod. Open a PR. Get approval. Apply.
- Something breaks: IAM doesn't match what the code calls, a queue name drifts, a secret is missing in prod.
- Fix, push, wait for the next apply, find the next thing.
- Application code and infrastructure code meet for the first time in production.

After Encore:

- Declare the resource in code. Run locally with `encore run`.
- Open a PR and get a cloud preview environment for end-to-end testing.
- `git push` to deploy. Infrastructure is compiled from the application code, validated at build time.
- IAM is configured from real code paths. No missing permissions. No unnecessary access.

The fail-loop moves from "push, wait, fix" to "run locally, see it work, push."
Platform teams set guardrails once. Encore enforces them on every deploy.

## Adopting Encore

You don't need a rewrite, and you don't need to use Encore for every resource in your stack.

- **Use any infrastructure as normal:** Encore doesn't try to own every resource. Import the AWS SDK, GCP client libraries, or any third-party API and provision the resource yourself, or wire it up alongside Encore-managed resources via the [Terraform provider](https://encore.dev/docs/platform/integrations/terraform).
- **Migrate service-by-service:** Build new services in Encore and run them next to your existing system, integrated over APIs.
- **Deploy where your stack already lives:** Encore can deploy into your existing Kubernetes cluster or into your VPC in AWS or GCP.

Start with a low-risk, frequently-changed service. See the [migration guide](https://encore.dev/docs/platform/migration/migrate-to-encore) for the full playbook.

## Limitations

Encore integrates at the application layer, which means a few constraints to be clear about up front:

- **Language:** Your services need to be written in a supported language: TypeScript (Node.js) or Go. Python is coming soon.
- **Infrastructure scope:** Encore is design to solve for the 99% use case, making it easier to work with resources you use over and over again; services, databases, Pub/Sub, object storage, caches, cron jobs, and secrets. For the remaining 1% that is specific to your domain, you can still integrate any other service as you normally would. Encore doesn't prevent it or make it harder.
- **Cloud providers:** Encore Cloud's fully-automated provisioning currently supports AWS and GCP. Azure is on the roadmap. Self-hosting via `encore build docker` works on any provider.

## How Encore Compares

| Tool                                   | What it does                                            | How Encore differs                                                                |
| -------------------------------------- | ------------------------------------------------------- | --------------------------------------------------------------------------------- |
| **Pulumi / CDK / Terraform / SST**     | Infrastructure-as-Code for provisioning cloud resources | No separate IaC to write. Infrastructure is generated from your application code, so the same code runs locally and deploys to AWS or GCP. |
| **Convex / Supabase / Firebase**       | Managed backend-as-a-service platforms                  | Runs in your own AWS or GCP account. No vendor lock-in.                           |
| **Render / Fly.io / Railway / Vercel** | PaaS-style deployment platforms                         | Deploys to your own cloud account, not a managed runtime.                         |

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

Encore is built for AI-assisted development. Every Encore app comes with built-in CLAUDE.md and MCP server that lets agents introspect your app and generate type-safe code that follows your patterns.
See the [AI integration docs](https://encore.dev/docs/ts/ai-integration) for more details.

## Local Dev Dashboard

The Encore CLI ships a local dashboard for inspecting services, APIs, traces, databases, Pub/Sub messages, and architecture diagrams in real time. Run `encore run` and it is there at `localhost:9400`.

https://github.com/user-attachments/assets/461b902f-8fd3-46f1-a73c-0ebbfa789ce3

## Deployment Platform

Encore Cloud is the optional managed platform. It connects to your AWS or GCP account and provisions the resources your code declares in your own VPC. Other features:

- Preview environments for each PR
- Distributed tracing, metrics, and logs across services
- Architecture diagrams generated from the application graph
- Cost analytics and infrastructure approval workflows
- Service catalog and API documentation

See [pricing](https://encore.dev/pricing) and learn more.

You can also skip the platform entirely. Run `encore build docker` to produce a standalone image you supply an infra config to.

## Who's Using Encore

150+ Teams are already shipping production apps with Encore, including: Groupon, Echo.xyz (a Coinbase company), Bookshop.org, Gradient Labs, Ashby, Later.com, Pallet, Pave Bank, and Playwire.
Use cases span AI, fintech, logistics, commerce, web3, and more. See [case studies](https://encore.cloud/customers) to learn more.

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
