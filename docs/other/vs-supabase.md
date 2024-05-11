---
seotitle: Encore compared to Supabase / Firebase
seodesc: See how Encore's Backend Development Platform lets you unlock the simplicity of tools like Supabase and Firebase, while maintaining the control and flexibility of building a real backend application.
title: Encore compared to Supabase + Firebase
subtitle: Get the simplicity you want — with flexibility and scalability
---

Supabase and Firebase are two popular _Backend as a Service_ providers, that provide developers with an easy way to get a database up and running for their applications. They also bundle some built-in services for common use cases like authentication. 

This can be a great way of getting off the ground quickly. But as many developers have come to learn, you risk finding yourself boxed into a corner if you're not in full control of your own backend when new use cases arise.

**Encore is not a _Backend as a Service_, it's a platform _for_ backend development**. It gives you many of the same benefits that Supabase and Firebase offer, like not needing to manually provision your [databases](/docs/primitives/databases) (or any other infrastructure for that matter). The key difference is, **Encore provisions your infrastructure in your own cloud account on AWS/GCP.** This also lets you easily use any cloud service offered by the major cloud providers, and you don't risk being limited by the platform and and having to start over from scratch.

Let's take a look at how Encore compares to BaaS platforms like Supabase and Firebase:

|                                                     | Encore                       | Supabase             | Firebase             |
| --------------------------------------------------- | ---------------------------- | -------------------- | -------------------- |
| **Approach?**                                       | Backend Development Platform | Backend as a Service | Backend as a Service |
| **Native PostgreSQL support?**                      | ✅︎ Yes                        | ✅︎ Yes                | ❌ No                 |
| **Support pgvector for AI use cases?**              | ✅︎ Yes                        | ✅︎ Yes                | ❌ No                 |
| **Supports major cloud providers like AWS/GCP?**    | ✅︎ Yes                        | ❌ No                 | ✅︎ Yes (GCP only)     |
| **Supports Microservices?**                         | ✅︎ Yes                        | ❌ No                 | ❌ No                 |
| **Supports Event-Driven systems?**                  | ✅︎ Yes                        | ❌ No                 | ❌ No                 |
| **Supports Kubernetes and custom infra?**           | ✅︎ Yes                        | ❌ No                 | ❌ No                 |
| **Infrastructure is Type-Safe?**                    | ✅︎ Yes                        | ❌ No                 | ❌ No                 |
| **Built-in local dev environment?**                 | ✅︎ Yes                        | ❌ No                 | ❌ No                 |
| **Built-in Preview Environments per Pull Request?** | ✅︎ Yes                        | ❌ No                 | ❌ No                 |
| **Built-in Distributed Tracing?**                   | ✅︎ Yes                        | ❌ No                 | ❌ No                 |
| **Charges for hosting?**                            | No                           | Yes                  | Yes                  |

## Encore is the simplest way of accessing the full power and flexibility of the major cloud providers

With Encore you don't need to be a cloud expert to make full use of the services offered by major cloud providers like AWS and GCP.

You simply use Encore's [Backend SDK](/docs/primitives) to **declare the infrastructure semantics directly in your application code**, and Encore then [automatically provisions the necessary infrastructure](/docs/deploy/infra) in your cloud, and provides a local development environment that matches your cloud environment.

### Example: Using PostgreSQL with Encore

Here's an example of how to use Encore's [Backend SDK](/docs/primitives) to define a PostgreSQL database (Go is used in the example, TypeScript support is also available):

To create a database, import `encore.dev/storage/sqldb` and call `sqldb.NewDatabase`, assigning the result to a package-level variable.
Databases must be created from within an [Encore service](/docs/primitives/services-and-apis).

For example:

```
-- todo/db.go --
package todo

// Create the todo database and assign it to the "tododb" variable
var tododb = sqldb.NewDatabase("todo", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// Then, query the database using db.QueryRow, db.Exec, etc.
-- todo/migrations/1_create_table.up.sql --
CREATE TABLE todo_item (
  id BIGSERIAL PRIMARY KEY,
  title TEXT NOT NULL,
  done BOOLEAN NOT NULL DEFAULT false
  -- etc...
);
```

As seen above, the `sqldb.DatabaseConfig` specifies the directory containing the database migration files,
which is how you define the database schema.

With this code in place Encore will automatically create the database when starting `encore run` (locally)
or on the next deployment (in the cloud). Encore automatically injects the appropriate configuration to authenticate
and connect to the database, so once the application starts up the database is ready to be used.

[Learn more about using databases with Encore](/docs/primitives/databases)

## Encore makes it simple to build type-safe event-driven systems

Unlike BaaS platforms like Supabase and Firebase, Encore has extensive support for building microservices backends and event-driven systems.

For example, Encore lets you [define APIs](/docs/primitives/services-and-apis) using regular functions and enables cross-service type-safety with IDE auto-complete when making API calls between services.

With Encore's [Backend SDK](/docs/primitives), you can build event-driven systems by defining Pub/Sub topcis and subscriptions as type-safe objects in your application.
This gives you type-safety for Pub/Sub with compilation errors for any type-errors.

## Encore's local development workflow lets application developers focus

When using BaaS service like Supabase to handle your infrastructure, you're not at all solving for local development.

This means, with Supabase, developers need to manually set up and maintain their local environment in order to facilitate local development and testing.

This can be a major distraction for application developers, because it forces them to spend time learning how to setup and maintain various local versions of cloud infrastructure, e.g. by using Docker Compose. This work is a continuous effort as the system evolves, and becomes more and more complex as the service and infrastructure footprint grows.

All this effort takes time away from product development and slows down onboarding time for new developers.

**When using Encore, your local and cloud environments are both defined by the same code base: your application code.** This means developers only need to use `encore run` to start their local dev envioronments. Encore's Open Source CLI takes care of setting up local version of all infrastructure and provides a [local development dashboard](/docs/observability/dev-dash) with built-in observability tools.

This greately speeds up development iterations as developers can start using new infrastructure immediately, which makes building new services and event-driven systems extremely efficient.

## Encore provides an end-to-end purpose-built workflow for cloud backend developement

Encore does a lot more than just automate infrastructure provisioning and configuration. It's designed as a purpose-built tool for cloud backend development and comes with out-of-the-box tooling for both development and DevOps.

### Encore's built-in developer tools
- Cross-service type-safety with IDE auto-complete
- Distributed Tracing
- Test Tracing
- Automatic API Documentation
- Automatic Architecture Diagrams
- API Client Generation
- Secrets Management
- Service/API mocking

### Encore's built-in DevOps tools
- Automatic Infrastructure provisioning on AWS/GCP
- Infrastructure Tracking & Approvals workflow
- Cloud Configuration 2-way sync between Encore and AWS/GCP
- Automatic least privilege IAM
- Preview Environments per Pull Request
- Cost Analytics Dashboard
- Encore Terraform provider for extending Encore with infrastructure that is not currently part of Encore's Backend SDK
