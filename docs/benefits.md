---
title: Why Encore?
subtitle: Encore helps you build better software, faster
---

Building a modern, reliable backend application is a huge undertaking. Usually, this involves:

- Multiple backend services, each define their own API and call each other to orchestrate complex operations.

- Complex state management, often with several different databases, is not only hard to manage
  but equally hard to understand.

- A hacked-together DevOps process consisting of many different tools, poorly tested scripts,
  and a lack of consistency between different parts of the system make improvements difficult.

By adopting Encore, these challenges become much more manageable. Encore is specifically designed
for building cloud backend applications, and to make these use cases simpler. Creating a new service
with Encore is as easy as creating a new Go package, and calling another API is as easy as a function call.

As you define new services and databases, Encore analyzes your application code to build up a model
of exactly how it works and what infrastructure each component requires. When it comes to deploying
your application, Encore handles your whole DevOps pipeline and makes sure everything is consistently
provisioned and configured.

## A state of the art workflow

Encore is all about providing the best development workflow you can imagine.
Traditional software development processes are disjointed, consisting of many disparate tools
that barely fit together into a whole. Often accomplishing even the simplest tasks involves
lots of manual steps and glue work.

Encore reimagines what backend development should be like. A single command to run your whole
application. Access to all your infrastructure at your fingertips, regardless of the environment.
A much faster feedback loop through features like Hot Reload, tracing for local development,
automatic and always up-to-date [API documentation](/docs/develop/api-docs) and
[generated, type-safe API clients](/docs/develop/client-generation).

## From prototype to production

Many frameworks and tools designed for rapid development are great for prototyping, but become
a liability as your application grows. *Encore is different.* It's designed from the ground up to
be suitable for large scale software engineering. On top of this foundation are various features
designed to make it incredibly easy to get started. In this way, you get the benefits of incredibly
rapid prototyping, while your application gracefully handles increased requirements and scale
as you move to production and beyond.

## Collaborate without silos

Building modern applications is done in teams. At the same time our development tools haven't caught up.
We're still working on code in silos. Sharing our work happens only when everything is done, usually
in the form of a pull request.

Encore makes it easy to co-develop, working interactively on the frontend and backend.
Through static analysis it builds up a detailed Application Model that helps
non-technical people make sense of your application and enables features like a built-in
CMS that works directly on top of your data model.
