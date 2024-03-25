---
seotitle: Structuring your microservices backend application
seodesc: Learn how to structure your microservices backend application. See recommended app structures for monoliths, small microservices backends, and large scale microservices applications.
title: App Structure
subtitle: Structuring your Encore application
lang: ts
---

Encore uses a monorepo design and it's best to use one Encore app for your entire backend application. This lets Encore build an application model that spans your entire app, necessary to get the most value out of many
features like [distributed tracing](/docs/observability/tracing) and [Encore Flow](/docs/develop/encore-flow).

If you have a large application, see advice on how to [structure an app with several systems](/docs/ts/develop/app-structure#large-applications-with-several-systems). 

It's simple to integrate Encore applications with pre-existing systems you might have, using APIs and built-in tools like [client generation](/docs/develop/client-generation). See more on how to approach building new functionality incrementally with Encore in the [migrating to Encore](/docs/how-to/migrate-to-encore) documentation.

## Monolith or Microservices

Encore is not opinionated about monoliths vs. microservices. It does however let you build microservices applications with a monolith-style developer experience. For example, you automatically get IDE auto-complete when making [API calls between services](/docs/ts/primitives/services-and-apis#calling-apis), along with cross-service type-safety.

When creating a cloud environment on AWS/GCP, Encore enables you to configure if you want to combine multiple services into one process or keep them separate. This can be useful for improved efficiency at smaller scales, and for co-locating services for increased performance. Learn more in the [environments documentation](/docs/deploy/environments#cloud-environments).

## Creating services

To create an Encore service, you create a folder and [define an API](/docs/develop/ts/services-and-apis) in a `.ts` file within it.

On disk it might look like this:

```
/my-app
├── encore.app          // ... and other top-level project files
├── package.json  
│
├── hello               // hello service (a folder)
│   ├── hello.ts        // hello service code
│   └── hello_test.ts   // tests for hello service
│
└── world               // world service (a folder)
    └── world.ts        // world service code
```

## Structure services using sub-modules

Within a service, it's possible to have multiple subdirectories. This is a good way to define components, helper
functions, or other code for your functions, should you wish to do that. You can create as many subdirectories, in any kind of nested structure within your service, as you want.

Note that currently all API endpoints must be defined in the top-level directory for the service.

For example, rather than define the entire logic for an endpoint in that endpoint's function, you can call functions
from sub-packages and divide the logic in any way you want.

On disk it might look like this:

```
/my-app
├── encore.app                       // ... and other top-level project files
│
├── hello                            // hello service (directory)
│   ├── migrations                   // hello service db migrations (directory)
│   │   └── 1_create_table.up.sql    // hello service db migration
│   ├── foo                          // sub-package foo (directory)
│   │   └── foo.ts                   // foo code (cannot define APIs)
│   ├── hello.ts                     // hello service code
│   └── hello_test.ts                // tests for hello service
│
└── world                            // world service (directory)
    └── world.ts                     // world service code
```

## Large applications with several systems

If you have a large application with several logical domains, each consisting of multiple services, it can be practical
to separate these into distinct systems.

Systems are not a special construct in Encore, they only help you divide your application logically around common concerns and purposes. Encore only handles services, the compiler will read your
systems and extract the services of your application. As applications grow, systems help you decompose your application
without requiring any complex refactoring.

To create systems, create a sub-directory for each system and put the relevant service packages within it.
This is all you need to do, since with Encore each service consists of a Go package.

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
│   │   └── board.ts            // board service code
│   └── card                    // card service (a directory)
│       └── card.ts             // card service code
│
├── premium                     // premium system (a directory)
│   ├── payment                 // payment service (a directory)
│   │   └── payment.ts          // payment service code
│   └── subscription            // subscription service (a directory)
│       └── subscription.ts     // subscription service code
│
└── usr                         // usr system (a directory)
    ├── org                     // org service (a directory)
    │   └── org.ts              // org service code
    └── user                    // user service (a directory)
        └── user.ts             // user service code
```

The only refactoring needed to divide an existing Encore application into systems is to move services into their respective
subfolders. This is a simple way to separate the specific concerns of each system. What matters for Encore are the packages containing services, and the division in systems or subsystems will not change the endpoints or
architecture of your application.
