---
seotitle: Adding authentication to APIs to auth users
seodesc: Learn how to add authentication to your APIs and make sure you know who's calling your backend APIs.
title: Authenticating users
subtitle: Knowing what's what and who's who
infobox: {
  title: "Authentication",
  import: "encore.dev/auth",
}
lang: ts
---
Almost every application needs to know who's calling it, whether the user
represents a person in a consumer-facing app or an organization in a B2B app.
Encore supports both use cases in a simple yet powerful way.

As described in the docs for [defining APIs](/docs/ts/primitives/defining-apis),
each API endpoint can be marked as requiring authentication, using the option `auth: true`
when defining the endpoint.


## Authentication Handlers

When an API is defined with `auth: true`, you must define an authentication handler
in your application. The authentication handler is responsible for inspecting incoming
requests to determine what user is authenticated (if any), and computing any other associated
authentication information.

The authentication handler is defined similarly to API endpoints, using the `authHandler`
function imported from `encore.dev/auth`.

Like API endpoints, the authentication handler defines what request information it's interested in,
in the form of HTTP headers, query strings, or cookies.

A simple authentication handler that inspects the `Authorization` header might look like this:

```ts
import { Header, Gateway } from "encore.dev/api";
import { authHandler } from "encore.dev/auth";

// AuthParams specifies the incoming request information
// the auth handler is interested in. In this case it only
// cares about requests that contain the `Authorization` header.
interface AuthParams {
    authorization: Header<"Authorization">;
}

// The AuthData specifies the information about the authenticated user
// that the auth handler makes available.
interface AuthData {
    userID: string;
}

// The auth handler itself.
export const auth = authHandler<AuthParams, AuthData>(
    async (params) => {
        // TODO: Look up information about the user based on the authorization header.
        return {userID: "my-user-id"};
    }
)

// Define the API Gateway that will execute the auth handler:
export const gateway = new Gateway({
    authHandler: auth,
})
```

With this in place, Encore will provision an API Gateway that will process
incoming requests to your application, and whenever a request contains
an `Authorization` header it will first call the authentication handler to
resolve information about the user.


<GitHubLink
    href="https://github.com/encoredev/examples/tree/main/ts/clerk"
    desc="Example application showing an auth handler implementation using Clerk."
/>

### Rejecting authentication

If the auth handler returns an `AuthData` object, Encore will consider the request
authenticated. To instead _reject_ the request, throw an exception. To signal that
the credentials are not valid, throw an `APIError` with code `Unauthenticated`.

For example:

```ts
import { APIError } from "encore.dev/api";

export const auth = authHandler<AuthParams, AuthData>(
    async (params) => {
        throw APIError.unauthenticated("bad credentials");
    }
)
```

## Understanding the Authentication Process

Encore's authentication process proceeds in two steps:

1. Determine if the request is authenticated
2. Call the endpoint, if permissible

#### Step 1: Determining if the request is authenticated

Whenever an incoming request contains any of the authentication parameters (defined by the auth handler),
Encore's API Gateway calls the auth handler to resolve the authentication data.

This happens regardless of the endpoint the request is for. Importantly, it happens even
when calling an endpoint that does not require authentication.

There are three possible outcomes from calling the auth handler:

1. If the auth handler succeeds, by returning `AuthData`, the request is considered authenticated.

2. If the auth handler throws an `APIError` with code `Unauthenticated`, the request is considered unauthenticated,
   exactly as if there was no authentication parameters in the request to begin with.

3. If the auth handler throws any other exception, the API Gateway aborts the request and returns the error to the caller.

Finally, if the request does not contain authentication data, the request is considered unauthenticated.

#### Step 2: Calling the endpoint, if permissible

Once the API Gateway has determined whether the request is authenticated, it checks whether the API Endpoint
being called requires authentication data.

If it does require authentication, and the request is not authenticated,
the API Gateway aborts the request and returns an "unauthenticated" error to the caller.

In all other situations, the API Gateway proceeds by calling the target endpoint.

If the request was successfully authenticated, the authentication data is passed along to the endpoint,
regardless of whether the endpoint requires authentication or not.

## Using auth data

If a request has been successfully authenticated, the API Gateway forwards the authentication data
to the target endpoint. The endpoint can query the available auth data from the `getAuthData` function,
available from the `~encore/auth` module.

This module is dynamically generated by Encore to enable type-safe resolution of the auth data.

### Propagating auth data

Encore automatically propagates the auth data when you make API calls to other Encore API endpoints
using the generated `~encore/clients` package.

<Callout type="info">

If an endpoint calls another endpoint during its processing, and the target endpoint
requires authentication while the original request does not have any authentication data,
the API call will fail with error code `Unauthenticated`.

This behavior preserves the guarantee that endpoints that
require authentication always have valid authentication data present.

</Callout>

## Overriding auth information

You can override the auth data for a specific endpoint when calling it via `~encore/clients` by passing `CallOpts`. Example:

```ts
import { svc } from "~encore/clients";

const resp = await svc.endpoint(params, { authData: { userID: "...", userEmail: "..." } });
```

<Callout type="info">

Overriding auth data is useful for testing endpoints that require authentication without having to
authenticate the request manually.

</Callout>

## Mocking auth

You can mock `getAuthData` with vitest. Example:

```ts
import { describe, expect, test, vi } from "vitest";
import * as auth from "~encore/auth";
import { get } from "./hello";


describe("get", () => {
  test("should combine string with parameter value", async () => {
    const spy = vi.spyOn(auth, 'getAuthData');
    spy.mockImplementation(() => ({ userEmail: "user@email.com" }))

    const resp = await get({ name: "world" });
    expect(resp.message).toBe("Hello world! You are authenticated with user@email.com");
  });
});
```
