<div align="center">
  <a href="https://encore.dev" alt="encore"><img width="189px" src="https://encore.dev/assets/img/logo.svg"></a>
  <h3><a href="https://encore.dev">Encore – The Backend Development Engine</a></h3>
</div>
Encore makes it super easy to create backend services and APIs. Built ground up for Go, Encore uses static analysis and code generation to provide a revolutionary developer experience that is extremely productive.
<br/><br/>

**Get started in minutes and read the complete documentation: [encore.dev/docs](https://encore.dev/docs/quick-start)**

## Key features

* **[No Boilerplate](https://encore.dev/docs/develop/services-and-apis):** Set up a production ready backend application in minutes. Define services, API endpoints,
  and call APIs with a single line of Go code.

* **[Databases Made Simple](https://encore.dev/docs/concepts/databases):** Define the schema and then start querying. Encore takes care of provisioning, migrations, connections and passwords.

* **[Distributed Tracing](https://encore.dev/docs/observability/tracing):** Your application is automatically instrumented for excellent observability.
  Automatically capture information about API calls, goroutines, HTTP requests,
  database queries, and more. Works for both local development and production.

* **[Infrastructure Provisioning](https://encore.dev/docs/deploy/infra):** Encore understands how your application works,
  and provisions and manages your cloud infrastructure. Works with all the major cloud providers using your own account (AWS/Azure/GCP)
  and for local development.
  
* **[Preview Environments](https://encore.dev/docs/deploy/platform):** Every pull request becomes an isolated test environment. Collaborate and iterate faster than ever.
  
* **[Simple Secrets](https://encore.dev/docs/develop/secrets):** It's never been this easy to store and securely use secrets and API keys. Define secrets in your code like any other variable, Encore takes care of the rest.

* **[API Documentation](https://encore.dev/docs/develop/api-docs):** Encore parses your source code to understand the request/response
  schemas for all your APIs, and automatically generates high-quality, interactive
  API Documentation for you.
  
* **[Generate Frontend Clients](https://encore.dev/docs/how-to/integrate-frontend):**  Automatically generate type-safe, documented clients for your frontends.

## Getting started

To start using Encore, follow our simple [Quick Start Guide](https://encore.dev/docs/quick-start).


### Setup Demo
[![Setup demo](https://asciinema.org/a/406681.svg)](https://asciinema.org/a/406681)

### Database Demo
[![Setting up a database](https://asciinema.org/a/406695.svg)](https://asciinema.org/a/406695)

### API Documentation

[![API Documentation](https://encore.dev/assets/img/api-docs-screenshot.png)](https://encore.dev/docs/concepts/api-docs)

### Distributed Tracing

[![Automatic Tracing](https://encore.dev/assets/img/tracing.jpg)](https://encore.dev/docs/observability/tracing)

## Contributing to Encore and building from source

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Frequently Asked Questions (FAQ)

#### Who's behind Encore?

We're long-time Staff Engineers from Spotify, who grew frustrated with all the boilerplate and boring stuff you have to do to build modern cloud applications.

#### Why is the framework integrated with cloud hosting?

We've found that to meaningfully improve developer productivity you have to operate across the full stack. Unless you understand how an application is deployed, there are lots of things in the development process that you can't simplify. You can still use your own account with any of the major cloud providers (AWS/Azure/GCP), or you can use Encore's cloud for free, for Hobby projects, with pretty generous "fair use" limits. 

#### Can I use an existing Kubernetes cluster with Encore?

Not right now. We definitely want to support deploying to an existing k8s cluster, and enable more flexible deployment topologies in general. It's a bit tricky since we set up the cluster in a certain way, and it's hard to know how the existing cluster is configured and we don't want to break any existing application that might be running there.

#### Can you have it provision in Kubernetes rather than a cloud infrastructure?

Right now we only support deploying Encore apps to Kubernetes. Either where we host it for you (using AWS under the hood), or you can tell Encore to deploy to your own cloud account. In that case we currently set up a new Kubernetes cluster.

#### Does Encore support using websockets?

Encore supports dropping down to plain HTTP requests which lets you use Websockets.

## Get Involved
We rely on your contributions and feedback to improve Encore.
We love hearing about your experiences using Encore, and about what may be unclear and we can do a better job explaining.

* Send us feedback or ask questions via [email](mailto:hello@encore.dev).
* Connect with other Encore users on [Slack](https://encore.dev/slack).
* Follow us on [Twitter](https://twitter.com/encoredotdev).
* Leave feedback on our [Product Roadmap](https://encore.dev/roadmap).
* [Book a session](https://calendly.com/encoreandre/encore-office-hours) to speak with us directly.
