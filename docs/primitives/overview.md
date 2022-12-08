---
seotitle: Cloud Primitives are the building blocks of most cloud backend applications
seodesc: Learn how to build cloud-agnostic backend applications using Encore's built-in cloud primitives.
title: Cloud Primitives
subtitle: The common building blocks needed to build most backend applications
---

Modern backend applications generally rely on the same small set of primitives to create most of the functionality. These primitives are things like: Services and APIs, databases, queues, caches, and scheduled jobs.

But if there are so few basic components, why is building a cloud backend application so complex?

The problem is that while infrastructure requirements change depending on context and scale, developers are forced to build their applications with very specific underlying infrastructure in mind.

This means having to refactor the application, over and over again, to cope with infrastructure changes. What's worse is, the many hours spent manually setting up infrastructure also need to be repeated.

## Focus on your product, not your cloud provider

To avoid having to make decisions based on specific cloud services when developing your application, the Encore Framework provides cloud-agnostic solutions for all common building blocks.

By letting you logically declare the infrastructure resources you need as part of your application code, Encore can understand the infrastructure requirements and automatically provision them for you. We think of the approach as _infrastructure from code_.

As an example, here is how you would create a PubSub topic for user signup events:

```go
import "encore.dev/pubsub"

type SignupEvent struct { UserID int }
var Signups = pubsub.NewTopic[*SignupEvent]("signups", pubsub.TopicConfig {
    DeliveryGuarantee: pubsub.AtLeastOnce,
})
```

When deploying your application, Encore understands what the infrastructure requirements are, and automatically provisions the necessary infrastructure in all environment types and across all major cloud providers.

This means that when your requirements evolve, or you want to add new environments, you don't need to do any refactoring or manual labor.

**See how to use Encore's cloud-agnostic APIs for these common building blocks:**

- [Services and APIs](/docs/primitives/services-and-apis)
- [Databases](/docs/primitives/databases)
- [Cron Jobs](/docs/primitives/cron-jobs)
- [Queues](/docs/primitives/pubsub)
- [Caching](/docs/primitives/caching)
- [Secrets](/docs/primitives/secrets)

<img src="/assets/docs/primitives.png" title="Cloud Primitives" className="noshadow mx-auto d:max-w-[50%]"/>
