---
seotitle: Encore compared to Heroku
seodesc: See how the Encore Backend Development Platform lets you avoid the lock-in problems of using Heroku.
title: Encore compared to Heroku
subtitle: Get the convenience you want — without limitations and lock-in
---

In the early days of the cloud, Heroku was seen as an innovative platform that made deployments and infrastructure management very simple. Ultimately, Heroku lost momentum and, as cloud services rapidly evolved in the past decade, the platform didn't manage to provide enough flexibility to support users' needs.

Fans of Heroku will recognize much of the same simplicity in Encore's **push to deploy** workflow — the big difference is that **Encore deploys to your own cloud in AWS/GCP**. This means you keep full flexibility to scale your application using battle-tested services from the major cloud providers, and can leverage their full arsenal of thousands of different services.

This works because Encore is designed to be flexible in enabling you to use infrastructure services not yet natively supported in Encore's [Infrastructure SDK](/docs/primitives/overview). In practice you can use any type cloud infrastructure as you normally would, the only drawback is that your developer experience will be more conventional, and you will need to manually provision the "unsupported" infrastructure.

|                                          | Encore                                          | Heroku                |
| ---------------------------------------- | ----------------------------------------------- | --------------------- |
| **Infrastructure approach?**             | Infrastructure from Code                        | Platform as a Service |
| **Charges for hosting?**                 | No                                              | Yes                   |
| **Deploy to all major cloud providers?** | Yes (AWS/GCP)                                   | No                    |
| **Deploy to your own cloud account?**    | Yes                                             | No                    |
| **Cloud lock-in?**                       | No                                              | Always                |
| **Built-in CI/CD?**                      | Yes                                             | Yes                   |
| **Built-in Preview Environments?**       | Yes                                             | Yes                   |
| **Automatic Distributed Tracing?**       | Yes                                             | No                    |
| **Pricing?**                             | [$99 per developer](https://encore.dev/pricing) | Variable (complex)    |

## Advantages of Encore's Infrastructure from Code approach

Encore's _infrastructure from code_ approach means there are no configuration files to maintain, nor any refactoring to do when changing your application's underlying infrastructure. Your application code is the source of truth for the logical infrastructure requirements!

In practise, you use Encore's [Infrastructure SDK](/docs/primitives/overview) to declare your infrastructure needs as part of your application code, and Encore [automatically provisions the necessary infrastructure](/docs/deploy/infra) in all types of environments and across all major cloud providers.
(This means your application is cloud-agnostic by default.)