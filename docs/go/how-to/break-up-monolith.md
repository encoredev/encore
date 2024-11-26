---
seotitle: Break a monolith into microservices
seodesc: Learn how to quickly break up your backend monolith into microservices using Encore, while avoiding the common pitfalls.
title: Break a monolith into microservices
subtitle: Evolving your architecture as needed
lang: go
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

Encore now understands these are separate services, and when you run your app you'll see that the [Service Catalog](/docs/go/observability/service-catalog) has been automatically updated accordingly.

<img src="/assets/docs/microservices-service-catalog.png" title="Service Catalog - Microservices" />

As well as the [Flow architecture diagram](/docs/go/observability/encore-flow).

<img src="/assets/docs/microservices-flow.png" title="Encore Flow - Microservices" />

## Sharing databases between services (or not)

Deciding whether to share a database between multiple services depends on your specific situation. Encore supports both options. Learn more in the [database documentation](/docs/go/primitives/share-db-between-services).

