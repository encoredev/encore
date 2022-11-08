---
seotitle: Developing Services and APIs
seodesc: Learn how to create microservices and define APIs for your cloud backend application using Go and Encore. The easiest way of building cloud backends.
title: Services and APIs
subtitle: Simplifying (micro-)service development
---

Encore makes it easy to build applications with one or many services, without needing to manually handle the typical complexity of developing microservices.

## Defining a service

With Encore you define a service by [defining one or more APIs](#defining-apis) within a regular Go package. Encore recognizes this as a service, and uses the package name as the service name.

On disk it might look like this:

```
/my-app
├── encore.app          // ... and other top-level project files
│
├── hello               // hello service (a Go package)
│   ├── hello.go        // hello service code
│   └── hello_test.go   // tests for hello service
│
└── world               // world service (a Go package)
    └── world.go        // world service code
```


This means building a microservices architecture is as easy as creating multiple Go packages within your application.
See the [app structure documentation](/docs/develop/app-structure) for more details.

## Defining APIs

Defining an API is simple, you define a regular Go function and add the `//encore:api` annotation
to tell Encore that this is an API. In the example below, we define the API endpoint `Ping`, in the `hello` service, which gets exposed as `hello.Ping`.

```go
package hello // service name

//encore:api public
func Ping(ctx context.Context, params *PingParams) (*PingResponse, error) {
    msg := fmt.Sprintf("Hello, %s!", params.Name)
    return &PingResponse{Message: msg}, nil
}
```

### Request and response schemas

In the example above we defined an API that uses request and response schemas. The request data is of type `PingParams` and the response data of type `PingResponse`. That means we need to define them like so:

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

Request and response schemas are both optional in case you don't need them.
That means there are four different ways of defining an API:

* `func Foo(ctx context.Context, p *Params) (*Response, error)` &ndash; when you need both.
* `func Foo(ctx context.Context) (*Response, error)` &ndash; when you only return a response.
* `func Foo(ctx context.Context, p *Params) error` &ndash; when you only respond with success/fail.
* `func Foo(ctx context.Context) error` &ndash; when you need neither request nor response data.

As you can see, two parts are always present: the `ctx context.Context` parameter and the `error` return value.

The `ctx` parameter is used for *cancellation*. It lets you detect when the caller is no longer interested in the result,
and lets you abort the request processing and save resources that nobody needs.
[Learn more about contexts on the Go blog](https://blog.golang.org/context).

The `error` return type is always required because APIs can always fail from the caller's perspective.
Therefore even though our simple `Ping` API endpoint above never fails in its implementation, from the perspective of the caller perhaps the service is crashing or the network is down and the service cannot be reached.

This approach is simple but very powerful. It lets Encore use [static analysis](/docs/introduction#meet-the-encore-application-model)
to understand the request and response schemas of all your APIs, which enables Encore to automatically generate API documentation, type-safe API clients, and much more.

### Access controls

When you define an API, you have three options for how it can be accessed:

* `//encore:api public` &ndash; defines a public API that anybody on the internet can call.
* `//encore:api private` &ndash; defines a private API that only backend services in your app can call.
* `//encore:api auth` &ndash; defines a public API that anybody can call, but requires valid authentication.

For more on defining APIs that require authentication, see the [authentication guide](/docs/develop/auth).

### REST APIs
Encore has support for RESTful APIs and lets you easily define resource-oriented API URLs, parse parameters out of them, and more.

To create a REST API, start by defining an endpoint and specify the `method` and `path` fields in the `//encore:api` comment. (Learn more in the [API schemas guide](/docs/develop/api-schemas#path-parameters).)

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

You cannot define paths that conflict with each other, including paths
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

Query parameters are more limited than structured JSON data, and can only consist of basic types (`string`, `bool`, integer and floating point numbers), [Encore's UUID types](https://pkg.go.dev/encore.dev/types/uuid#UUID), and slices of those types.

### Raw endpoints

Encore lets you define raw endpoints that operate at a lower abstraction level.
This gives you access to the underlying HTTP request, which is useful for things like accepting webhooks.

To define a raw endpoint, change the `//encore:api` annotation and function signature like so:

```go
package service

import "net/http"

// Webhook receives incoming webhooks from Some Service That Sends Webhooks.
//encore:api public raw
func Webhook(w http.ResponseWriter, req *http.Request) {
    // ... operate on the raw HTTP request ...
}
```

Like any other Encore API endpoint, once deployed this will be exposed at the URL <br/>
`https://<env>-<app-id>.encr.app/service.Webhook`.

Experienced Go developers will have already noted this is just a regular Go HTTP handler.
(See the <a href="https://pkg.go.dev/net/http#Handler" target="_blank" rel="nofollow">net/http documentation</a> for how Go HTTP handlers work.)

You can read more about receiving webhooks in the [receive webhooks guide](/docs/how-to/webhooks).

## Calling an API
Calling an API endpoint with Encore looks like a regular function call. Import the service package as if it's a regular
Go package, using `import "encore.app/package-name"` and then call the API endpoint as if it's a regular function.

In the example below, we import the service package `hello` and call the `Ping` endpoint using a function call to `hello.Ping`.

```go
import "encore.app/hello" // import service

//encore:api public
func MyOtherAPI(ctx context.Context) error {
    resp, err := hello.Ping(ctx, &hello.PingParams{Name: "World"})
    if err == nil {
        log.Println(resp.Message) // "Hello, World!"
    }
    return err
}
```

This means your development workflow is as simple as building a monolith, even if you use multiple services.
You get all the benefits of function calls, like compile-time checking of all the parameters and auto-completion in your editor, while still allowing the division of code into logical components, services, and systems.

Then when building your application, Encore uses [static analysis](/docs/introduction#meet-the-encore-application-model) to parse all API calls and compiles them to proper API calls.

## Current Request

By using Encore's [current request API](https://pkg.go.dev/encore.dev/#Request) you can get meta-information about the
current request. Including the type of request, the time the request started, the service and endpoint called and the path
which was called on the service.

For more information, see the [metadata documentation](/docs/develop/metadata).

## Service Structs

You can also define a **service struct** which then enables you to define APIs as methods
on that service struct. This is primarily helpful for [dependency injection](/docs/how-to/dependency-injection).

It works by defining a struct type of your choice (typically called `Service`)
and declaring it with `//encore:service`.
Then, you can define a special function named `initService`
(or `initWhatever` if you named the type `Whatever`)
that gets called by Encore to initialize your service when it starts up.

It looks like this:
```go
//encore:service
type Service struct {
	// Add your dependencies here
}

func initService() (*Service, error) {
	// Write your service initialization code here.
}

//encore:api public
func (s *Service) MyAPI(ctx context.Context) error {
	// ...
}
```

### Calling APIs defined on service structs

When using a service struct like above, Encore will create a file named `encore.gen.go`
in your service directory. This file contains package-level functions for the APIs defined
as methods on the service struct. In the example above, you would see:

```go
// Code generated by encore. DO NOT EDIT.

package email

import "context"

// These functions are automatically generated and maintained by Encore
// to simplify calling them from other services, as they were implemented as methods.
// They are automatically updated by Encore whenever your API endpoints change.

func Send(ctx context.Context, p *SendParams) error {
	// The implementation is elided here, and generated at compile-time by Encore.
	return nil
}
```

These functions are generated in order to allow other services to keep calling your
APIs as package-level functions, in the same way as before: `email.Send(...)`.
This means other services do not need to care about whether you're using Dependency Injection
internally. You must always use these generated package-level functions for making API calls.

<Callout type="info">

Encore will automatically generate these files and keep them up to date
whenever your code changes. There is no need to manually invoke anything
to regenerate this code.

</Callout>

Encore adds all `encore.gen.go` files to your `.gitignore` since you typically
don't want to commit them to your repository; doing so ends up creating
a lot of unnecessary merge conflicts.

However, in some cases when running third-party linters in a CI/CD environment
it can be helpful to generate these wrappers to make the linter happy.
You can do that by invoking `encore gen wrappers`.

### Graceful Shutdown

When defining a service struct, Encore supports notifying
your service when it's time to gracefully shut down. This works
by having your service struct implement the method
`func (s *Service) Shutdown(force context.Context)`.

If that method exists, Encore will call it when it's time to begin
gracefully shutting down. Initially the shutdown is in "graceful mode",
which means that you have a few seconds to complete ongoing work.

The provided `force` context is canceled when the graceful shutdown window
is over, and it's time to forcefully shut down. How much time you have
from when `Shutdown` is called to when forceful shutdown begins depends on the
cloud provider and the underlying infrastructure. Typically it's in the range 5-30 seconds.

<Callout type="info">

Encore automatically handles graceful shutdown of all Encore-managed
functionality, such as HTTP servers, database connection pools,
Pub/Sub message receivers, distributed tracing recorders, and so on.

The graceful shutdown functionality is provided if you have additional,
non-Encore-related resources that need graceful shutdown.

</Callout>

Note that graceful shutdown in Encore is *cooperative*: Encore will wait indefinitely
for your `Shutdown` method to return. If your `Shutdown` method does not return promptly
after the `force` context is closed, the underlying infrastructure at your cloud provider
will typically force-kill your service, which can lead to lingering connections and other
such issues.

In summary, when your `Shutdown(force context.Context)` function is called:

- Immediately begin gracefully shutting down
- When the `force` context is canceled, you should forcefully shut down
  the resources that haven't yet completed their shutdown
- Wait until the shutdown is complete before returning from the `Shutdown` function
