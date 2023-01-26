---
seotitle: Cloud Primitives are the building blocks of most cloud backend applications
seodesc: Learn how to build cloud-agnostic backend applications using Encore's built-in cloud primitives.
title: Cloud Primitives
subtitle: The common building blocks needed to build backend applications
---

Modern backend applications generally rely on the same small set of primitives, things like: Services and APIs, databases, queues, caches, and scheduled jobs.

But if there are so few basic components, why is building a cloud backend application so complex?

The problem is that while infrastructure requirements change depending on context and scale, developers are forced to build their applications with very specific underlying infrastructure in mind.

This means having to refactor the application, over and over again, to cope with infrastructure changes. What's worse is, the many hours spent manually setting up infrastructure also need to be repeated.

## Focus on your product, not your cloud provider

To avoid having to make decisions based on specific cloud services when developing your application, Encore provides cloud-agnostic solutions for all common building blocks.

This lets you logically declare the infrastructure resources you need as part of your application code. Encore then automatically provision them for you, in any environment and for all major cloud providers. We think of the approach as _infrastructure from code_.

This means that when your requirements evolve, or you want to add new environments, you don't need to do any refactoring or manual labor.

As an example, here is how you would create a PubSub topic for user signup events:

```go
import "encore.dev/pubsub"

type SignupEvent struct { UserID int }
var Signups = pubsub.NewTopic[*SignupEvent]("signups", pubsub.TopicConfig {
    DeliveryGuarantee: pubsub.AtLeastOnce,
})
```

With Encore you can always use any type cloud infrastructure, as you normally would, even if it's not an existing built-in building block. The drawback is that your developer experience will be more conventional, and you will need to manually provision and maintain it.

## Learn more about Encore's cloud-agnostic primitives

Learn more about how each primitive works with these guides.

- [Services and APIs](/docs/primitives/services-and-apis)
- [Databases](/docs/primitives/databases)
- [Cron Jobs](/docs/primitives/cron-jobs)
- [Queues](/docs/primitives/pubsub)
- [Caching](/docs/primitives/caching)
- [Secrets](/docs/primitives/secrets)

<img src="/assets/docs/primitives.png" title="Cloud Primitives" className="noshadow mx-auto d:max-w-[50%]"/>
