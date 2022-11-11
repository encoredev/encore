---
seotitle: Encore Flow automatic microservices architecture diagrams
seodesc: Visualize your microservices architecture automatically using Encore Flow. Get real-time interactive architecture diagrams for your entire application.
title: Flow Architecture Diagram
subtitle: Visualize your cloud microservices architecture
---

Flow is a visual tool that gives you an up-to-date view of your entire system. Flow helps you improve your
microservices architecture by letting you instantly identify which services depend on each other and how they work
together.

## Birds-eye view

Having access to a zoomed out representation of your system can be invaluable in pretty much all parts of the
development cycle. Flow helps you:

* Track down bottlenecks before they grow into big problems.
* Get someone new to the team up onboarded and up to speed.
* Pinpoint hot paths in your system, services that might need extra attention.

Services and PubSub topics are represented as boxes, arrows indicate a dependency. In the example below
the `login` service has dependencies on the `user` and `authentication` services. Dashed arrows shows publications or
subscriptions to a topic. Here, `payment` publishes to the `payment-made` topic and `email` subscribe to it:


<img src="/assets/docs/flow-diagram.png" title="Encore Flow - Highlight Dependencies" />

## Highlight dependencies

Hover over a service, or PubSub topic, to instantly reveal the nature and scale of its dependencies.

Here the `login` service and its dependencies are highlighted. We can see that `login` makes queries to the
database and requests to two of the endpoints from the `user` service as well as requests to one endpoint from
the `authentication` service:

<img src="/assets/docs/flow-highlight.png" title="Encore Flow - Highlight Dependencies" />

## Auto update

Flow is available in the [Development Dashboard](/docs/observability/dev-dash) which is accessible after
running `encore run` in your terminal. From there, Flow will auto update in real-time to reflect your architecture as you
develop your application. This helps you be mindful of important dependencies and make it clear if you introduce new ones.

In the example below a new subscription on the topic `payment-made` is introduced and then removed in `user` service:

<video autoPlay playsInline loop controls muted className="w-full h-full">
	<source src="/assets/docs/flow-auto-update.mp4" className="w-full h-full" type="video/mp4" />
</video>
