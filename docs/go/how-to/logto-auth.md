---
seotitle: How to use Logto for your backend application
seodesc: Learn how to use Logto for user authentication in your backend application. In this guide we show you how to integrate your Go backend with Logto.
title: Use Logto with your app
lang: go
---

[Logto](https://logto.io) is a modern Auth0 alternative that helps you build the sign-in experience and user identity within minutes. It's particularly well-suited for protecting API services built with Encore.

This guide will show you how to integrate Logto with your Encore application to add authentication and authorization capabilities. You can find the complete [Logto example](https://github.com/encoredev/examples/tree/main/logto-react-sdk) in our examples repo.

## Logto settings

Before we begin integrating with Encore, you'll need to set up a few things in Logto:

1. Create an account at [Logto Cloud](https://cloud.logto.io) if you don't have one yet.

2. Create an API Resource in Logto Console, this represents your Encore API service
   - Go to "API Resources" in Logto Console and create a new API
   - Set a name and API identifier (e.g., `https://api.encoreapp.com`)
   - Note down the API identifier on the API resource details page as we'll need it later
  
  <img src="/assets/docs/logto-api-resource.png" title="Logto API Resource"/>

3. Create an application for your frontend application
  - Go to "Applications" in Logto Console
  - Create a new application according to your frontend framework (We use React as an example, but you can create any Single-Page Application (SPA) or native app)
  - (Optional, we'll cover this later) Integrate Logto with your frontend application according to the guide in the Logto Console.
  - Note down the application ID and issuer URL on the Application details page as we'll need them later

  <img src="/assets/docs/logto-application-endpoints.png" title="Logto application endpoints"/>

## Setup the auth handler

Now let's implement the authentication in your Encore application. We'll use Encore's built-in [auth handler](/docs/go/develop/auth) to validate Logto's JWT tokens.

Add these two modules in your Encore application:

```shell
$ go get github.com/golang-jwt/jwt/v5
$ go get github.com/MicahParks/keyfunc/v3
```

Create `auth/auth.go` and add the following code:

```go
package auth

import (
	"context"
	"time"

	"encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/config"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// Configuration variables for authentication
type LogtoAuthConfig struct {
	// The issuer URL
	Issuer config.String
	// URL to fetch JSON Web Key Set (JWKS)
	JwksUri config.String
	// Expected audience for the JWT
	ApiResourceIndicator config.String
	// Expected client ID in the token claims
	ClientId config.String
}

var authConfig *LogtoAuthConfig = config.Load[*LogtoAuthConfig]()

// RequiredClaims defines the expected structure of JWT claims
// Extends the standard JWT claims with a custom ClientID field
type RequiredClaims struct {
	ClientID string `json:"client_id"`
	jwt.RegisteredClaims
}

// AuthHandler validates JWT tokens and extracts the user ID
// Implements Encore's authentication handler interface
//
//encore:authhandler
func AuthHandler(ctx context.Context, token string) (auth.UID, error) {
	// Fetch and parse the JWKS (JSON Web Key Set) from the identity provider
	jwks, err := keyfunc.NewDefaultCtx(ctx, []string{authConfig.JwksUri()})
	if err != nil {
		return "", &errs.Error{
			Code:    errs.Internal,
			Message: "failed to fetch JWKS",
		}
	}

	// Parse and validate the JWT token with required claims and validation options
	parsedToken, err := jwt.ParseWithClaims(
		token,
		&RequiredClaims{},
		jwks.Keyfunc,
		// Expect the token to be intended for this API resource
		jwt.WithAudience(authConfig.ApiResourceIndicator()),
		// Expect the token to be issued by this issuer
		jwt.WithIssuer(authConfig.Issuer()),
		// Allow some leeway for clock skew
		jwt.WithLeeway(time.Minute*10),
	)

	// Check if there were any errors during token parsing
	if err != nil {
		return "", &errs.Error{
			Code:    errs.Unauthenticated,
			Message: "invalid token",
		}
	}

	// Verify that the client ID in the token matches the expected client ID
	if parsedToken.Claims.(*RequiredClaims).ClientID != authConfig.ClientId() {
		return "", &errs.Error{
			Code:    errs.Unauthenticated,
			Message: "invalid token",
		}
	}

	// Extract the user ID (subject) from the token claims
	userId, err := parsedToken.Claims.GetSubject()
	if err != nil {
		return "", &errs.Error{
			Code:    errs.Unauthenticated,
			Message: "invalid token",
		}
	}

	// Return the user ID as an Encore auth.UID
	return auth.UID(userId), nil
}
```

Create a [configuration file](https://encore.dev/docs/go/develop/config) in the auth service and name it `auth-config.cue`. Add the following:

```cue
Issuer: "<your-logto-issuer-url>"
JwksUri: "<your-logto-issuer-url>/jwks"
ApiResourceIndicator: "<your-api-resource-indicator>"
ClientId: "<your-client-id>"
```

Replace the values with the ones you noted down from your Logto settings:
- `<your-logto-issuer-url>`: The issuer URL from your Logto application endpoints (e.g., `https://your-tenant.logto.app`)
- `<your-api-resource-indicator>`: The API identifier you set when creating the API resource (e.g., `https://api.encoreapp.com`)
- `<your-client-id>`: The application ID from your Logto application details page

For example, your `auth-config.cue` might look like:

```cue
Issuer: "https://your-tenant.logto.app"
JwksUri: "https://your-tenant.logto.app/jwks"
ApiResourceIndicator: "https://api.encoreapp.com"
ClientId: "2gadf3mp0zotlq8j1k5x"
```

And then, you can use this auth handler to protect your API endpoints:

```go
package api

import (
	"context"
	"fmt"

	"encore.dev/beta/auth"
	"encore.dev/beta/errs"
)

//encore:api auth path=/api/hello
func Api(ctx context.Context) (*Response, error) {
	userId, hasUserId := auth.UserID()

	if !hasUserId {
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: "User ID not found",
		}
	}

	msg := fmt.Sprintf("Hello, %s!", userId)

	return &Response{Message: msg}, nil
}

type Response struct {
	Message string
}

```

## Frontend

We've completed our work in the Encore API service. Now we need to integrate Logto with our frontend application.

You can choose the framework you are using in the [Logto Quick start](https://docs.logto.io/quick-starts) page to integrate Logto with your frontend application. In this guide we use React as an example.

Check out the [Add authentication to your React application](https://docs.logto.io/quick-starts/react) guide to learn how to integrate Logto with your React application. In this example, you only need to complete up to the Integration section. After that, we'll demonstrate how the frontend application can obtain an access token from Logto to access the Encore API.

First, update your `LogtoConfig` by adding the API resource used in your Encore app to the `resources` field. This tells Logto that we will be requesting access tokens for this API resource (Encore API).

```ts
import { LogtoConfig } from '@logto/react';

const config: LogtoConfig = {
  // ...other configs
  resources: ['<your-api-resource-indicator>'],
};
```

After updating the `LogtoConfig`, if a user is already signed in, they need to sign out and sign in again for the new `LogtoConfig` settings to take effect.

Once the user is logged in, you can use the `getAccessToken` method provided by the Logto React SDK to obtain an access token for accessing specific API resources. For example, to access the Encore API, we use `https://api.encoreapp.com` as the API resource identifier.

Then, add this access token to the request headers as the `Authorization` field in subsequent requests.

```ts
const { getAccessToken } = useLogto();
const accessToken = await getAccessToken('<your-api-resource-indicator>');

// Add this access token to the request headers as the 'Authorization' field in subsequent requests
fetch('<your-encore-api-endpoint>/hello', {
  headers: {
    Authorization: `Bearer ${accessToken}`,
  },
});
```

Here's the key frontend code:

```tsx
-- config/logto.tsx --
import { LogtoConfig } from '@logto/react'

export const config: LogtoConfig = {
  endpoint: '<your-logto-endpoint>',
  appId: '<your-app-id>',
  resources: ['<your-api-resource-indicator>'],
}

export const appConfig = {
  apiResourceIndicator: '<your-api-resource-indicator>',
  signInRedirectUri: '<your-sign-in-redirect-uri>',
  signOutRedirectUri: '<your-sign-out-redirect-uri>',
}

export const encoreApiEndpoint = '<your-encore-api-endpoint>'
-- pages/ProtectedResource.tsx --
import { useLogto } from "@logto/react";
import { useState } from "react";
import { Navigate } from "react-router-dom";
import { appConfig, encoreApiEndpoint } from "../config/logto";

export function ProtectedResource() {
  const { isAuthenticated, getAccessToken } = useLogto();
  const [message, setMessage] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");

  const fetchProtectedResource = async () => {
    setIsLoading(true);
    setError("");
    try {
      const accessToken = await getAccessToken(appConfig.apiResourceIndicator);
      const response = await fetch(`${encoreApiEndpoint}/api/hello`, {
        headers: {
          Authorization: `Bearer ${accessToken}`,
        },
      });
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data = await response.json();
      setMessage(JSON.stringify(data));
    } catch (error) {
      console.error("Error fetching protected resource:", error);
      setError("Failed to fetch protected resource. Please try again.");
    } finally {
      setIsLoading(false);
    }
  };

  if (!isAuthenticated) {
    return <Navigate to="/" replace />;
  }

  return (
    <div>
      <h2>Protected Resource</h2>

      {message && !error && (
        <div>
          <h3>Response from Protected API</h3>
          <pre>{message}</pre>
        </div>
      )}

      <button
        onClick={fetchProtectedResource}
        disabled={isLoading}
      >
        {isLoading ? "Loading..." : "Fetch protected resource"}
      </button>

      {error && <div>{error}</div>}
    </div>
  );
}
```

That's it, you've successfully integrated Logto with your Encore application.

You can find the complete example code [here](https://github.com/encoredev/examples/tree/main/logto-react-sdk).

## Explore more

If you want to use more Logto features, you can refer to the following links for more information:

- Combine Logto's [Custom token claims](https://docs.logto.io/developers/custom-token-claims) to set [custom user data](/docs/go/develop/auth#with-custom-user-data) in the auth handler
- Use [Logto RBAC features](https://docs.logto.io/authorization/role-based-access-control) to add authorization support to your application. The React integration tutorial also demonstrates how to add `scope` information to your Access token (note that you need to sign in again after updating Logto config)
