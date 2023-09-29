---
title: Trying Encore for an existing project
subtitle: Extending, Refactoring, and Rebuilding
seotitle: Trying Encore for an existing project
seodesc: Learn how to try Encore for your existing backend application using Extending, Refactoring, or Rebuilding, depending on your situation and priorities.
---

Making changes to your backend requires a thoughtful approach and how you best evaluate a new tool, like Encore, depends on your situation and priorities. Here we’ll explore three approaches and introduce the common scenarios and procedures for each:
- **Extend:** Using Encore to speed up building an independent new system or creating proof of concept.
- **Refactor:** Using Encore when refactoring an existing backend to unlock productivity benefits and remove complexity.
- **Rebuild:** Using Encore when rebuilding an existing application from the ground up, ensuring modern best practices and cloud-portability.

## Extend
Extending your existing backend best suits teams who are mostly satisfied with their current setup, but are on the lookout for more efficient workflows to cut down delivery times for new projects, or wish to improve the developer experience for ongoing development.

### Use cases
- Extending an existing application with a new service or system, integrated using APIs.
- Reducing effort when building a new system in an isolated domain, such as a new product experiment.
- Tackling an independent project that demands fast delivery times.

### When to consider Encore
If your existing setup feels right but you’re curious about Encore, evaluating it in an independent project is the right move.
For example when:
- You want to create a new service or system, and deploy it to your cloud in **AWS** or **GCP**, without manual infrastructure setup.
- You want to try out development tools like [preview environments](/docs/deploy/preview-environments), and [local tracing](/docs/observability/dev-dash), without any manual instrumentation.
- You want to validate Encore’s workflow and reliability without making changes to existing systems.

### How to adopt Encore when Extending
- **1. Identify Extension Points:** Decide on an upcoming project, or proof of concept, that is relatively independent from your existing application and is appropriate for building as a new service or system.
- **2. Create New Services:** Develop new services or systems using Encore’s [API Framework](/docs/primitives/services-and-apis) and [Infrastructure SDK](/docs/primitives) to get off the ground quickly. This lets you try out all Encore features and enables you to design your new system with Encore’s [automatic architecture diagrams](/docs/observability/encore-flow).
- **3. Integrate via APIs:**  Where relevant, integrate your new system with your existing backend application using APIs. This can be made simpler by using Encore’s [generated API clients](/docs/develop/client-generation).
- **3. Validate & Iterate:** Deploy the new services to an [cloud environment](/docs/deploy/infra), automatically provisioned by Encore, and validate their performance and interoperability. Use Encore’s [distributed tracing](/docs/observability/tracing) to find bugs or performance issues.
- **4. Connect cloud and Deploy:** When you are satisfied that you application is working as expected, [connect your cloud account](/docs/deploy/own-cloud) (AWS or GCP) and create a production environment for your application. Encore automatically provisions the infrastructure needed using each cloud’s native services, or you can deploy your application into an [existing Kubernetes cluster](/docs/how-to/import-kubernetes-cluster).

## Refactor
Refactoring can serve as a breath of fresh air for your existing code, revitalizing it by optimizing existing structures. In this approach, your goal is to improve on your existing backend application, often focusing on shedding unnecessary complexity and enabling new opportunities.

### Use cases
- Transforming a **monolith** into **microservices**.
- Changing system architecture, e.g. moving to an [event-driven architecture](/blog/event-driven-architecture).
- Cloud migration, e.g. from **AWS** to **GCP**.
- Changing foundational infrastructure, e.g. migrating to **Kubernetes**.
- Removing unwanted complexity that’s become engrained as you’ve scaled up quickly.

### When to consider Encore
Your application is already built using a supported programming language like **Go** (and soon, **TypeScript**) and you want to unlock modern development tools like [infrastructure automation](/docs/deploy/infra), [preview environments](/docs/deploy/preview-environments), and [distributed tracing](/docs/observability/tracing), with minimal adjustments to your existing backend and no manual setup.

### How to adopt Encore in a Refactor
- **1. Assess Your Goal:** Start by evaluating what changes you want to make to your existing application, look for unnecessary complexities and bottlenecks that can be eliminated. Depending on your goal, you can decide if you want to fully implement Encore’s [API Framework](/docs/primitives/services-and-apis) or if you prefer to minimize changes by using a catch-all handler on your current router. Keep in mind that in order to use features like the [Service Catalog](/docs/develop/api-docs) and [API Explorer](/docs/observability/dev-dash), you need to use the API Framework. ([Learn more about the different options](https://encore.dev/docs/how-to/migrate-to-encore))
- **2. Implement Infrastructure SDK:** Implement Encore's [Infrastructure SDK](/docs/primitives) in your application by replacing existing infrastructure configuration and boilerplate. This enables you to use Encore's infrastructure automation and removes the hassle of manual infrastructure setup. **Tip:** [Existing databases can be integrated](/docs/how-to/connect-existing-db) so you don’t need to migrate existing data.
- **3. Resolve compile-time errors:** Encore comes with a parser and compiler that ensures your application correctly implements the API Framework and Infrastructure SDK. This lets you discover problems at compile time and provides insightful error messages to help you quickly resolve any errors.
- **4. Test & Iterate:** Test the refactored application to ensure stability and reliability using Encore’s automatically provisioned cloud [environments](/docs/deploy/environments) and [distributed tracing](/docs/observability/tracing) for fast debugging and iteration. If relevant, you can use a [generated client](/docs/develop/client-generation) to integrate with your existing application frontend.
- - **5. Connect cloud and Deploy:** When you are satisfied that you application is working as expected, [connect your cloud account](/docs/deploy/own-cloud) (AWS or GCP) and create a production environment for your application. Encore automatically provisions the infrastructure needed using each cloud’s native services, or you can deploy your application into an [existing Kubernetes cluster](/docs/how-to/import-kubernetes-cluster).

## Rebuild
The Rebuild strategy is for those who want a fresh start by recreating an application from the ground up. It’s particularly relevant for companies looking to make a bigger change like changing programming language or migrating from legacy self-hosted infrastructure. A full rebuild, although potentially labor-intensive, opens up opportunities to harness the latest cloud services and developer tools like Encore.

### Use cases
- Changing programming languages to adopt more performant or modern ones for your project.
- Migrating from legacy self-hosted solutions to scalable cloud providers like **AWS** or **GCP**.
- Starting fresh by recreating an app from the ground up.

### When to consider Encore
- You’re intending to use a supported programming language like **Go** (and soon, **TypeScript**).
- You want to leverage the scalability and services of cloud providers like **AWS** or **GCP**, but don’t want to become locked-in to one specific provider. (Encore applications are cloud-portable by default.)
- You want modern development tools like [infrastructure automation](/docs/deploy/infra), [preview environments](/docs/deploy/preview-environments), and [distributed tracing](/docs/observability/tracing), without manual setup or instrumentation.

### How to adopt Encore in a Rebuild
- **1. Plan & Design:** Start by creating a design, considering the application's core requirements and architecture. Decide on the programming language, keeping in mind Encore's supported languages.
**- 2. Develop from Scratch:** Develop your new application using Encore's [API Framework](/docs/primitives/services-and-apis) and [Infrastructure SDK](/docs/primitives) to get up and running quickly in a shared environment using Encore’s built-in development cloud.
- **3. Test & Iterate:** Test your new application to ensure reliability using Encore’s [distributed tracing](/docs/observability/tracing) for fast debugging and iteration. Use the [generated API clients](/docs/develop/client-generation) to integrate with your application frontend.
- **4. Connect cloud and Deploy:** When you are satisfied that you application is working as expected, [connect your cloud account](/docs/deploy/own-cloud) (AWS or GCP) and create a production environment for your application. Encore automatically provisions the infrastructure needed using each cloud’s native services, or you can deploy your application into an [existing Kubernetes cluster](/docs/how-to/import-kubernetes-cluster).

## Get support adopting Encore
Each approach has different benefits and is relevant in different scenarios. Which one is right for your team depends on your priorities and existing setup.

Whether it’s expanding your horizons with **Extend**, revitalizing existing structures through **Refactor**, or starting afresh with **Rebuild**, we’re available to support as you explore Encore to unlock improved productivity and developer experience.

We're happy to chat through your use case in a call. You can also [join Slack](https://encore.dev/slack) to ask questions and meet fellow Encore developers.
<a href="/book">
    <Button className="mt-4" kind="primary" section="white">Book call</Button>
</a>
