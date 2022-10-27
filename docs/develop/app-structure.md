---
title: App Structure
subtitle: Structuring your Encore application
---

Encore uses a monorepo design and it's best to use one Encore app for your entire backend application.
This lets Encore build an application model that spans your entire app, necessary to get the most value out of many features like [distributed tracing](/docs/observability/tracing) and [Encore Flow](/docs/develop/encore-flow).

If you have a large application, see advice on how to [structure an app with several systems](/docs/develop/app-structure#large-applications-with-several-systems). 

It's easy to integrate Encore applications with any pre-existing systems you might have. To make this frictionless, Encore comes with built-in tools like [client generation](/docs/develop/client-generation).

## Monoliths and small microservices applications

Simple architectures like monoliths, or apps with a small number of services, are straightforward to structure.

Simply create a Go package for each service (also known as service packages), [define APIs](/docs/develop/services-and-apis) and business logic in Go files, and add database migrations to define the structure of any database(s). (See more about [SQL databases](/docs/develop/databases).)

On disk it might look like this:

```
/my-app
├── encore.app                       // ... and other top-level project files
│
├── hello                            // hello service (a Go package)
│   ├── migrations                   // hello service db migrations (directory)
│   │   └── 1_create_table.up.sql    // hello service db migration
│   ├── hello.go                     // hello service code
│   └── hello_test.go                // tests for hello service
│
└── world                            // world service (a Go package)
    └── world.go                     // world service code
```

Encore is not opinionated about whether you use a monolith or multiple services. However, it does solve most of the traditional drawbacks that come with building microservices.

## Large applications with several systems

If you have a large application with several logical domains, each consisting of multiple services, it can be practical to separate these into distinct systems. (Note: Systems have no technical meaning in Encore, they are simply a way of describing a logical grouping of services.)

To create systems, simply create a sub-directory for each system and put the relevant service packages within it.
This is all you need to do, since with Encore each service consists of a Go package.

As an example, a company building a Trello app might divide their application into three systems: the **Trello** system (for the end-user facing app with boards and cards), the **User** system (for user and organization management), and the **Premium** system (for handling payments and subscriptions).

On disk it might look like this:

```
/my-trello-clone
├── encore.app                  // ... and other top-level project files
│
├── trello                      // trello system (a directory)
│   ├── board                   // board service (a Go package)
│   │   └── board.go            // board service code
│   └── card                    // card service (a Go package)
│       └── card.go             // coard service code
│
├── premium                     // premium system (a directory)
│   ├── payment                 // payment service (a Go package)
│   │   └── payment.go          // payment service code
│   └── subscription            // subscription service (a Go package)
│       └── subscription.go     // subscription service code
│
└── usr                         // usr system (a directory)
    ├── org                     // org service (a Go package)
    │   └── org.go              // org service code
    └── user                    // user service (a Go package)
        └── user.go             // user service code
```

## Define components with sub-packages

Within a service, it's possible to have multiple sub-packages. This is a good way to define components, should you wish to do that.

To create sub-packages, you create sub-directories within a service package (which, again, is a regular Go package).

Note that only service packages can define APIs, and sub-packages within services cannot themselves define APIs.
You can however define an API in a service package that calls a function within a sub-package.

On disk it might look like this:

```
/my-app
├── encore.app                       // ... and other top-level project files
│
├── hello                            // hello service (a Go package)
│   ├── migrations                   // hello service db migrations (directory)
│   │   └── 1_create_table.up.sql    // hello service db migration
│   ├── foo                          // sub-package foo (directory)
│   │   └── foo.go                   // foo code (cannot define APIs)
│   ├── hello.go                     // hello service code
│   └── hello_test.go                // tests for hello service
│
└── world                            // world service (a Go package)
    └── world.go                     // world service code
```