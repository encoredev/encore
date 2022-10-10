---
title: Tracing
---

When building a distributed system, or any backend really, it can be difficult to understand what your code is doing, or what’s going on in general. That’s where Tracing comes in. If you haven’t seen it before, it may just about change your life.

<video autoPlay playsInline loop controls muted className="w-full h-full">
	<source src="/assets/docs/dtracing.mp4" className="w-full h-full" type="video/mp4" />
</video>

Tracing is a revolutionary way to gain insight into what applications and distributed systems are doing, by capturing the series of events as they occur during the execution of your code (a “trace”). A trace id is propagated between all the systems so that when the trace information is sent off to a server for analysis, they can be correlated and joined together to present a unified picture of what happened.

Implementing tracing is a ton of work. It involves instrumenting each and every part of your application, propagating trace IDs. It also reduces performance so it’s not running on every request. It’s complicated to set up so you typically only do it for production.

In practice these downsides lead to tracing falling short of realizing its full potential as a revolutionary way to debug backend applications. No more — Encore solves all of them.

* Encore automatically traces your application, using the Encore Application Model and code generation to automatically instrument everything
* Encore’s tracing works for all environments — production, testing, and even local development!
* And unlike other tracing solutions, Encore understands what each trace event is, and captures unique insights about each one. Stack traces, structured logging, HTTP requests and network connection information, API calls, database queries, and more.
* The implementation is also done at a lower abstraction level, leveraging the Go runtime to do highly performant tracing. The end result is that the performance impact is much lower than other tracing implementations.

Traces are captured automatically and can be found through the [local development dashboard](./dev-dash) for local development, and in the [Encore web platform](https://app.encore.dev) for Production and other environments.