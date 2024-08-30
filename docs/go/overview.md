---
seotitle: Start building backends using Encore.go
seodesc: Learn how Encore's Go Backend Framework works, and get to know the powerful features that help you build cloud backend applications easier than ever before.
title: Encore.go Backend Framework
subtitle: Learn how to use Encore.go to build scalable backend applications and distributed systems
toc: false
lang: go
---

Modern backend applications rely on a common set of infrastructure primitives for most of the behavior. To improve your development workflow, Encore.go enables you to declare these primitives as type-safe objects in application code. Encore.go then takes care of running your local environments, provides observability and documentation, and helps you [integrate with cloud infrastructure](/docs/deploy/infra). Encore also offers a [Cloud Platform](/use-cases/devops-automation) which fully automates infrastructure and DevOps for your cloud on AWS/GCP.

**See how to use each primitive:**
- [Services and APIs](/docs/primitives/services-and-apis)
- [Databases](/docs/primitives/databases)
- [Cron Jobs](/docs/primitives/cron-jobs)
- [Pub/Sub & Queues](/docs/primitives/pubsub)
- [Caching](/docs/primitives/caching)
- [Secrets](/docs/primitives/secrets)

Check out the [Tutorials](/docs/tutorials) section for examples of complete Encore applications.

<img src="/assets/docs/primitives.png" title="Cloud Primitives" className="noshadow mx-auto d:w-1/2"/>

## Benefits of using Encore

Using the Go Backend Framework to declare infrastructure in application code helps unlock these benefits:

- **Develop new features locally as if the infrastructure is already set up**: Encore automatically compiles your app and sets up the necessary infrastructure on the fly.
- **Faster feedback loops:** With type-safe infrastructure you can identify problems as early as in your code editor, instead of learning about issues via the — much slower — deployment cycle.
- **No manual maintenance required**: There is no need to manually write [IaC](/resources/infrastructure-as-code) configuration, like Terraform. Encore uses your application code as the source of truth and automatically keeps all environments in sync.
- **One codebase for all environments**: Encore [automatically provisions](/docs/deploy/infra) your local, [preview](/docs/deploy/preview-environments), and cloud environments (using [your own cloud account](/docs/deploy/own-cloud)) on AWS/GCP.
- **Cloud-agnostic by default**: The Backend Framework is an abstraction layer on top of the cloud provider's APIs, so you avoid becoming locked in to a single cloud.
- **Evolve infrastructure without code changes**: As your requirements evolve, you can change and configure the provisioned infrastructure by using Encore's Cloud Dashboard or your cloud provider's console.

## Simplicity without giving up flexibility

While most requirements are met by a common set of infrastructure primitives, sooner or later you will likely need something highly specific to your problem domain. Encore is designed to ensure you can use any cloud infrastructure, even if it's not built into Encore's Backend Framework. This works seamlessly since Encore [provisions infrastructure](/docs/deploy/infra) in your own cloud account, so you can use any of your cloud provider's services as you traditionally would.
