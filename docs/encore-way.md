---
title: The Encore Way
subtitle: A developer experience designed to help you stay in the flow state
---

Encore helps you develop, deploy, and debug distributed systems and backend APIs faster than ever before.
With Encore you can focus on what makes your application unique, instead of spending time on "boilerplate" &mdash; the boring, repetitive work
you traditionally spend a lot of time on.

<img src="/assets/docs/encore-way.png" title="The Encore Way" className="noshadow"/>

## 1. Develop with the Encore Backend SDK

Setting up a productive development environment for building a modern backend application
is very time-consuming. Many different services need to be integrated; complex
configuration is required to connect all the pieces together; and there are many manual steps.

The Encore Backend SDK lets you get back to what matters &ndash; building your application.
With Encore you get a powerful way to write backend APIs using Go, that takes away the pain
of building production-ready backend applications:

- It simplifies managing backend services, creating APIs, calling other APIs, and so on.

- It generates API documentation and type-safe API clients for your frontend out of the box.

- It sets up and migrates your databases, handles connections and database passwords securely.

- It simplifies your workflow, getting you up and running in seconds.
  Setting everything up is as easy as `encore run`.

## 2. Collaborate effortlessly

As software quickly takes over the world, the need for a collaborative development process
becomes even greater. The Encore Platform provides a suite of collaboration tools that integrate
effortlessly with applications built with Encore, including:

- Each Pull Request automatically gets a dedicated Preview Environment,
  where you can easily verify the change &ndash; and test your frontend, too!

- Encore generates API Documentation for your application through its static analysis.
  It also generates type-safe API Clients for your frontend!

- Co-develop a frontend and backend in realtime by exposing your local machine
  as a bespoke environment using `encore run --tunnel`. Perfect for remote work.

## 3. Ship with confidence and speed

Instead of spending time setting up complicated build and deployment pipelines,
provisioning Kubernetes clusters, databases, and more; Encore takes care
of setting it all up for you.

You can either use the built in Encore Cloud and deploy in true serverless fashion, or connect your own cloud account with AWS or GCP and let Encore deploy there.

Encore applications automatically scale horizontally, can be run in one or multiple
regions, and even across multiple cloud providers.

### Debug production issues with ease

Encore comes with state of the art Distributed Tracing functionality, which dramatically
simplifies finding the source of production issues. Is a service slow to respond?
What data was being passed in to cause a particular bug to surface? Tracing makes
it easy to find the answer.
