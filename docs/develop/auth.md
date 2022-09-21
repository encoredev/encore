---
title: Authenticating users
subtitle: Knowing what's what and who's who
---
Almost every application needs to know who's calling it, whether the user
represents a person in a consumer-facing app or an organization in a B2B app.
Encore supports both use cases in a simple yet powerful way.

As described in [defining APIs](/docs/concepts/services-and-apis), Encore offers three access levels
for APIs:

* `//encore:api public` &ndash; defines a public API that anybody on the internet can call
* `//encore:api private` &ndash; defines a private API that only other backend services can call
* `//encore:api auth` &ndash; defines a public API that anybody can call, but that requires valid authentication.

When an API is defined with access level `auth`, outside calls to that API must specify
an authorization header, in the form `Authorization: Bearer <token>`. The token is passed to
a designated auth handler function and the API call is allowed to go through only if the
auth handler determines the token is valid.

## The auth handler

Encore applications can designate a special function to handle authentication,
by defining a function and annotating it with `//encore:authhandler`. This annotation
tells Encore to run the function whenever an incoming API call contains an authentication token.

The auth handler is responsible for validating the incoming authentication token
and returning an `auth.UID` (a string type representing a **user id**). The `auth.UID`
can be whatever you wish, but in practice it usually maps directly to the primary key
stored in a user table (either defined in the Encore service or in an external service like Firebase or Okta).

### With custom user data

Oftentimes it's convenient for the rest of your application to easily be able to look up
information about the authenticated user making the request. If that's the case,
define the auth handler like so:

```go
import "encore.dev/beta/auth"

// Data can be named whatever you prefer (but must be exported).
type Data struct {
    Username string
    // ...
}

// AuthHandler can be named whatever you prefer (but must be exported).
//encore:authhandler
func AuthHandler(ctx context.Context, token string) (auth.UID, *Data, error) {
    // Validate the token and look up the user id and user data,
    // for example by calling Firebase Auth.
}
```

### Without custom user data

When you don't require custom user data and it's sufficient to use `auth.UID`,
simply skip it in the return type:

```go
import "encore.dev/beta/auth"

// AuthHandler can be named whatever you prefer (but must be exported).
//encore:authhandler
func AuthHandler(ctx context.Context, token string) (auth.UID, error) {
    // Validate the token and look up the user id,
    // for example by calling Firebase Auth.
}
```

## Handling auth errors

When a token doesn't match your auth rules (for example if it's expired, the token has been revoked, or the token is invalid), you should return a non-nil error from the auth handler.

Encore passes the error message on to the user when you use [Encore's built-in error package](errors), so we recommend using that with the error code `Unauthenticated` to communicate what happened. For example:

```go
import "encore.dev/beta/errs"

//encore:authhandler
func AuthHandler(ctx context.Context, token string) (auth.UID, error) {
    return "", &errs.Error{
        Code: errs.Unauthenticated,
        Message: "invalid token",
    }
}
```

<Callout type="important">

Note that for security reasons you may not want to reveal too much information about why a request did not pass your auth checks. There are many subtle security considerations when dealing with authentication and we don't have time to go into all of them here.

Whenever possible we recommend using a third-party auth provider.<br/>
See [Using Firebase Authentication](/docs/how-to/firebase-auth) for an example of how to do that.

</Callout>

## Using auth data

Once the user has been identified by the auth handler, the API handler is called
as usual. If it wishes to inspect the authenticated user, it can use the
`encore.dev/beta/auth` package:

- `auth.Data()` returns the custom user data returned by the auth handler (if any)
- `auth.UserID()` returns `(auth.UID, bool)` to get the authenticated user id (if any)

For an incoming request from the outside to an API that uses the `auth` access level,
these are guaranteed to be set since the API won't be called if the auth handler doesn't succeed.

<Callout type="important">

If an endpoint calls another endpoint during its processing, and the original
does not have an authenticated user, the request will fail. This behavior
preserves the guarantees that `auth` endpoints always have an authenticated user.

</Callout>

Note that the auth handler is invoked for **all** requests that specify an auth token,
which lets users optionally authenticate to public APIs. This can be useful in some
circumstances to return additional information. The second return value from `auth.UserID()`
can be used to determine if the request has an authenticated user.
