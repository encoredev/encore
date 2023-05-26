<p align="center" dir="auto">
<a href="https://encore.dev"><img src="https://user-images.githubusercontent.com/78424526/214602214-52e0483a-b5fc-4d4c-b03e-0b7b23e012df.svg" width="160px" alt="encore icon"></img></a><br/><br/>
<b>Encore – Backend Development Platform</b><br/>
</p>
Encore is an end-to-end backend development platform that automatically provisions your application's infrastructure in your cloud account.
It's designed to help you build your product without platform distractions, removes boilerplate, and comes with built-in tools for observability and collaboration.
</br></br>
Start building today and unlock your creative potential, free from cloud complexity.
</br></br>

**🏁 Try Encore:** [Quick Start Guide](https://encore.dev/docs/quick-start) / [Example Applications](https://github.com/encoredev/examples/)

**⭐ Star this repository** to help spread the word.

**👋 Have questions?** Join the friendly [developer community](https://community.encore.dev), or say hello on [Slack](https://encore.dev/slack).

**📞 See if Encore fits your project:** [Book a 1:1 demo](https://encore.dev/book).
</br></br>

## How Encore works
![Encore Overview](https://user-images.githubusercontent.com/78424526/228284568-c1bbd16c-d71d-4178-bf74-cbcf1b450182.png)

Most backend applications use a small set of common building blocks: services, APIs, databases, queues, caches, and cron jobs. The real complexity lies in provisioning and managing the underlying infrastructure, and the endless boilerplate that's required to tie it all together in your application.

Encore simplifies backend development by providing an infrastructure SDK that lets you declare and use these common building blocks as objects directly in your code.

Encore parses your application and builds a graph of both its logical architecture and its infrastructure requirements. It then automatically provisions the necessary infrastructure, instruments your application with logs and traces, and much more.

It works the same way for local development, preview and test environments, and in production using your cloud account.

This completely removes the need for infrastructure configuration files, increases standardization in both your codebase and infrastructure, and makes your application portable across cloud providers by default.

**Learn more about using Encore's building blocks in the docs:**
- [Services and APIs](https://encore.dev/docs/primitives/services-and-apis)
- [Databases](https://encore.dev/docs/primitives/databases)
- [Cron Jobs](https://encore.dev/docs/primitives/cron-jobs)
- [PubSub](https://encore.dev/docs/primitives/pubsub)
- [Caching](https://encore.dev/docs/primitives/caching)

## Key features

- **[Microservices without complexity](https://encore.dev/docs/develop/services-and-apis):** Define services and API endpoints with a single line of Go code, and call APIs using regular function calls.

- **[Develop as if the infrastructure is already in place](https://encore.dev/docs/deploy/infra):** Encore's infrastructure SDK lets you define databases, queues, caches, and scheduled tasks, as objects in your code.

- **[Built-in DevOps](https://encore.dev/docs/introduction#a-developer-experience-designed-to-help-you-stay-in-the-flow):** Run `git push encore` to build, test, provision the necessary cloud infrastructure, and deploy.

- **[Provision environments automatically](https://encore.dev/docs/deploy/environments):** Deploy to unlimited dev/preview/cloud environments without code changes. Encore provisions the necessary infrastructure and ensures environments stay in sync.

- **[Preview environments](https://encore.dev/docs/how-to/github):** Integrate with GitHub to automatically set up each pull request as an ephemeral preview environment.

- **[Deploy to your existing cloud account](https://encore.dev/docs/deploy/infra):** Encore deploys your application to your existing cloud account in AWS/GCP/Azure, using best practices for building secure and scalable distributed systems.
 
- **[Intelligent architecture diagrams](https://encore.dev/docs/develop/encore-flow):** Visualise your application in with automated and interactive architecture diagrams.

- **[Distributed Tracing](https://encore.dev/docs/observability/tracing):** Your application is automatically instrumented to capture information about API calls, goroutines, HTTP requests, database queries, and more.

- **[Metrics](https://encore.dev/docs/observability/metrics):** Encore comes with built-in metrics and dashboards, and provides simple integrations with your favorite observability tools.
  
- **[Secrets management](https://encore.dev/docs/develop/secrets):** Securely store and use secrets, and API keys, without doing any work. Define secrets in your code, like any other variable, and Encore takes care of the rest.

- **[Automated API documentation](https://encore.dev/docs/develop/api-docs):** Encore parses your source code to understand the request/response
  schemas for all your APIs and automatically generates high-quality API Documentation.

- **[Generated Frontend Clients](https://encore.dev/docs/how-to/integrate-frontend):** Automatically generate type-safe API clients to integrate with your frontend.

## Why choose Encore?

- **Faster Development**: Encore streamlines the development process with its infrastructure SDK, clear abstractions, and built-in development tools, enabling you to build and deploy applications more quickly.
- **Reduced Costs**: Encore's infrastructure management minimizes wasteful cloud expenses and reduces DevOps workload, allowing you to work more efficiently.
- **Scalability & Performance**: Encore simplifies building microservices applications that can handle growing user bases and demands, without the normal boilerplate and complexity.
- **Control & Standardization**: Built-in tools like automated architecture diagrams, infrastructure overviews, and approval workflows make it easy for teams and leaders to get an overview of the entire application.
- **Security & Compliance**: Encore helps ensure your application and data is secure and compliant by enforcing standards and provisioning infrastructure according to best practises for each cloud provider.

## Common use cases

Encore is designed to give teams a productive and less complex experience when solving most backend use cases. Many teams use Encore to build things like:

-   High-performance B2B Platforms
-   Fintech & Consumer apps
-   Global E-commerce marketplaces
-   Microservices backends for SaaS applications and mobile apps
-   And much more...

## Getting started

- **1.** [Sign up and install the Encore CLI](https://encore.dev/signup)
- **2.** [Follow a tutorial and start building](https://encore.dev/docs/tutorials/)
- **3.** Follow and star the project on [GitHub](https://github.com/encoredev/encore) to stay up to date
- **4.** Explore the [Documentation](https://encore.dev/docs) to learn more about Encore's features
- **5.** [Book a 1:1](https://encore.dev/book) or [join Slack](https://encore.dev/slack) to discuss your use case or how to begin adopting Encore

## Open Source

The Encore infrastructure SDK, parser, compiler, and CLI are all Open Source.
A free Encore account is needed to unlock features like distributed tracing, secrets management, and deploying to cloud environments, as this functionality is orchestrated by the Encore Platform.

## Join the most pioneering developer community

Developers building with Encore are forward-thinkers who want to focus on creative programming and building great software to solve meaningful problems. It's a friendly place, great for exchanging ideas and learning new things! **Join the conversation on [Slack](https://encore.dev/slack).**

We rely on your contributions and feedback to improve Encore for everyone who is using it.
Here's how you can contribute:

- ⭐ **Star and watch this repository to help spread the word and stay up to date.**
- Share your ideas and ask questions on [Discourse](https://community.encore.dev/).
- Meet fellow Encore developers and chat on [Slack](https://encore.dev/slack).
- Follow Encore on [Twitter](https://twitter.com/encoredotdev).
- Share feedback or ask questions via [email](mailto:hello@encore.dev).
- Leave feedback on the [Public Roadmap](https://encore.dev/roadmap).
- Send a pull request here on GitHub with your contribution.

## Demo video

<a href="https://www.youtube.com/watch?v=IwplIbwJtD0" alt="Encore Demo Video" target="_blank">![demo video](https://user-images.githubusercontent.com/78424526/205661341-086c2813-455c-4af4-9517-b0398def6364.gif)</a>
</br>
<a href="https://www.youtube.com/watch?v=IwplIbwJtD0" alt="Encore Demo Video" target="_blank">Play full video on YouTube</a>

## Visuals

### Code example

https://github.com/encoredev/encore/assets/78424526/f511b3fe-751f-4bb8-a1da-6c9e0765ac08

### Local Development Dashboard

https://github.com/encoredev/encore/assets/78424526/565ab682-8c39-468e-8d21-c4d648b2dcb2

### Generated Architecture Diagrams & Service Catalog

https://github.com/encoredev/encore/assets/78424526/a880ed2d-e9a6-4add-b5a8-a4b44b97587b

### Auto-Provisioning Infrastructure & Multi-cloud Deployments

https://github.com/encoredev/encore/assets/78424526/8116b387-d4d4-4e54-8768-3686ba0245f5

### Distributed Tracing & Metrics

https://github.com/encoredev/encore/assets/78424526/35189335-e3d7-4046-bab0-1af0f00d2504

## Frequently Asked Questions (FAQ)

### Who's behind Encore?

Encore was founded by long-time backend engineers from Spotify, Google, and Monzo with over 50 years of collective experience. We’ve lived through the challenges of building complex distributed systems with thousands of services, and scaling to hundreds of millions of users.

Encore grew out of these experiences and is a solution to the frustrations that came with them: unnecessary crippling complexity and constant repetition of undifferentiated work that suffocates the developer’s creativity. With Encore, we want to set developers free to achieve their creative potential.

### Who is Encore for?

For individual developers building for the cloud, Encore provides a radically improved experience. With Encore you’re able to stay in the flow state and experience the joy and creativity of building.

For startup teams who need to build a scalable backend to support the growth of their product, Encore lets them get up and running in the cloud within minutes. It lets them focus on solving the needs of their users, instead of spending most of their time re-solving the everyday challenges of building distributed systems in the cloud.

For teams in mature organizations that want to focus on innovating and building new features, Encore lets them stop spending time on operations and onboarding new team members. Using Encore for new feature development is easy, just spin up a new backend service in a few minutes.

### How is Encore different?

Encore is the only tool that understands what you’re building. Encore uses static analysis to deeply understand the application you’re building. This enables a unique developer experience that helps you stay in the flow as you’re building. For instance, you don't need to bother with configuring and managing infrastructure, setting up environments and keeping them in sync, or writing documentation and drafting architecture diagrams. Encore does all of this automatically out of the box.

Unlike many tools that aim to only make cloud deployment easier, Encore is not a cloud hosting provider. With Encore, you can use your cloud account with all the major cloud providers: AWS/Azure/GCP. This means you’re in control of your data and can maintain your trust relationship with your cloud provider. You can also use Encore's development cloud for free, with pretty generous "fair use" limits.

### Why is the Encore SDK integrated with a cloud platform?

We've found that to meaningfully improve the developer experience, you have to operate across the full stack. Unless you understand how an application is deployed, there are a large number of things in the development process that you can't simplify. That's why so many other developer tools have such a limited impact. With Encore's more integrated approach, we're able to unlock a radically better experience for developers.

### What if I want to migrate away from Encore?

Encore has been designed to let you go outside of the SDK when you want to, and easily drop down in abstraction level when you need to. This means you're not likely to run into any "dead ends".

Should you want to migrate away, it's easy. As when you build an Encore application, the vast majority of code is just plain Go. So in practice, the amount of code specific to Encore is very small.

Encore has built-in support for [ejecting](https://encore.dev/docs/how-to/migrate-away#ejecting) your application as a way of removing the connection to the Encore Platform. Ejecting your app produces a standalone Docker image that can be deployed anywhere you'd like, and can help facilitate the migration away according to the process above.

Migrating away is low risk since Encore deploys to your cloud account from the start, which means there's never any data to migrate.

Open Source also plays a role. Encore's code generation, compiler, and parser are all open source and can be used however you want. So if you run into something unforeseen down the line, you have free access to the tools you might need.

And since Encore is designed for building distributed systems, it's straightforward to use it in combination with other backends that aren't built with Encore. So if you come across a use case where Encore for some reason doesn't fit, you won't need to tear everything up and start from scratch. You can just build that specific part without Encore.

We believe that adopting Encore is a low-risk decision, given it needs no initial investment in foundational work. The ambition is to simply add a lot of value to your everyday development process, from day one.

## Contributing to Encore and building from source

See [CONTRIBUTING.md](CONTRIBUTING.md).
