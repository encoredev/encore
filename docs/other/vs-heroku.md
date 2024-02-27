---
seotitle: Encore compared to Heroku
seodesc: See how the Encore Backend Development Platform lets you avoid the lock-in problems of using Heroku.
title: Encore compared to Heroku
subtitle: Get the convenience you want — without limitations and lock-in
---

In the early days of the cloud, Heroku was seen as an innovative platform that made deployments and infrastructure management very simple using a Platform as a Service (PaaS) approach. Ultimately, Heroku lost momentum and, as cloud services rapidly evolved in the past decade, the platform didn't manage to provide enough flexibility to support users' needs.

Fans of Heroku will recognize much of the same simplicity in Encore's **push to deploy** workflow — the big difference is that **Encore deploys to your own cloud in AWS/GCP**. This means you keep full flexibility to scale your application using battle-tested services from the major cloud providers, and can leverage their full arsenal of thousands of different services.

Let's take a look at how Encore compares to PaaS tools like Heroku:

|                                                      | Encore                                          | Heroku                |
| ---------------------------------------------------- | ----------------------------------------------- | --------------------- |
| **Infrastructure approach?**                         | Infrastructure from Code                        | Platform as a Service |
| **Built-in CI/CD?**                                  | ✅︎ Yes                                           | ✅︎ Yes                 |
| **Built-in Preview Environments?**                   | ✅︎ Yes                                           | ✅︎ Yes                 |
| **Built-in local dev environment?**                  | ✅︎ Yes                                           | ❌ No                  |
| **Built-in Distributed Tracing?**                    | ✅︎ Yes                                           | ❌ No                  |
| **Deploys to major cloud providers like AWS & GCP?** | ✅︎ Yes                                           | ❌ No                  |
| **Avoids cloud lock-in?**                            | ✅︎ Yes                                           | ❌ No                  |
| **Supports Kubernetes and custom infra?**            | ✅︎ Yes                                           | ❌ No                  |
| **Infrastructure is Type-Safe?**                     | ✅︎ Yes                                           | ❌ No                  |
| **Charges for hosting?**                             | No                                              | Yes                   |
| **Pricing?**                                         | [$99 per developer](https://encore.dev/pricing) | Variable (complex)    |

## Encore is the simplest way of accessing the full power and flexibility of the major cloud providers

With Encore you don't need to be a cloud expert to make full use of the services offered by major cloud providers like AWS and GCP.

You simply use Encore's [Infrastructure SDK](/docs/primitives) to **declare the infrastructure semantics directly in your application code**, and Encore then [automatically provisions the necessary infrastructure](/docs/deploy/infra) in your cloud, and provides a local development environment that matches your cloud environment.

You get the same, easy to use, "push to deploy" workflow that many developers appreciate with Heroku, while still being able to build large-scale distributed systems and event-driven applications deployed to AWS and GCP.

## Encore's local development workflow lets application developers focus

When using PaaS service like Heroku to deploy your application, you're not at all solving for an efficient local development workflow.

This means, with Heroku, developers need to manually set up and maintain their local environment and observability tools, in order to facilitate local development and testing.

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
- Automatic Infrastructure provisioning in AWS/GCP
- Infrastructure Tracking & Approvals workflow
- Cloud Configuration 2-way sync between Encore and AWS/GCP
- Automatic least privilege IAM
- Preview Environments per Pull Request
- Cost Analytics Dashboard
- Encore Terraform provider for extending Encore with infrastructure that is not currently part of Encore's Infrastructure SDK
