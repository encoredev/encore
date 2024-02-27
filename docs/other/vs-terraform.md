---
seotitle: Encore compared to Terraform and Pulumi
seodesc: See how Encore's infrastructure from code approach lets you avoid the common pitfalls of infrastructure as code solutions like Terraform and Pulumi.
title: Encore compared to Terraform & Pulumi
subtitle: How Encore is different from Infrastructure as Code tools
---

There are many tools designed to overcome the challenges of cloud infrastructure complexity. Terraform and Pulumi are _Infrastructure as Code_ tools that help you provision infrastructure by writing infrastructure configuration files. **Encore uses a fundamentally different approach that lets you declare infrastructure as type-safe objects in your application**.

Let's take a look at how Encire compares to IaC tools like Terraform and Pulumi:

|                                                              | Encore                                           | Terraform              | Pulumi                 |
| ------------------------------------------------------------ | ------------------------------------------------ | ---------------------- | ---------------------- |
| **Approach?**                                                | Infrastructure from Code                         | Infrastructure as Code | Infrastructure as Code |
| **Infrastructure is Type-Safe?**                             | ✓ Yes                                            | ✘ No                   | ✘ No                   |
| **Built-in local dev environment?**                          | ✓ Yes                                            | ✘ No                   | ✘ No                   |
| **Built-in Preview Environments per Pull Request?**          | ✓ Yes                                            | ✘ No                   | ✘ No                   |
| **Built-in Distributed Tracing?**                            | ✓ Yes                                            | ✘ No                   | ✘ No                   |
| **Supports major cloud providers like AWS/GCP?**             | ✓ Yes                                            | ✓ Yes                  | ✓ Yes                  |
| **Supports Kubernetes and custom infra configuration?**      | ✓ Yes                                            | ✓ Yes                  | ✓ Yes                  |
| **Need to write infra config files?**                        | ✓ No                                             | ✘ Yes                  | ✘ Yes                  |
| **Need to learn new DSL?**                                   | ✓ No                                             | ✘ Yes                  | ✓ No                   |
| **Need to maintain separate codebase for infra config?**     | ✓ No                                             | ✘ Yes                  | ✘ Yes                  |
| **Requires continuous effort to keep environments in sync?** | ✓ No                                             | ✘ Yes                  | ✘ Yes                  |
| **Pricing?**                                                 | [$299 per developer](https://encore.dev/pricing) | Variable (complex)     | Variable (complex)     |

## Encore removes manual effort and maintenance required with IaC

A common challenge with Infrastructure as Code (IaC) is that it takes a lot of manual effort to write. What's worse is, you need to repeat the effort for each new environment, or take a short cut by duplicating your prod environment and creating costly over-provisioned test or staging environments.

When you use IaC you also end up with a separate codebase to maintain and keep in sync with your application's actual requirements. The complexity and scope of this problem grows as you introduce more infrastructure and more environments. That means as your system grows, with IaC, you will need to spend more and more time to maintain your infrastructure configuration.

**Encore's _infrastructure from code_ approach means there are no configuration files to maintain**, nor any refactoring to do when changing the underlying infrastructure. Your application code is the source of truth for the semantic infrastructure requirements.

In practise, you use Encore's [Infrastructure SDK](/docs/primitives/overview) to declare infrastructure as type-safe objects in your application code, and **Encore [automatically provisions the necessary infrastructure](/docs/deploy/infra) in all environments.** Including in your own cloud, with support for major cloud providers like AWS/GCP. (This also means your application is cloud-agnostic by default and **you avoid cloud lock-in**.)

## Encore's local development workflow lets application developers focus

When using IaC to provision cloud environments, you're not at all solving for local development.

This means, with Terraform, developers need to manually set up and maintain their local environment to mimic what's running in the cloud, in order to facilitate local development and testing.

This can be a major distraction for application developers, because it forces them to spend time learning how to setup and maintain various local versions of cloud infrastructure, e.g. by using Docker Compose and NSQ. This work is a continuous effort as the system evolves, and becomes more and more complex as the footprint grows.

All this effort takes time away from product development and slows down onboarding time for new developers.

**When using Encore, your local and cloud environments are both defined by the same code base: your application code.** This means developers only need to use `encore run` to start their local dev envioronments. Encore's Open Source CLI takes care of setting up local version of all infrastructure and provides a [local development dashboard](/docs/observability/dev-dash) with built-in observability tools.

This greately speeds up development iterations as developers can start using new infrastructure immediately, which makes building new services and event-driven systems extremely efficient.

## Encore ensures your cloud environments are secure by automating IAM

When using IaC tools like Terraform, you must always assign explicit permissions using IAM idenities and IAM policies. This can be very time consuming when developing a large-scale distributed systems, and when you get it wrong it can lead to glaring security holes or unexpected system behavior.

When using Encore, IAM identities and policies are automatically defined according to best practices for least privilege security. This is possible because Encore parses your source code and builds a graph of the logical architecture, it then uses this to define the infrastructure needs. This means Encore knows exactly which services needs access to which infrastructure for your application to function as expected.

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
- Automatic Infrastructure provisioning in AWS/GCP
- Infrastructure Tracking & Approvals workflow
- Cloud Configuration 2-way sync between Encore and AWS/GCP
- Automatic least privilege IAM
- Preview Environments per Pull Request
- Cost Analytics Dashboard
- Encore Terraform provider for extending Encore with infrastructure that is not currently part of Encore's Infrastructure SDK
