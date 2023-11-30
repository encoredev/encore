---
seotitle: Encore compared to Supabase / Firebase
seodesc: See how Encore's Backend Development Platform lets you unlock the simplicity of tools like Supabase and Firebase, while maintaining the control and flexibility of building a real backend application.
title: Encore compared to Supabase + Firebase
subtitle: Get the simplicity you want — with flexibility and scalability
---

Supabase and Firebase are two popular _Backend as a Service_ providers, that give developers an easy way to get a database up and running for their applications. They also bundle built-in services for certain common use cases like authentication. This can be a great way of getting off the ground quickly. But as many developers have come to learn, you risk finding yourself boxed into a corner if you're not in full control of your own backend when new use cases arise.

**Encore is not a _Backend as a Service_, it's a platform _for_ backend development**. It gives you many of the same benefits that Supabase and Firebase offer, like not needing to manually provision your [databases](/docs/primitives/databases) (or any other infrastructure for that matter). The key difference is, **Encore provisions your infrastructure in your own cloud account in AWS/GCP.** This also lets you easily use any cloud service offered by the major cloud providers, and you don't risk being limited by the platform and and having to start over from scratch.

With Encore you can always build your application using the appropriate infrastructure for your use case. Using Encore's [Infrastructure SDK](/docs/primitives/overview) to **declare your infrastructure as type-safe objects in your application code, and Encore will [automatically provisions the necessary infrastructure](/docs/deploy/infra)**. This works the same way in all types of environments (local, preview, cloud) and across the major cloud providers (GCP/AWS). This means your application is cloud-agnostic by default and you avoid cloud lock-in.

You can also use any type of cloud infrastructure, as you normally would, even if it's not a built-in [building block](/docs/primitives/overview). The only drawback is that your developer experience will be more conventional, and you will need to manually provision the "unsupported" infrastructure.

|                                          | Encore                                          | Supabase             | Firebase             |
| ---------------------------------------- | ----------------------------------------------- | -------------------- | -------------------- |
| **Approach?**                            | Backend Development Platform                    | Backend as a Service | Backend as a Service |
| **Charges for hosting?**                 | No                                              | Yes                  | Yes                  |
| **Deploy to all major cloud providers?** | Yes                                             | No                   | No                   |
| **Deploy to your own cloud account?**    | Yes                                             | No                   | Yes (GCP only)       |
| **Cloud lock-in?**                       | No                                              | Always               | Always               |
| **Native PostgreSQL support?**           | Yes                                             | Yes                  | No                   |
| **Supports microservices?**              | Yes                                             | No                   | No                   |
| **Built-in Preview Environments?**       | Yes                                             | No                   | No                   |
| **Automatic Distributed Tracing?**       | Yes                                             | No                   | No                   |
| **Pricing?**                             | [$99 per developer](https://encore.dev/pricing) | Variable (complex)   | Variable (complex)   |
