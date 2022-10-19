---
title: Metadata
subtitle: Not everything is meta...
---

While Encore tries to provide a cloud-agnostic environment, sometimes it's helpful to know more about the environment
your application is running in. For this reason Encore provides an API for accessing metadata about the
[application](#application-metadata) and the environment it's running in, as well as information about the
[current request](#current-request) as part of the `encore.dev` package.

## Application Metadata

Calling `encore.Meta()` will return an [encore.AppMetadata](https://pkg.go.dev/encore.dev/#AppMetadata) instance which
contains information about the application, including:

 - `AppID` - the application name.
 - `APIBaseURL` - the URL the application API can be publicly accessed on.
 - `Environment` - the [environment](/docs/deploy/environments) the application is currently running in.
 - `Build` - the revision information of the build from the version control system.
 - `Deploy` - the deployment ID and when this version of the app was deployed.

## Current Request

`encore.CurrentRequest()` can be called from anywhere within your application and will return an
[encore.Request](https://pkg.go.dev/encore.dev/#Request) instance which will provides information about why the current
code is running.

The [encore.Request](https://pkg.go.dev/encore.dev/#Request) type contains information about the running request, such as:
 - The service and endpoint being called
 - Path and path parameter information
 - When the request started

This works automatically as a result of Encore's request tracking, and works even in other goroutines that were spawned
during request handling.  If no request is processed by the caller, which can happen if you call it during service
initialization, the Type field returns None. If `CurrentRequest()` is called from a goroutine spawned during request
processing it will continue to report the same request even if the request handler has already returned.

This can be useful on [raw endpoints](/docs/develop/services-and-apis#raw-endpoints) with [path parameters](/docs/develop/services-and-apis#rest-apis)
as the standard `http.Request` object passed into the raw endpoint does not provide access to the parsed path parameters,
however by calling `encore.CurrentRequest().PathParams()` you can get access to the parsed path parameters.


## Example Use Cases

### Using Cloud Specific Services

All the [clouds](/docs/deploy/own-cloud) contain a large number of services, not all of which Encore natively supports.
By using information about the [environment](/docs/deploy/environments), you can define the implementation of these and
use different services for each environment's provider. For instance if you are pushing audit logs into a data warehouse, when running on GCP you could use BigQuery, but when running on AWS you could use Redshift, when running locally you could
simply write them to a file.

```go
package audit

import (
    "encore.dev"
    "encore.dev/beta/auth"
)

func Audit(ctx context.Context, action message, user auth.UID) error {
    switch encore.Meta().Environment.Cloud {
    case encore.CloudAWS:
        return writeIntoRedshift(ctx, action, user)
    case encore.CloudGCP:
        return writeIntoBigQuery(ctx, action, user)
    case encore.CloudLocal:
        return writeIntoFile(ctx, action, user)
    default:
        return fmt.Errorf("unknown cloud: %s", encore.Meta().Environment.Cloud)
    }
}
```

### Checking Environment type

When implementing a signup system, you may want to skip email verification on user signups when developing the application.
Using the `encore.Meta()` API, we can check the environment and decide whether to send an email or simply mark the user as
verified upon signup.

```go
package user

import "encore.dev"

//encore:api public
func Signup(ctx context.Context, params *SignupParams) (*SignupResponse, error) {
    // ...

    // If this is a testing environment, skip sending the verification email
    switch encore.Meta().Environment.Type {
    case encore.EnvTest, encore.EnvDevelopment:
        if err := MarkEmailVerified(ctx, userID); err != nil {
            return nil, err
        }
    default:
        if err := SendVerificationEmail(ctx, userID); err != nil {
            return nil, err
        }
    }

    // ...
}
```
