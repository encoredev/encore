---
seotitle: Defining type-safe APIs with Encore.go
seodesc: Learn how to create APIs for your cloud backend application using Go and Encore.go
title: Defining Type-Safe APIs
subtitle: Simplifying type-safe API development
lang: go
---

Encore.go enables you to create type-safe APIs from regular Go functions.

To define an API, add the `//encore:api` annotation to a function in your code.
This tells Encore that the function is an API endpoint and Encore will automatically generate the necessary boilerplate at compile-time.

In the example below, we define the API endpoint `Ping`, in the `hello` service, which gets exposed as `hello.Ping`.

```go
package hello // service name

//encore:api public
func Ping(ctx context.Context, params *PingParams) (*PingResponse, error) {
    msg := fmt.Sprintf("Hello, %s!", params.Name)
    return &PingResponse{Message: msg}, nil
}
```

<GitHubLink 
    href="https://github.com/encoredev/examples/tree/main/hello-world" 
    desc="Hello World REST API example application." 
/>

## Access controls

When you define an API, you have three options for how it can be accessed:

* `//encore:api public` &ndash; defines a public API that anybody on the internet can call.
* `//encore:api private` &ndash; defines a private API that is never accessible to the outside world. It can only be called from other services in your app and via cron jobs.
* `//encore:api auth` &ndash; defines a public API that anybody can call, but requires valid authentication.

You can optionally send in auth data to `public` and `private` APIs, in which case the auth handler will be used. When used for `private` APIs, they are still not accessible from the outside world.

For more on defining APIs that require authentication, see the [authentication guide](/docs/go/develop/auth).

## API Schemas

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
Request and response schemas are both optional. There are four different ways of defining an API:

**Using both request and response data:**<br/>
`func Foo(ctx context.Context, p *Params) (*Response, error)`

**Only returning a response:**<br/>
`func Foo(ctx context.Context) (*Response, error)`

**With only request data:**<br/>
`func Foo(ctx context.Context, p *Params) error`

**Without any request or response data:**<br/>
`func Foo(ctx context.Context) error`

As you can see, two parts are always present: the `ctx context.Context` parameter and the `error` return value.

The `ctx` parameter is used for *cancellation*. It lets you detect when the caller is no longer interested in the result,
and lets you abort the request processing and save resources that nobody needs.
[Learn more about contexts on the Go blog](https://blog.golang.org/context).

The `error` return type is always required because APIs can always fail from the caller's perspective.
Therefore even though our simple `Ping` API endpoint above never fails in its implementation, from the perspective of the caller perhaps the service is crashing or the network is down and the service cannot be reached.

This approach is simple but very powerful. It lets Encore use [static analysis](/docs/go/concepts/application-model)
to understand the request and response schemas of all your APIs, which enables Encore to automatically generate API documentation, type-safe API clients, and much more.

### Request and response data types

Request and response data types are structs (or pointers to structs) with optional field tags, which Encore uses to encode API requests to HTTP messages. The same struct can be used for requests and responses, but the `query` tag is ignored when generating responses.

All tags except `json` are ignored for nested tags, which means you can only define `header` and `query` parameters for root level fields.

For example, this struct:
```go
type NestedRequestResponse struct {
	Header string `header:"X-Header"`// this field will be read from the http header
	Query  string `query:"query"`// this field will be read from the query string
	Body1  string `json:"body1"`
	Nested struct {
	    Header2 string `header:"X-Header2"`// this field will be read from the body
		Query2  string `query:"query2"`// this field will be read from the body
		Body2   string `json:"body2"`
    } `json:"nested"`
}
```

Would be unmarshalled from this request:

```output
POST /example?query=a%20query HTTP/1.1
Content-Type: application/json
X-Header: A header

{
   "body1": "a body",
   "nested": {
      "Header2": "not a header",
      "Query2": "not a query",
      "body2": "a nested body"
   }
}

```

And marshalled to this response:

```output
HTTP/1.1 200 OK
Content-Type: application/json
X-Header: A header

{
   "Query": "not a query",
   "body1": "a body",
   "nested": {
      "Header2": "not a header",
      "Query2": "not a query",
      "body2": "a nested body"
   }
}

```

### Path parameters

Path parameters are specified by the `path` field in the `//encore:api` annotation.
To specify a placeholder variable, use `:name` and add a function parameter with the same name to the function signature.
Encore parses the incoming request URL and makes sure it matches the type of the parameter. The last segment of the path
can be parsed as a wildcard parameter by using `*name` with a matching function parameter.

```go
// GetBlogPost retrieves a blog post by id.
//encore:api public method=GET path=/blog/:id/*path
func GetBlogPost(ctx context.Context, id int, path string) (*BlogPost, error) {
    // Use id to query database...
}
```

### Fallback routes

Encore supports defining fallback routes that will be called if no other endpoint matches the request,
using the syntax `path=/!fallback`.

This is often useful when migrating an existing backend service over to Encore, as it allows you to gradually
migrate endpoints over to Encore while routing the remaining endpoints to the existing HTTP router using
a raw endpoint with a fallback route.

For example:

```go
//encore:service
type Service struct {
	oldRouter *gin.Engine // existing HTTP router
}

// Route all requests to the existing HTTP router if no other endpoint matches.
//encore:api public raw path=/!fallback
func (s *Service) Fallback(w http.ResponseWriter, req *http.Request) {
    s.oldRouter.ServeHTTP(w, req)
}
```

### Headers

Headers are defined by the `header` field tag, which can be used in both request and response data types. The tag name is used to translate between the struct field and http headers.
In the example below, the `Language` field of `ListBlogPost` will be fetched from the
`Accept-Language` HTTP header.

```go
type ListBlogPost struct {
    Language string `header:"Accept-Language"`
    Author      string // Not a header
}
```

### Cookies

Cookies can be set in the response by using the `header` tag with the `Set-Cookie` header name.

```go
type LoginResponse struct {
    SessionID string `header:"Set-Cookie"`
}

//encore:api public method=POST path=/login
func Login(ctx context.Context) (*LoginResponse, error) {
    return &LoginResponse{SessionID: "session=123"}, nil
}
````

The cookies can then be read using e.g. [structured auth data](/docs/go/develop/auth#accepting-structured-auth-information). 

### Query parameters

For `GET`, `HEAD` and `DELETE` requests, parameters are read from the query string by default.
The query parameter name defaults to the [snake-case](https://en.wikipedia.org/wiki/Snake_case)
encoded name of the corresponding struct field (e.g. BlogPost becomes blog_post).

The `query` field tag can be used to parse a field from the query string for other HTTP methods (e.g. POST) and to override the default parameter name. 

Query strings are not supported in HTTP responses and therefore `query` tags in response types are ignored.

In the example below, the `PageLimit` field will be read from the `limit` query
parameter, whereas the `Author` field will be parsed from the query string (as `author`) only if the method of
the request is `GET`, `HEAD` or `DELETE`.

```go
type ListBlogPost struct {
    PageLimit  int `query:"limit"` // always a query parameter
    Author     string              // query if GET, HEAD or DELETE, otherwise body parameter
}
```

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


### Body parameters

Encore will default to reading request parameters from the body (as JSON) for all HTTP methods except `GET`, `HEAD` or
`DELETE`. The name of the body parameter defaults to the field name, but can be overridden by the
`json` tag. Response fields will be serialized as JSON in the HTTP body unless the `header` tag is set.

There is no tag to force a field to be read from the body, as some infrastructure entities
do not support body content in `GET`, `HEAD` or `DELETE` requests.

```go
type CreateBlogPost struct {
    Subject    string `json:"limit"` // query if GET, HEAD or DELETE, otherwise body parameter
    Author     string                // query if GET, HEAD or DELETE, otherwise body parameter
}
```

### Supported types
The table below lists the data types supported by each HTTP message location.

| Type            | Header | Path | Query | Body |
| --------------- | ------ | ---- | ----- | ---- |
| bool            | X      | X    | X     | X    |
| numeric         | X      | X    | X     | X    |
| string          | X      | X    | X     | X    |
| time.Time       | X      | X    | X     | X    |
| uuid.UUID       | X      | X    | X     | X    |
| json.RawMessage | X      | X    | X     | X    |
| list            |        |      | X     | X    |
| struct          |        |      |       | X    |
| map             |        |      |       | X    |
| pointer         |        |      |       | X    |


## Sensitive data

Encore.go comes with built-in tracing functionality that automatically captures request and response payloads
to simplify debugging. While helpful, that's not always desirable. For instance when a request or response payload contains sensitive data, such
as API keys or personally identifiable information (PII).

For those use cases Encore supports marking a field as sensitive using the struct tag `encore:"sensitive"`.
Encore's tracing system will automatically redact fields tagged as sensitive. This works for both individual
values as well as nested fields.

Note that inputs to [auth handlers](/docs/go/develop/auth) are automatically marked as sensitive and are always redacted.

Raw endpoints lack a schema, which means there's no way to add a struct tag to mark certain data as sensitive.
For this reason Encore supports tagging the whole API endpoint as sensitive by adding `sensitive` to the `//encore:api` annotation.
This will cause the whole request and response payload to be redacted, including all request and response headers.

<Callout type="info">

The `encore:"sensitive"` tag is ignored for local development environments to make development and debugging with the Local Development Dashboard easier.

</Callout>


### Example

```go
package blog // service name
import (
	"time"
	"encore.dev/types/uuid"
)

type Updates struct {
	Author      string `json:"author,omitempty"`
	PublishTime time.Time `json:"publish_time,omitempty"`
}

// BatchUpdateParams is the request data for the BatchUpdate endpoint.
type BatchUpdateParams struct {
	Requester     string    `header:"X-Requester"`
	RequestTime   time.Time `header:"X-Request-Time"`
	CurrentAuthor string    `query:"author"`
	Updates       *Updates  `json:"updates"`
	MySecretKey   string    `encore:"sensitive"`
}

// BatchUpdateResponse is the response data for the BatchUpdate endpoint.
type BatchUpdateResponse struct {
	ServedBy   string       `header:"X-Served-By"`
	UpdatedIDs []uuid.UUID  `json:"updated_ids"`
}

//encore:api public method=POST path=/section/:sectionID/posts
func BatchUpdate(ctx context.Context, sectionID string, params *BatchUpdateParams) (*BatchUpdateResponse, error) {
	// Update blog posts for section
	return &BatchUpdateResponse{ServedBy: hostname, UpdatedIDs: ids}, nil
}

```

## REST APIs
Encore has support for RESTful APIs and lets you easily define resource-oriented API URLs, parse parameters out of them, and more.

To create a REST API, start by defining an endpoint and specify the `method` and `path` fields in the `//encore:api` comment.

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
