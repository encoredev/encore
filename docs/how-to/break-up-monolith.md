---
seotitle: Break a monolith into microservices
seodesc: Learn how to quickly break up your backend monolith into microservices using Encore, while avoiding the common pitfalls.
title: Break a monolith into microservices
subtitle: Evolving your architecture as needed
---

It's common to want to break out specific functionality into separate services. Perhaps you want to independently scale a specific service, or simply want to structure your codebase in smaller pieces.

Encore makes it simple to evolve your system architecture over time, and enables you to deploy your application in multiple different ways without making code changes.

## How to break out a service from a monolith

As a (slightly silly) example, let's imagine we have a monolith `hello` with two API endpoints `H1` and `H2`. It looks like this:

```go
package hello

import (
	"context"
)

//encore:api public path=/hello/:name
func H1(ctx context.Context, name string) (*Response, error) {
	msg := "Hello, " + name + "!"
	return &Response{Message: msg}, nil
}

//encore:api public path=/yo/:name
func H2(ctx context.Context, name string) (*Response, error) {
	msg := "Yo, " + name + "!"
	return &Response{Message: msg}, nil
}

type Response struct {
	Message string
}
```

Now we're going to break out `H2` into its own separate service. Happily, all we need to do is create a new package, let's call it `yo`, and move the `H2` endpoint into it.

Like so:
```go
package yo

import (
	"context"
)

//encore:api public path=/yo/:name
func H2(ctx context.Context, name string) (*Response, error) {
	msg := "Yo, " + name + "!"
	return &Response{Message: msg}, nil
}

type Response struct {
	Message string
}
```

On disk we now have:
```
/my-app
├── encore.app        // ... and other top-level project files
│
├── hello             // hello service (a Go package)
│   └── hello.go      // hello service code
│
└── yo                // yo service (a Go package)
    └── yo.go         // yo service code
```

Encore now understands these are separate services, and when you run your app you'll see that the [Service Catalog](/docs/develop/api-docs) has been automatically updated accordingly.

<img src="/assets/docs/microservices-service-catalog.png" title="Service Catalog - Microservices" />

As well as the [Flow architecture diagram](/docs/observability/encore-flow).

<img src="/assets/docs/microservices-flow.png" title="Encore Flow - Microservices" />

## Microservices process allocation

Just because you want to deploy each service separately in some environments, doesn't mean you want to do it in _all_ environments.

Handily, Encore lets you decide how you want to deploy your services for _each_ environment. You don't need to change a single line of code.

When you [create an environment](/docs/deploy/environments), you can simply decide which process allocation you want for that environment.

<img src="/assets/docs/microservices-process-allocation.png" title="Microservices - Process Allocation" />

## Sharing databases between services (or not)

Deciding whether to share a database between multiple services depends on your specific situation. Encore supports both options. Learn more in the [database documentation](/docs/how-to/share-db-between-services).

