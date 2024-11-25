---
seotitle: Raw Endpoints
seodesc: Learn how to create raw API endpoints for your cloud backend application using Go and Encore.go
title: Raw Endpoints
subtitle: Drop down in abstraction to access the raw HTTP request
lang: go
---

Sometimes you need to operate a lower abstraction than Encore.go normally provides.
For example, you might want to access the underlying HTTP request, often useful for things like accepting webhooks.

Encore.go has you covered using "raw endpoints".

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

Like any other Encore API endpoint, once deployed this will be exposed at the URL: <br/>
`https://<env>-<app-id>.encr.app/service.Webhook`. Just like regular endpoints, raw endpoints support the use of `:id` and `*wildcard` segments.

Experienced Go developers will have already noted this is just a regular Go HTTP handler.
(See the <a href="https://pkg.go.dev/net/http#Handler" target="_blank" rel="nofollow">net/http documentation</a> for how Go HTTP handlers work.)

Learn more about receiving webhooks and using WebSockets in the [receiving regular HTTP requests guide](/docs/go/how-to/http-requests).

<GitHubLink 
    href="https://github.com/encoredev/examples/tree/main/slack-bot" 
    desc="Slack Bot example application that uses Raw endpoints to accept webhooks." 
/>
