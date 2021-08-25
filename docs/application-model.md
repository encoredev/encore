---
title: Encore Application Model
---

You might have gotten the impression that Encore is another backend framework. Spoiler alert: it’s not a backend framework at all! In reality it’s a completely new approach to building cloud-native backend applications that offers a radically improved developer experience. What sets Encore apart, and what makes it possible to deliver that experience, is what we call the **Encore Application Model**.

When you build an Encore backend, Encore makes heavy use of static analysis — a fancy term for parsing and analyzing the code you write — to build up a really detailed understanding of how your backend works. This understanding is in the shape of a graph, and very closely represents your own mental model about your system: boxes and arrows, representing systems and services that communicate with other systems, pass data and connect to infrastructure.

<img src="https://encore.dev/assets/blog/app-graph.png" title="Encore Application Model" className="mx-auto" width="300" />

This graph — the Encore Application Model — is the key to Encore’s revolutionary productivity. It’s what enables us to:

* [Provision and configure your cloud infrastructure](deploy/infra) for all the major cloud providers
* Generate all the boilerplate you’d normally have to write
* Automatically instrument your application with [Distributed Tracing](observability/tracing)
* Generate interactive [API documentation](develop/api-docs) that’s always up to date
* Provide an incredibly simple and idiomatic approach to [secrets management](develop/secrets)
* Reduce vendor lock-in by working at a higher abstraction level
* *And lots more!*

Every Encore feature we build is designed with the Encore Application Model in mind. As developers we’re not used to tools that actually understand what we’re doing. The height of intelligence has basically been “rename this function for me”. No longer.

With the Encore Application Model we can build an entirely new generation of tools that understand what we as developers are trying to do, and then help us do it. And that’s the key to developer productivity. And best of all? [It’s open source](https://github.com/encoredev/encore).