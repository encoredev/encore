---
seotitle: Encore Application Model
seodesc: How Encore understands your application using static analysis
title: Encore Application Model
subtitle: How Encore understands your application
lang: ts
---

Encore works by using static analysis to understand your application. This is a fancy term for parsing and analyzing the code you write and creating a graph of how your application works. This graph closely represents your own mental model of the system: boxes and arrows that represent systems and services that communicate with other systems, pass data and connect to infrastructure. We call it the Encore Application Model.

Because the Open Source framework, parser, and compiler, are all designed together, Encore can ensure 100% accuracy when creating the application model. Any deviation is caught as a compilation error.

Using this model, Encore can provide tools to solve problems that normally would be up to the developer to do manually. From creating architecture diagrams and API documentation to provisioning cloud infrastructure.

We're continuously expanding on Encore's capabilities and are building a new generation of developer tools that are enabled by Encore's understanding of your application.

The framework, parser, and compiler that enable this are all [Open Source](https://github.com/encoredev/encore).

<img src="/assets/docs/flow-diagram.png" title="Encore Application Model" className="mx-auto md:max-w-lg"/>

## Standardization brings clarity

Developers make dozens of decisions when creating a backend application. Deciding how to structure the codebase, defining API schemas, picking underlying infrastructure, etc. The decisions often come down to personal preferences, not technical rationale. This creates a huge problem in the form of fragmentation! When every stack looks different, all tools have to be general purpose.

When you adopt Encore, many of these stylistic decisions are already made for you. The Encore framework ensures your application follows modern best practices. And when you run your application, Encore's Open Source parser and compiler check that you're sticking to the standard. This means you're free to focus your energy on what matters: writing your application's business logic.