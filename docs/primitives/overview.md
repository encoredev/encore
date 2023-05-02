---
seotitle: Cloud Primitives are the building blocks of most cloud backend applications
seodesc: Learn how to build cloud-agnostic backend applications using Encore's built-in cloud primitives.
title: Cloud Primitives
subtitle: Encore's Infrastructure SDK provides the building blocks for creating backend applications
---

Modern backend applications rely on a small set of infrastructure primitives for most of the behavior. To improve your development workflow, Encore provides a declarative way of using them directly in application code.

See how to use each primitive:

- [Services and APIs](/docs/primitives/services-and-apis)
- [Databases](/docs/primitives/databases)
- [Cron Jobs](/docs/primitives/cron-jobs)
- [Pub/Sub & Queues](/docs/primitives/pubsub)
- [Caching](/docs/primitives/caching)
- [Secrets](/docs/primitives/secrets)

<img src="/assets/docs/primitives.png" title="Cloud Primitives" className="noshadow mx-auto d:max-w-[50%]"/>

## Encore removes complexity so you can focus on your product, not your cloud provider

If there are so few basic components, why is building a cloud backend application so complex without tools like Encore?

The problem is: infrastructure requirements evolve depending on context and scale, yet developers traditionally have to program their applications with very specific infrastructure in mind.

In practise this leads to refactoring the application, over and over again, in order to cope with infrastructure requirement changes. Many hours also have to be spent manually setting up and configuring new infrastructure.

This is what Encore's Infrastructure SDK solves, by letting you declare infrastructure on a higher abstraction level that makes your code agnostic to any specific cloud services. This means your Encore application can be deployed using different types of infrastructure, in different clouds, without making any code changes. The best part is, Encore even takes care of automatically provisioning the necessary infrastructure when you deploy.

## Simplicity without giving up flexibility

Encore is designed to ensure you can use any type cloud infrastructure, even if it's not built into Encore's Infrastructure SDK. This works since Encore deploys to your own cloud account, so you can seamlessly use your cloud provider's services as you normally would.