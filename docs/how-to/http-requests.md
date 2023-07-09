---
seotitle: How to receive regular HTTP requests in your backend application
seodesc: Learn how to receive regular HTTP requests in your Go based backend application using Encore.
title: Receive regular HTTP requests
subtitle: Dropping down in abstraction level
---

Encore makes it easy to define APIs and expose them, but it works best when you are in charge of the API schema.

Sometimes you need more control over the underlying HTTP request, such as to accept incoming webhooks from other
services, or to use WebSockets to stream data to/from the client.

For these use cases Encore lets you define **raw endpoints**. Raw endpoints operate at a lower abstraction level,
giving you access to the underlying HTTP request.

## Defining raw endpoints

To define a raw endpoint, change the `//encore:api` annotation and function signature like so:

```go
package service

import "net/http"

// Webhook receives incoming webhooks from Some Service That Sends Webhooks.
//encore:api public raw method=POST path=/webhook
func Webhook(w http.ResponseWriter, req *http.Request) {
    // ... operate on the raw HTTP request ...
}
```

If you're an experienced Go developer, this is just a regular Go HTTP handler.

See the <a href="https://pkg.go.dev/net/http#Handler" target="_blank" rel="nofollow">net/http documentation</a>
for more information on how Go HTTP handlers work.

## Reading path parameters

Sometimes webhooks have information in the path that you may be interested in retrieving or validating.

To do so, define the path with a path parameter, and then use [`encore.CurrentRequest`](https://pkg.go.dev/encore.dev#CurrentRequest)
to access the path parameters. For example:

```go
package service  
  
import (  
   "net/http"  
   
   "encore.dev"
 )

//encore:api public raw method=POST path=/webhook/:id
func Webhook(w http.ResponseWriter, req *http.Request) {  
    id := encore.CurrentRequest().PathParams().Get("id")
	// ... Do something with id
 }
```
