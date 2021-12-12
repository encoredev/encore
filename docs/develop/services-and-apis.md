---
title: Services and APIs
subtitle: Simplifying (micro-)service development
---

Encore divides applications into systems, services, and components.

## Defining a service

An Encore service consists of a regular Go package that defines **one or more APIs**.
The package name is the service name.

Within a service, you can have multiple packages; this is a good way to define components.
Note that only the **service package** can define APIs; sub-packages within a service
cannot themselves define APIs. However, if it makes sense you can define an API in the service package
that simply calls a function within one of the sub-packages.

## Defining APIs

So if what defines a service is having one or more APIs, how do we define an API, then? It's simple:

```go
package hello // service name

// PingParams is the request data for the Ping endpoint.
type PingParams struct {
    Name string
}

// PingResponse is the response data for the Ping endpoint.
type PingResponse struct {
    Message string
}

// Ping is an API endpoint that responds with a simple response.
// This is exposed as "hello.Ping".
//encore:api public
func Ping(ctx context.Context, params *PingParams) (*PingResponse, error) {
    msg := fmt.Sprintf("Hello, %s!", params.Name)
    return &PingResponse{Message: msg}, nil
}
```

In other words, you define a regular function and use the `//encore:api` annotation
to tell Encore that this is an API.

### Access controls

In fact, when you define an API you have three options in terms of how the API can be accessed:

* `//encore:api public` &ndash; defines a public API that anybody on the internet can call
* `//encore:api private` &ndash; defines a private API that only other backend services can call
* `//encore:api auth` &ndash; defines a public API that anybody can call, but that requires valid authentication.

For defining APIs that require authentication, see the [authentication guide](/docs/concepts/auth).

This approach is simple, but really powerful. It lets Encore use [static analysis](/docs/concepts/application-model)
to understand the request and response schemas of all your APIs, which enables it to automatically generate API documentation
and type-safe API clients, and much more.

### Request and response schemas

APIs in Encore consist of a regular function with a request data type and a response data type.
In the example above we have both: the request data is of type `PingParams` and the response data of type
`PingResponse`. This is usually the case, but in fact they're both optional in case you don't need them.
That means there are four different ways of defining an API:

* `func Foo(ctx context.Context, p *Params) (*Response, error)` &ndash; when you need both
* `func Foo(ctx context.Context) (*Response, error)` &ndash; when you only return a response
* `func Foo(ctx context.Context, p *Params) error` &ndash; when you only respond with success/fail
* `func Foo(ctx context.Context) error` &ndash; when you need neither request nor response data

As you can see there are two parts always present: the `ctx context.Context` parameter and the `error` return value.

The `ctx` parameter is used for *cancellation*. It lets you detect when the caller is no longer interested in the result,
and therefore lets you abort the request processing and save resources that nobody needs.
[Learn more about contexts on the Go blog](https://blog.golang.org/context).

The `error` return type is always required, because APIs can always fail from the caller's perspective.
Therefore even though our simple `Ping` API endpoint above never fails in its implementation, from the perspective of the caller perhaps the service is crashing or the network is down and the service cannot be reached.

### REST APIs
Encore comes with great, built-in support for RESTful APIs. It lets you easily define resource-oriented API URLs, parse parameters out of them, and more.

Start by defining an endpoint and specify the `method` and `path` fields in the `//encore:api` comment.

To specify a placeholder variable, use `:name` and add a function parameter with the same name to the function signature. Encore parses the incoming request URL and makes sure it matches the type of the parameter.

For example, if you want to have a `GetBlogPost` endpoint that takes a numeric id as a parameter:

```go
// GetBlogPost retrieves a blog post by id.
//encore:api public method=GET path=/blog/:id
func GetBlogPost(ctx context.Context, id int) (*BlogPost, error) {
    // Use id to query database...
}
```

You can also combine path parameters with body payloads. For example, if you want to have an `UpdateBlogPost` endpoint:

```go
// UpdateBlogPost updates an existing blog post by id.
//encore:api public method=PUT path=/blog/:id
func UpdateBlogPost(ctx context.Context, id int, post *BlogPost) error {
    // Use `post` to update the blog post with the given id.
}
```

<Callout type="important">
You will not be able to define paths that conflict with each other, including paths
where the static part can be mistaken for a parameter, e.g both `/blog` and `/blog/:id` would conflict with `/:username`.
</Callout>

As a rule of thumb, try to place path parameters at the end of the path and
prefix them with the service name, e.g:

```
GET /blog/posts
GET /blog/posts/:id
GET /user/profile/:username
GET /user/me
```

#### Query parameters
When fetching data with `GET` endpoints, it's common to receive additional parameters for optional behavior, like filtering a list or changing the sort order.

When you use a struct type as the last argument in the function signature,
Encore automatically parses these fields from the HTTP query string (for the `GET`, `HEAD`, and `DELETE` methods).

For example, if you want to have a `ListBlogPosts` endpoint:

```go
type ListParams struct {
    Limit uint // number of blog posts to return
    Offset uint // number of blog posts to skip, for pagination
}

type ListResponse struct {
    Posts []*BlogPost
}

//encore:api public method=GET path=/blog
func ListBlogPosts(ctx context.Context, opts *ListParams) (*ListResponse, error) {
    // Use limit and offset to query database...
}
```

This could then be queried as `/blog?limit=10&offset=20`.

Since query parameters are much more limited than structured JSON data, they can consist of basic types (`string`, `bool`, integer and floating point numbers, and `encore.dev/types/uuid.UUID`), as well as slices of those types.

## Calling an API
Calling an API endpoint with Encore looks like a regular function call. Import the service package as if it's a regular
Go package, and then call the API endpoint as if it's a regular function.

For example:
```go
import "app.encore.dev/myapp/hello" // import service

//encore:api public
func MyOtherAPI(ctx context.Context) error {
    resp, err := hello.Ping(ctx, &hello.PingParams{Name: "World"})
    if err == nil {
        log.Println(resp.Message) // "Hello, World!"
    }
    return err
}
```

When building your application, Encore uses [static analysis](/docs/concepts/application-model) to find all
API calls and compiles them to proper API calls. This provides all the benefits of function calls, like 
compile-time checking of all the parameters and auto-completion in your editor,
while still allowing the division of code into logical components, services, and systems.

## App Structure

As an example, a Trello app might consist of two systems: the **Trello** system (for managing trello boards & cards),
the **User** system (for user and organization management, and authentication), and the **Premium** system (for subscriptions
and paid features).

On disk it might look like this:

```go
/my-trello-clone
├── encore.app       // ... and other top-level project files
│
├── premium          // premium system
│   ├── payment      // payment service
│   └── subscription // subscription service
│
├── trello           // trello system
│   ├── board        // board service
│   └── card         // card service
│
└── usr              // user system
    ├── org          // org service
    └── user         // user service
```

Each service consists of Go files that implement the service business logic,
database migrations for defining the structure of the database(s), and so on.