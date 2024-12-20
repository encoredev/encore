---
seotitle: Encore Cloud OAuth Clients
seodesc: Learn how to use OAuth Clients for access to the Encore Cloud API
title: OAuth Clients
lang: platform
---

OAuth clients provide a framework for delegated and scoped access to the Encore Cloud API. An OAuth client creates short-lived access tokens on demand, and supports the principle of least privilege by allowing fine-grained control on the access granted to the client using scopes.

## How it works
You create an OAuth client that defines the scopes to allow when your client application uses the Encore Cloud API.

Scopes are currently grouped into "roles", which include a set of permissions.
For example, the `deployer` role grants access to the triggering deployments.

An OAuth client consists of a client ID and a client secret. When you create an OAuth client, Encore Cloud creates these for you. Within your client application, use the client ID and client secret to request an API access token from the Encore Cloud's OAuth token endpoint. You use the access token to make calls to the Encore Cloud API. The access token grants permission only for the scopes that were defined when you created the OAuth client.

An API access token expires after one hour. For continuous access, shortly before an API access token expires, request a new API access token from Encore Cloud's OAuth token endpoint.

OAuth client libraries in popular programming languages can handle the API access token generation and renewal.

Encore Cloud's OAuth implementation is based on the [OAuth 2.0 protocol](https://www.rfc-editor.org/rfc/rfc6749).

## Prerequisites
You need to be an Owner of the Encore application in order to create or revoke OAuth clients.

### Setting up an OAuth client
Open the OAuth clients page in the application settings page.

In the Generate OAuth client dialog, select the set of operations that can be performed with tokens created by the new OAuth client.

After generating the client, you can see the new OAuth client's ID and secret. Copy both the client ID and secret, as you need them for your client code.
Note that after you close the Generated new OAuth client dialog, you won't be able to copy the secret again.
**Store the client secret securely.**

Your OAuth client is now configured. Use the client ID and secret when you configure your OAuth client application. Note that the provided client secrets are case-sensitive.

If an OAuth client is created by a user who is later removed from your application, the OAuth client will continue to function and generate API access tokens.
Application owners can see all configured OAuth clients in the OAuth clients page of the application settings.

### Roles
Roles define which operations are permitted in API access tokens that are created by your client application.

Currently there is a single supported role: **Deployer**. The deployer role
allows for programatically triggering deployments.

When new Encore Cloud functionality is provided, we will add it to existing roles where applicable.
That means a role is not restricted to only access of APIs that existed at the time the client was initially authorized &mdash; a role will contain additional access where it makes sense for new or updated functionality.

### Revoking an OAuth client
Open the OAuth clients page of the application settings page.

Find the OAuth client that you want to delete and select Revoke.

Select Revoke OAuth client to confirm you want to revoke the OAuth client.

When you revoke an OAuth client, any active API access tokens that were created by the client are also revoked.

### Encore Cloud OAuth token endpoint
Encore Cloud's OAuth token endpoint is https://api.encore.cloud/api/oauth/token.
See the [Encore Cloud API Reference](/docs/platform/integrations/api-reference) documentation for more information.

Make requests to the OAuth token endpoint when you need an API access token. The OAuth token endpoint accepts requests that conform to the OAuth 2.0 client credentials grant request format, and returns responses that conform to the OAuth 2.0 client credentials grant response format.

## OAuth client libraries
Popular programming languages provide OAuth client libraries to simplify your use of OAuth clients.

For example, the following Go code shows how to create an OAuth client object that uses your client ID and client secret to generate an API access token for calls to the Encore Cloud API.
Similar libraries exist for other popular programming languages.

```go
package main

import (
    "context"
    "os"

    "golang.org/x/oauth2/clientcredentials"
)

func main() {
    oauthConfig := &clientcredentials.Config{
        ClientID:     os.Getenv("OAUTH_CLIENT_ID"),
        ClientSecret: os.Getenv("OAUTH_CLIENT_SECRET"),
        TokenURL:     "https://api.encore.cloud/api/oauth/token",
    }

    client := oauthConfig.Client(context.Background())

    // Make API calls using `client.Get` etc.
    resp, err := client.Get("https://api.encore.cloud.com/api/...")
    // ...
}
```

The example requires that you define environment variables `OAUTH_CLIENT_ID` and `OAUTH_CLIENT_SECRET`, with their values set to the client ID and client secret that are created when you set up an OAuth client.

### Verifying you can generate API access tokens
After you set up an OAuth client, an easy way to confirm that you can generate API access tokens is to make a curl request to the Encore Cloud OAuth token endpoint.


```bash
curl -d "client_id=${OAUTH_CLIENT_ID}" -d "client_secret=${OAUTH_CLIENT_SECRET}" \
     -d "grant_type=client_credentials" "https://api.encore.cloud/api/oauth/token"
```

The example requires that you define environment variables OAUTH_CLIENT_ID and OAUTH_CLIENT_SECRET, with their values set to your client ID and client secret.

Here's an example response showing the API access token:

```json
{"access_token":"MTcyODQ3NTg3NXww...GDxfmxnuq9zDEAmHmP5D44=","token_type":"Bearer","expires_in":3600, "actor": "o2c_my_key_id"}
```

## Limitations

An OAuth access token expires after 1 hour &mdash; this duration cannot be modified.
