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

- `appId` - the application name.
- `apiBaseUrl` - the URL the application API can be publicly accessed on.
- `environment` - the [environment](/docs/platform/deploy/environments) the application is currently running in.
- `build` - the revision information of the build from the version control system.
- `deploy` - the deployment ID and when this version of the app was deployed.

## Current Request

The `currentRequest()` function, also provided by the `encore.dev` module, can be called from anywhere within your application and returns a
`Request` object that contains information the current request being processed.

The object contains different fields depending on whether the
current request is an API call or a Pub/Sub message being processed.

```typescript
-- API Call --
/** Describes an API call being processed. */
export interface APICallMeta {
  /** Specifies that the request is an API call. */
  type: "api-call";

  /** Describes the API Endpoint being called. */
  api: APIDesc;

  /** The HTTP method used in the API call. */
  method: Method;

  /**
   * The request URL path used in the API call,
   * excluding any query string parameters.
   * For example "/path/to/endpoint".
   */
  path: string;

  /**
   * The request URL path used in the API call,
   * including any query string parameters.
   * For example "/path/to/endpoint?with=querystring".
   */
  pathAndQuery: string;

  /**
   * The parsed path parameters for the API endpoint.
   * The keys are the names of the path parameters,
   * from the API definition.
   *
   * For example {id: 5}.
   */
  pathParams: Record<string, any>;

  /**
   * The request headers from the HTTP request.
   * The values are arrays if the header contains multiple values,
   * either separated by ";" or when the header key appears more than once.
   */
  headers: Record<string, string | string[]>;

  /**
   * The parsed request payload, as expected by the application code.
   * Not provided for raw endpoints or when the API endpoint expects no
   * request data.
   */
  parsedPayload?: Record<string, any>;
}

-- Pub/Sub Message --
/** Describes a Pub/Sub message being processed. */
export interface PubSubMessageMeta {
  /** Specifies that the request is a Pub/Sub message. */
  type: "pubsub-message";

  /** The service processing the message. */
  service: string;

  /** The name of the Pub/Sub topic. */
  topic: string;

  /** The name of the Pub/Sub subscription. */
  subscription: string;

  /**
   * The unique id of the Pub/Sub message.
   * It is the same id returned by `topic.publish()`.
   * The message id stays the same across delivery attempts.
   */
  messageId: string;

  /**
   * The delivery attempt. The first attempt starts at 1,
   * and increases by 1 for each retry.
   */
  deliveryAttempt: number;

  /**
   * The parsed request payload, as expected by the application code.
   */
  parsedPayload?: Record<string, any>;
}
```

This works automatically as a result of Encore's request tracking.
If no request is processed by the caller, which can happen if you call it during service
initialization, `currentRequest()` returns `undefined`.


## Example Use Cases

### Using Cloud Specific Services

All the [clouds](/docs/platform/deploy/own-cloud) contain a large number of services, not all of which Encore natively supports.

By using information about the [environment](/docs/platform/deploy/environments), you can define the implementation of these and use different services for each environment's provider.

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
      case "test":
      case "development":
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
