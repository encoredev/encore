---
seotitle: Encore compared to Terraform and Pulumi
seodesc: See how Encore's infrastructure from code approach lets you avoid the common pitfalls of infrastructure as code solutions like Terraform and Pulumi.
title: Encore compared to Terraform / Pulumi
subtitle: Helping you avoid the pitfalls of infrastructure as code
---

There are many tools designed to overcome the challenges of cloud infrastructure complexity. Terraform and Pulumi are _infrastructure as code_ tools that help you provision infrastructure by writing infrastructure configuration files. **Encore uses a fundamentally different approach that lets you declare infrastructure as type-safe objects in your application**. Let's take a look at how they compare.

|                                                              | Encore                   | Terraform              | Pulumi                 |
| ------------------------------------------------------------ | ------------------------ | ---------------------- | ---------------------- |
| **Approach?**                                                | Infrastructure from Code | Infrastructure as Code | Infrastructure as Code |
| **Supports all major cloud providers?**                      | Yes                      | Yes                    | Yes                    |
| **Infrastructure is Type-Safe?**                             | Yes                      | No                     | No                     |
| **Write configuration files?**                               | Never                    | Always                 | Always                 |
| **Uses custom DSL?**                                         | No                       | Yes                    | No                     |
| **Separate codebase for infra config?**                      | No                       | Yes                    | Yes                    |
| **Continuous effort required to keep environments in sync?** | Never                    | Always                 | Always                 |
| **Preview Environments?**                                    | Built-in                 | Manual work            | Manual work            |
| **Automatic Distributed Tracing?**                           | Yes                      | No                     | No                     |

## Drawbacks of Infrastructure as Code

A common challenge with Infrastructure as Code, aside from being a lot of manual labor, is that you end up with a separate codebase to maintain and keep in sync with your application's actual requirements. The complexity and scope of this problem grows as you introduce development and test environments.

What's worse is, infrastructure as code does very little to help you cope with evolving infrastructure requirements. You still need to manually write new infrastructure configuration files, and refactor your application to function with the new infrastructure.

Not to mention you need to manually set up and maintain your local environment to match what you have running in the cloud.

**Encore's _infrastructure from code_ approach means there are no configuration files to maintain**, nor any refactoring to do when changing the underlying infrastructure. Your application code is the source of truth for the logical infrastructure requirements!

In practise, you use Encore's [Infrastructure SDK](/docs/primitives/overview) to declare infrastructure as type-safe objects in your application code, and **Encore [automatically provisions the necessary infrastructure](/docs/deploy/infra) in your own cloud** with the major cloud providers (AWS/GCP). (This means your application is cloud-agnostic by default and **you avoid cloud lock-in**.)