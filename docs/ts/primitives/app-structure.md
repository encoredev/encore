---
seotitle: Structuring your microservices backend application
seodesc: Learn how to structure your microservices backend application. See recommended app structures for monoliths, small microservices backends, and large scale microservices applications.
title: App Structure
subtitle: Structuring your Encore application
lang: ts
---

Encore uses a monorepo design and it's best to use one Encore app for your entire backend application. This lets Encore build an application model that spans your entire app, necessary to get the most value out of many
features like [distributed tracing](/docs/ts/observability/tracing) and [Encore Flow](/docs/ts/observability/flow).

If you have a large application, see advice on how to [structure an app with several systems](#large-applications-with-several-systems).

It's simple to integrate Encore applications with pre-existing systems you might have, using APIs and built-in tools like [client generation](/docs/ts/cli/client-generation). See more on how to approach building new functionality incrementally with Encore in the [migrating to Encore](/docs/platform/migration/migrate-to-encore) documentation.

## Monolith or Microservices

Encore is not opinionated about monoliths vs. microservices. It does however let you build microservices applications with a monolith-style developer experience. For example, you automatically get IDE auto-complete when making [API calls between services](/docs/ts/primitives/api-calls), along with cross-service type-safety.

When creating a cloud environment on AWS/GCP, Encore enables you to configure if you want to combine multiple services into one process or keep them separate. This can be useful for improved efficiency at smaller scales, and for co-locating services for increased performance. Learn more in the [environments documentation](/docs/platform/deploy/environments).

## Defining services

To create an Encore service, add a file named `encore.service.ts` in a directory.

The file must export a service instance, by calling `new Service`, imported from `encore.dev/service`.

For example:

```ts

import { Service } from "encore.dev/service";

export default new Service("my-service");
```

That's it! Encore will consider this directory and all its subdirectories as part of the service.

Within the service, you can then [define APIs](/docs/ts/primitives/defining-apis) and use infrastructure resources like querying databases.

<RelatedDocsLink paths={["/docs/ts/primitives/services", "/docs/ts/primitives/defining-apis"]} />

## Examples

Let's take a look at a few different approaches to structuring your Encore application, depending on the size and complexity of your application.

### Single-service application

The best place to start, especially if you're new to Encore, is by having
a single service in your application. Once you've familiarized yourself with
the Encore development model, it's easy to break it up into multiple services.

The best way to do this is by defining the `encore.service.ts` in the root
of your project, next to the `package.json` file.

On disk it might look like this (but feel free to change as you see fit):

```
/my-app
├── package.json
├── encore.app
├──  // ... other project files
│
├── encore.service.ts    // defines your service root
├── api.ts               // API endpoints
├── db.ts                // Database definition
```

Services can have subdirectories, so as the complexity of your service grows
you can add subdirectories as you see fit, to better organize the code base.

### Multi-service application (Distributed System)

For larger applications it's often useful to break it apart into multiple
services. This helps improve reliability, scalability, and lead to clearer
code organization.

Encore makes it easy to structure your application as multiple services.

Just like before, you add an `encore.service.ts` file to mark a directory
(and its subdirectories) as a service.


<Callout type="info">
Note that services cannot be nested: each must be defined in its own directory,
and cannot live in a subdirectory of another service.

If you have a single-service project with a `encore.service.ts` file at the top-level directory of your project, and you want to break it apart, start by moving that service code into a subdirectory.
</Callout>

On disk it might look like this:

```
/my-app
├── encore.app                       // ... and other top-level project files
│
├── hello                            // hello service (directory)
│   ├── migrations                   // hello service db migrations (directory)
│   │   └── 1_create_table.up.sql    // hello service db migration
│   ├── encore.service.ts            // hello service definition
│   ├── hello.ts                     // hello service APIs
│   └── hello_test.ts                // tests for hello service
│
└── world                            // world service (directory)
│   ├── encore.service.ts            // world service definition
    └── world.ts                     // world service APIs
```

### Large applications with several systems

If you have a large application with several logical domains, each consisting of multiple services, it can be practical
to separate these into distinct systems.

Systems are not a special construct in Encore, they only help you divide your application logically around common concerns and purposes. Encore only handles services, the compiler will read your
systems and extract the services of your application. As applications grow, systems help you decompose your application
without requiring any complex refactoring.

To create systems, simply create a sub-directory for each system and put the relevant service packages within it.

As an example, a company building a Trello app might divide their application into three systems: the **Trello** system
(for the end-user facing app with boards and cards), the **User** system (for user and organization management), and
the **Premium** system (for handling payments and subscriptions).

On disk it might look like this:

```
/my-trello-clone
├── encore.app
├── package.json                // ... and other top-level project files
│
├── trello                      // trello system (a directory)
│   ├── board                   // board service (a directory)
│   │   ├── encore.service.ts   // service definition
│   │   └── board.ts            // service code
│   │
│   └── card                    // card service (a directory)
│       ├── encore.service.ts   // service definition
│       └── card.ts             // service code
│
├── premium                     // premium system (a directory)
│   ├── payment                 // payment service (a directory)
│   │   ├── encore.service.ts   // service definition
│   │   └── payment.ts          // service code
│   │
│   └── subscription            // subscription service (a directory)
│       ├── encore.service.ts   // service definition
│       └── subscription.ts     // service code
│
└── usr                         // usr system (a directory)
    ├── org                     // org service (a directory)
    │   ├── encore.service.ts   // service definition
    │   └── org.ts              // service code
    │
    └── user                    // user service (a directory)
        ├── encore.service.ts   // service definition
        └── user.ts             // service code
```

The only refactoring needed to divide an existing Encore application into systems is to move services into their respective
subfolders. This is a simple way to separate the specific concerns of each system. What matters for Encore are the packages containing services, and the division in systems or subsystems will not change the endpoints or
architecture of your application.

## Package Management

For Encore.ts projects, using a single root-level `package.json` file (monorepo approach) is the recommended practice.
It has several benefits:

- Ensures consistent dependency versions across your services
- Simplifies TypeScript configuration management
- Makes it easier to share common types and utilities
- Reduces npm install overhead
- Works seamlessly with TypeScript's project references

Encore.ts also supports separate `package.json` files in sub-packages, with the following limitations:
- The Encore.ts application must use one package with a single `package.json` file
- Other separate packages must be pre-transpiled to JavaScript

Further package management options are planned for the future, particularly for supporting automatically transpiling and bundling workspace packages.
