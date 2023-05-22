---
title: Building a GraphQL API
subtitle: Learn how to build a GraphQL API using Go and Encore
seotitle: How to build a GraphQL API using Go and Encore
seodesc: Learn how to build a microservices backend in Go, powered by GraphQL and Encore.
---

Encore has great support for GraphQL with its type-safe approach to building APIs.

The best way to use GraphQL with Encore is using [gqlgen](https://gqlgen.com/), which
has similar goals as Encore (type-safe APIs, minimal boilerplate, code generation, etc).

Encore's automatic tracing also makes it easy to find and fix
performance issues that often arise in GraphQL APIs (like the [N+1 problem](https://hygraph.com/blog/graphql-n-1-problem)).

The final code will look like this:

<div className="not-prose my-10">
   <Editor projectName="graphql" />
</div>

## 1. Create your Encore application

This tutorial uses the [REST API](/docs/tutorials/rest-api) tutorial as a starting point.

You can either follow that tutorial first, or you can create a new Encore application
using the `url-shortener` template by running:

```shell
$ encore app create --example=url-shortener
```

## 2. Initialize gqlgen

To get started, initialize gqlgen by creating a `tools.go` file in the application root:

```go
-- tools.go --
//go:build tools

package tools

import (
    _ "github.com/99designs/gqlgen"
    _ "github.com/99designs/gqlgen/graphql/introspection"
)
```

Then run `go mod tidy` to download the dependencies.

Next, create a `gqlgen.yml` file in the application root containing:

```
-- gqlgen.yml --
# Where are all the schema files located? globs are supported eg  src/**/*.graphqls
schema:
  - graphql/*.graphqls

# Where should the generated server code go?
exec:
  filename: graphql/generated/generated.go
  package: generated

# Where should any generated models go?
model:
  filename: graphql/model/models_gen.go
  package: model

# Where should the resolver implementations go?
resolver:
  layout: follow-schema
  dir: graphql
  package: graphql

# gqlgen will search for any type names in the schema in these go packages
# if they match it will use them, otherwise it will generate them.
autobind:
 - "encore.app/url"

# This section declares type mapping between the GraphQL and go type systems
#
# The first line in each type will be used as defaults for resolver arguments and
# modelgen, the others will be allowed when binding to fields. Configure them to
# your liking
models:
  ID:
    model:
      - github.com/99designs/gqlgen/graphql.ID
      - github.com/99designs/gqlgen/graphql.Int
      - github.com/99designs/gqlgen/graphql.Int64
      - github.com/99designs/gqlgen/graphql.Int32
  Int:
    model:
      - github.com/99designs/gqlgen/graphql.Int
      - github.com/99designs/gqlgen/graphql.Int64
      - github.com/99designs/gqlgen/graphql.Int32
```

## 3. Create Encore service

Now it's time to create our Encore service that will provide the GraphQL API.

First generate the gqlgen boilerplate:

```shell
$ mkdir -p graphql/generated graphql/model
$ echo "package model" > graphql/model/model.go
$ go run github.com/99designs/gqlgen generate
```

This will create a bunch of files in the `graphql` directory.

Next, create a `graphql/service.go` file containing:

```go
-- graphql/service.go --
package graphql

import (
    "net/http"

    "encore.app/graphql/generated"
    "github.com/99designs/gqlgen/graphql/handler"
    "github.com/99designs/gqlgen/graphql/playground"
)

//go:generate go run github.com/99designs/gqlgen generate

//encore:service
type Service struct {
    srv        *handler.Server
    playground http.Handler
}

func initService() (*Service, error) {
    srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: &Resolver{}}))
    pg := playground.Handler("GraphQL Playground", "/graphql")
    return &Service{srv: srv, playground: pg}, nil
}

//encore:api public raw path=/graphql
func (s *Service) Query(w http.ResponseWriter, req *http.Request) {
    s.srv.ServeHTTP(w, req)
}

//encore:api private raw path=/graphql/playground
func (s *Service) Playground(w http.ResponseWriter, req *http.Request) {
    s.playground.ServeHTTP(w, req)
}
```

This creates an Encore service that exposes the `/graphql` and `/graphql/playground` endpoints.
The playground is a private route so it can be accessed from the browser when developing locally, but not in production.

It also adds a `//go:generate` directive that lets you re-run the gqlgen code generation
by running `go generate ./graphql`.

## 4. Add GraphQL schema

Now it's time to define the GraphQL schema. Create a `graphql/schema.graphqls` file containing:

```
-- graphql/url.graphqls --
type Query {
  urls: [URL!]!
  get(id: ID!): URL!
}

type Mutation {
  shorten(input: String!): URL!
}

type URL {
  id:  ID!     # shortened id
  url: String! # full URL
}
```

Then, re-run the code generation to generate the resolver stubs:

```shell
$ go generate ./graphql
```
The stubs will be written to `graphql/url.resolvers.go` and will contain a bunch of unimplemented resolver methods
that look something like this:

```go
// Shorten is the resolver for the shorten field.
func (r *mutationResolver) Shorten(ctx context.Context, input string) (*url.URL, error) {
	panic(fmt.Errorf("not implemented: Shorten - shorten"))
}
```

## 5. Implement resolvers

Now, modify the resolvers to call the `url` service. Since the GraphQL API uses the same types
(thanks to the `autobind` directive in `gqlgen.yml`)  as the Encore API exposes we can just call the
endpoints directly. Implement the resolvers in `graphql/url.resolvers.go` like this:

```go
-- graphql/url.resolvers.go --
// Shorten is the resolver for the shorten field.
func (r *mutationResolver) Shorten(ctx context.Context, input string) (*url.URL, error) {
	return url.Shorten(ctx, &url.ShortenParams{URL: input})
}

// Urls is the resolver for the urls field.
func (r *queryResolver) Urls(ctx context.Context) ([]*url.URL, error) {
	resp, err := url.List(ctx)
	if err != nil {
		return nil, err
	}
	return resp.URLs, nil
}

// Get is the resolver for the get field.
func (r *queryResolver) Get(ctx context.Context, id string) (*url.URL, error) {
	return url.Get(ctx, id)
}
```

As you can see, the resolvers are just thin wrappers around the Encore API endpoints themselves.

## 6. Trying it out

With that, the GraphQL API is done! Try it out by running `encore run` and opening up [the playground](http://localhost:4000/graphql/playground).

Enter the query:
```graphql
mutation {
    shorten(input: "https://encore.dev") {
        id
    }
}
```

You should get back an id like `MnTWA8Jo`. Pass the id you got (it will be something different) to a `get` query:

```graphql
query {
    get(id: "<your-id-here>") {
        url
    }
}
```

And you should get back `https://encore.dev`. 

## Conclusion

We've now built a GraphQL API gateway to our application, that forwards requests to the
underlying Encore services in a type-safe way with minimal boilerplate.

Note that the concepts discussed here are general and can be easily applied to any GraphQL API.
Whenever you make a change to the schema or configuration, re-run `go generate ./graphql` to
regenerate the GraphQL boilerplate. And for more information on how to use `gqlgen`,
see the [gqlgen documentation](https://gqlgen.com/).
