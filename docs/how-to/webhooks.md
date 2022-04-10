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

Some times webhooks have information in the path that you may be interested in retrieving or validating. One example is a webhook which looks like this:
```
https://staging-testapp-1234.encr.app/id/12345
```
Perhaps you want to pull out the `12345` piece. You can do that with Encore as follows:

```go
package service  
  
import (  
   "net/http"  
   "strings"  
  
   "encore.dev/rlog"
 )

//encore:api public raw method=POST path=/id/:id
func Webhook(w http.ResponseWriter, req *http.Request) {  
   idSplit := strings.Split(req.URL.Path, "/id/")  
   
   if len(idSplit) < 2 {  
      rlog.Error("not enough path parts", "idSplit", idSplit)  
      w.WriteHeader(http.StatusBadRequest)  
      return  
  }  
   
   // value of id is 12345
   id := idSplit[1]
  ...
  // rest of the owl
  ...
 }
```
