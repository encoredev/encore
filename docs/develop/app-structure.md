---
seotitle: Structuring your microservices backend application
seodesc: Learn how to structure your microservices backend application. See recommended app structures for monoliths, small microservices backends, and large scale microservices applications.
title: App Structure
subtitle: Structuring your Encore application
---

Encore uses a monorepo design and it's best to use one Encore app for your entire backend application.
This lets Encore build an application model that spans your entire app, necessary to get the most value out of many
features like [distributed tracing](/docs/observability/tracing) and [Encore Flow](/docs/develop/encore-flow).

If you have a large application, see advice on how to [structure an app with several systems](/docs/develop/app-structure#large-applications-with-several-systems). 

It's easy to integrate Encore applications with any pre-existing systems you might have. To make this frictionless,
Encore comes with built-in tools like [client generation](/docs/develop/client-generation).

## Monoliths and small microservices applications

Small architectures like monoliths, or apps with a small number of services, can be structured using one or many Encore
services. Services are the base building blocks of your application and divide it into isolated units
with their own endpoints and database(s). In a microservices application, an Encore service represents a
single microservice. While services are isolated and have their own databases by default,
[databases can also be shared between services](/docs/how-to/share-db-between-services).

To create an Encore service, create a Go package for each service (also known as service packages),
[define APIs](/docs/develop/services-and-apis) and business logic in Go files, and add database migrations to define the
structure of any database(s). (See more about [SQL databases](/docs/develop/databases).)

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

Encore is not opinionated about whether you use a monolith or multiple services. However, it does solve most of the
traditional drawbacks that come with building microservices.

## Structure services using sub-packages

Within a service, it's possible to have multiple sub-packages. This is a good way to define components, helper
functions, or other code for your functions, should you wish to do that. You can create as many sub-packages, in any kind of nested structure within your service, as you want.

To create sub-packages, you create sub-directories within a service package. Sub-packages are internal to services,
they are not themselves service packages. This means sub-packages within services cannot
themselves define APIs.
You can however define an API in a service package that calls a function within a sub-package.

For example, rather than define the entire logic for an endpoint in that endpoint's function, you can call functions
from sub-packages and divide the logic in any way you want.

**`hello/hello.go`**

```go
package hello

import (
	"context"
	
	"encore.app/hello/foo"
)

//encore:api public path=/hello/:name
func World(ctx context.Context, name string) (*Response, error) {
	msg := foo.GenerateMessage(name)
	return &Response{Message: msg}, nil
}

type Response struct {
    Message string
}
```

**`hello/foo/foo.go`**

```go
package foo

import (
	"fmt"
)

func GenerateMessage(name string) string {
	return fmt.Sprintf("Hello %s!", name)
}

```

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

## Large applications with several systems

If you have a large application with several logical domains, each consisting of multiple services, it can be practical
to separate these into distinct systems. Systems are not a special construct in Encore, they only help you divide your
application logically around common concerns and purposes. Encore only handles services, the compiler will read your
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

The only refactoring needed to divide an existing Encore application into systems is to move services into their respective
subfolders. This is a simple way to separate the specific concerns of each system. What
matters for Encore are the packages containing services, and the division in systems or subsystems will not change the endpoints or
architecture of your application.
