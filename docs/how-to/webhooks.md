---
title: Receive webhooks
subtitle: Call me maybe!
---

Encore makes it easy to define APIs and expose them, but it works best when you are in charge of the API schema.

When you want to accept webhooks – other services that make an API call when something happens on their side –
they define the API schema that you must fulfill. They also often require you to parse custom HTTP headers and do
other low-level things that Encore usually lets you skip.

For these circumstances Encore lets you define **Raw Endpoints**. Raw endpoints operate at a lower abstraction level,
giving you access to the underlying HTTP request.

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

Like any other Encore API endpoint, this will be exposed at the URL <br/>
`https://<app-id>.encoreapi.com/<env>/service.Webhook`.

If you're an experienced Go developer, this is just a regular Go HTTP handler.

See the <a href="https://pkg.go.dev/net/http#Handler" target="_blank" rel="nofollow">net/http documentation</a>
for more information on how Go HTTP handlers work.
