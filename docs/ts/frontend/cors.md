---
seotitle: Handling CORS (Cross-Origin Resource Sharing)
seodesc: See how you can configure CORS for your Encore application.
title: CORS
subtitle: Configure CORS (Cross-Origin Resource Sharing) for your Encore application
lang: ts
---

CORS is a web security concept that defines which website origins are allowed to access your API.

A deep-dive into CORS is out of scope for this documentation, but [MDN](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS)
provides a good overview. In short, CORS affects requests made by browsers to resources hosted on
other origins (a combination of the scheme, domain, and port).

## Configuring CORS

Encore provides a default CORS configuration that is suitable for many APIs. You can override these settings
by specifying the `global_cors` key in the `encore.app` file, which has the following
structure:

```cue
{
    // debug enables CORS debug logging.
    "debug": true | false,

    // allow_headers allows an app to specify additional headers that should be
    // accepted by the app.
    //
    // If the list contains "*", then all headers are allowed.
    "allow_headers": [...string],

    // expose_headers allows an app to specify additional headers that should be
    // exposed from the app, beyond the default set always recognized by Encore.
    //
    // If the list contains "*", then all headers are exposed.
    "expose_headers": [...string],

    // allow_origins_without_credentials specifies the allowed origins for requests
    // that don't include credentials. If nil it defaults to allowing all domains
    // (equivalent to ["*"]).
    "allow_origins_without_credentials": [...string],

    // allow_origins_with_credentials specifies the allowed origins for requests
    // that include credentials. If a request is made from an Origin in this list
    // Encore responds with Access-Control-Allow-Origin: <Origin>.
    //
    // The URLs in this list may include wildcards (e.g. "https://*.example.com"
    // or "https://*-myapp.example.com").
    "allow_origins_with_credentials": [...string],
}
```

## Allowed origins

The main CORS configuration is the list of allowed origins, meaning which websites are allowed
to access your API (via browsers).

For this purpose, CORS makes a distinction between requests that contain authentication information
(cookies, HTTP authentication, or client certificates) and those that do not. CORS applies stricter
rules to authenticated requests.

By default, Encore allows unauthenticated requests from all origins but disallows requests that do
include authorization information from other origins. This is a good default for many APIs.
This can be changed by setting the `allow_origins_without_credentials` key (see above).
For convenience Encore also allows all origins when developing locally.

For security reasons it's necessary to explicitly specify which origins are allowed to make
authenticated requests. This is done by setting the `allow_origins_with_credentials` key (see above).

## Allowed headers and exposed headers

CORS also lets you specify which headers are allowed to be sent by the client ("allowed headers"),
and which headers are exposed to scripts running in the browser ("exposed headers").

Encore automatically configures headers by parsing your program using static analysis.
If your API defines a request or response type that contains a header field, Encore automatically adds the header to
the list of exposed and allowed headers in request types respectively.

To add additional headers to these lists, you can set the `allow_headers` and `expose_headers` keys (see above).
This can be useful when your application relies on custom headers in e.g. raw endpoints that aren't seen by Encore's
static analysis.
