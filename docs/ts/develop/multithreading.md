---
seotitle: Multithreading in Encore.ts
seodesc: See how Encore.ts provides true multithreading for JavaScript applications, and how to enable Worker Pooling for CPU-intensive workloads.
title: Multithreading
subtitle: True multithreading for JavaScript applications
lang: ts
---

Encore.ts runs using a high-performance Rust runtime that uses multiple threads to handle incoming requests.
The Encore.ts Rust runtime handles virtually everything outside of your core business logic:

- Parsing and validating incoming requests
- Making API calls to other services
- Serializing and writing API responses
- Observability integrations like distributed tracing
- Infrastructure integrations, like executing database queries, reading and writing from object storage, publishing and consuming messages from Pub/Sub, and more

This architecture allows for much higher performance and scalability compared to traditional JavaScript frameworks.
By offloading most of this to multithreaded Rust, the single-threaded JavaScript event loop becomes free to focus on executing your core business logic.

But for more CPU-intensive workloads, the single-threaded JavaScript event loop can still become a performance bottleneck.
For these use cases Encore.ts offers Worker Pooling. With Worker Pooling enabled, Encore.ts starts up multiple NodeJS event loops
and load-balances incoming requests across them. This can provide a significant performance boost for CPU-intensive workloads.

<img src="https://encore.dev/assets/blog/worker-pooling/encore-pooling.png" className="bg-black p-3 brand-shadow mx-auto" />

## Enabling Worker Pooling

To enable Worker Pooling, add `"build": {"worker_pooling": true}` to your `encore.app` file.

## Designing your application to work with Worker Pooling

Most application code will work with Worker Pooling without any changes. However, it's important to understand
the implications of running in a multi-threaded environment.

When utilizing Worker Pooling, Encore.ts will automatically spin up multiple NodeJS isolates (one per CPU) to handle incoming requests.
Each NodeJS isolate is a separate JavaScript runtime, with its own event loop and memory space.

This means that you cannot rely on global shared state that is shared across all incoming requests,
since each request may be handled by a different NodeJS isolate.
