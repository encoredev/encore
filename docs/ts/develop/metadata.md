---
seotitle: Metadata API – Get data about the app and environment
seodesc: See how to use Encore's Metadata API to get information about the app and the environment it's running in.
title: Metadata
subtitle: Use the metadata API to get information about the app and the environment it's running in
infobox: { title: "Metadata API", import: "encore.dev" }
lang: ts
---

While Encore tries to provide a cloud-agnostic environment, sometimes it's helpful to know more about the environment
your application is running in. For this reason Encore provides an API for accessing metadata about the
[application](#application-metadata) and the environment it's running in as
part of the `encore.dev` package.

## Application Metadata

Calling `appMeta()` from the `encore.dev` package returns an object that
contains information about the application, including:

- `appID` - the application name.
- `apiBaseURL` - the URL the application API can be publicly accessed on.
- `environment` - the [environment](/docs/deploy/environments) the application is currently running in.
- `build` - the revision information of the build from the version control system.
- `deploy` - the deployment ID and when this version of the app was deployed.

## Example Use Cases

### Using Cloud Specific Services

All the [clouds](/docs/deploy/own-cloud) contain a large number of services, not all of which Encore natively supports.

By using information about the [environment](/docs/deploy/environments), you can define the implementation of these and use different services for each environment's provider.

For instance if you are pushing audit logs into a data warehouse, when running on GCP you could use BigQuery, but when running on AWS you could use Redshift, when running locally you could simply write them to a file.

```ts
import { appMeta } from "encore.dev";

// Emit an audit event.
async function audit(userID: string, event: Record<string, any>) {
  const cloud = appMeta().environment.cloud;
  switch (cloud) {
    case "aws":
      return writeIntoRedshift(userID, event);
    case "gcp":
      return writeIntoBigQuery(userID, event);
    case "local":
      return writeIntoFile(userID, event);
    default:
      throw new Error(`unknown cloud: ${cloud}`);
  }
}
```

### Checking Environment type

When implementing a signup system, you may want to skip email verification on user signups when developing the application.
Using the `appMeta` API, we can check the environment and decide whether to send an email or simply mark the user as
verified upon signup.

```ts
import { appMeta } from "encore.dev";

export const signup = api(
  { expose: true },
  async (params: SignupParams): Promise<SignupResponse> => {
    // more code...

    // If this is a testing environment, skip sending the verification email.
    switch (appMeta().environment.type) {
      case ("test", "development"):
        await markEmailVerified(userID);
        break;
      default:
        await sendVerificationEmail(userID);
        break;
    }

    // more code...
  },
);
```
