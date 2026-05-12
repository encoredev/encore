---
title: encore.dev
lang: ts
toc: true
---

# encore.dev

## Interfaces

### APICallMeta

Defined in: [req\_meta.ts:33](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L33)

Describes an API call being processed.

#### Properties

##### api

```ts
api: APIDesc;
```

Defined in: [req\_meta.ts:38](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L38)

Describes the API Endpoint being called.

##### headers

```ts
headers: Record<string, string | string[]>;
```

Defined in: [req\_meta.ts:71](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L71)

The request headers from the HTTP request.
The values are arrays if the header contains multiple values,
either separated by ";" or when the header key appears more than once.

##### method

```ts
method: Method;
```

Defined in: [req\_meta.ts:41](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L41)

The HTTP method used in the API call.

##### middlewareData?

```ts
optional middlewareData?: Record<string, any>;
```

Defined in: [req\_meta.ts:83](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L83)

Contains values set in middlewares via `MiddlewareRequest.data`.

##### parsedPayload?

```ts
optional parsedPayload?: Record<string, any>;
```

Defined in: [req\_meta.ts:78](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L78)

The parsed request payload, as expected by the application code.
Not provided for raw endpoints or when the API endpoint expects no
request data.

##### path

```ts
path: string;
```

Defined in: [req\_meta.ts:48](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L48)

The request URL path used in the API call,
excluding any query string parameters.
For example "/path/to/endpoint".

##### pathAndQuery

```ts
pathAndQuery: string;
```

Defined in: [req\_meta.ts:55](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L55)

The request URL path used in the API call,
including any query string parameters.
For example "/path/to/endpoint?with=querystring".

##### pathParams

```ts
pathParams: Record<string, any>;
```

Defined in: [req\_meta.ts:64](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L64)

The parsed path parameters for the API endpoint.
The keys are the names of the path parameters,
from the API definition.

For example {id: 5}.

##### type

```ts
type: "api-call";
```

Defined in: [req\_meta.ts:35](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L35)

Specifies that the request is an API call.

***

### APIDesc

Defined in: [req\_meta.ts:4](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L4)

Describes an API endpoint.

#### Properties

##### auth

```ts
auth: boolean;
```

Defined in: [req\_meta.ts:15](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L15)

Whether the endpoint requires auth.

##### endpoint

```ts
endpoint: string;
```

Defined in: [req\_meta.ts:9](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L9)

The name of the endpoint itself.

##### raw

```ts
raw: boolean;
```

Defined in: [req\_meta.ts:12](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L12)

Whether the endpoint is a raw endpoint.

##### service

```ts
service: string;
```

Defined in: [req\_meta.ts:6](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L6)

The name of the service that the endpoint belongs to.

##### tags

```ts
tags: string[];
```

Defined in: [req\_meta.ts:18](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L18)

Tags specified on the endpoint.

***

### AppMeta

Defined in: [app\_meta.ts:4](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L4)

Describes the running Encore application.

#### Properties

##### apiBaseUrl

```ts
apiBaseUrl: string;
```

Defined in: [app\_meta.ts:19](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L19)

The base URL which can be used to call the API of this running application.

For local development it is "http://localhost:<port>", typically "http://localhost:4000".

If a custom domain is used for this environment it is returned here, but note that
changes only take effect at the time of deployment while custom domains can be updated at any time.

##### appId

```ts
appId: string;
```

Defined in: [app\_meta.ts:9](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L9)

The Encore application ID. If the application is not linked to the Encore platform this will be an empty string.
To link to the Encore platform run `encore app link` from your terminal in the root directory of the Encore app.

##### build

```ts
build: BuildMeta;
```

Defined in: [app\_meta.ts:25](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L25)

Information about the build.

##### deploy

```ts
deploy: DeployMeta;
```

Defined in: [app\_meta.ts:28](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L28)

Information about the deployment.

##### environment

```ts
environment: EnvironmentMeta;
```

Defined in: [app\_meta.ts:22](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L22)

Information about the environment the app is running in.

***

### BaseRequestMeta

Defined in: [req\_meta.ts:145](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L145)

Common fields shared by all request meta types.

#### Properties

##### trace?

```ts
optional trace?: TraceData;
```

Defined in: [req\_meta.ts:147](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L147)

Information about the trace, if the request is being traced

***

### BuildMeta

Defined in: [app\_meta.ts:72](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L72)

Information about the build that formed the running application.

#### Properties

##### revision

```ts
revision: string;
```

Defined in: [app\_meta.ts:74](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L74)

The git commit that formed the base of this build.

##### uncommittedChanges

```ts
uncommittedChanges: boolean;
```

Defined in: [app\_meta.ts:77](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L77)

Whether there were uncommitted changes on top of the commit.

***

### DeployMeta

Defined in: [app\_meta.ts:81](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L81)

Information about the deployment of the running application.

#### Properties

##### hostedServices

```ts
hostedServices: Record<string, HostedService>;
```

Defined in: [app\_meta.ts:86](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L86)

The services hosted by this deployment, keyed by the service name.

##### id

```ts
id: string;
```

Defined in: [app\_meta.ts:83](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L83)

The unique id of the deployment. Generated by the Encore Platform.

***

### EnvironmentMeta

Defined in: [app\_meta.ts:32](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L32)

Describes the environment the Encore application is running in.

#### Properties

##### cloud

```ts
cloud: CloudProvider;
```

Defined in: [app\_meta.ts:49](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L49)

The cloud this is running in.
For local development it is "local".

##### name

```ts
name: string;
```

Defined in: [app\_meta.ts:37](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L37)

The name of environment that this application.
For local development it is "local".

##### type

```ts
type: EnvironmentType;
```

Defined in: [app\_meta.ts:43](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L43)

The type of environment is this application running in.
For local development it is "development".

***

### HostedService

Defined in: [app\_meta.ts:89](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L89)

#### Properties

##### name

```ts
name: string;
```

Defined in: [app\_meta.ts:91](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L91)

The name of the service

***

### PubSubMessageMeta

Defined in: [req\_meta.ts:87](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L87)

Describes a Pub/Sub message being processed.

#### Properties

##### deliveryAttempt

```ts
deliveryAttempt: number;
```

Defined in: [req\_meta.ts:111](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L111)

The delivery attempt. The first attempt starts at 1,
and increases by 1 for each retry.

##### messageId

```ts
messageId: string;
```

Defined in: [req\_meta.ts:105](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L105)

The unique id of the Pub/Sub message.
It is the same id returned by `topic.publish()`.
The message id stays the same across delivery attempts.

##### parsedPayload?

```ts
optional parsedPayload?: Record<string, any>;
```

Defined in: [req\_meta.ts:116](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L116)

The parsed request payload, as expected by the application code.

##### service

```ts
service: string;
```

Defined in: [req\_meta.ts:92](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L92)

The service processing the message.

##### subscription

```ts
subscription: string;
```

Defined in: [req\_meta.ts:98](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L98)

The name of the Pub/Sub subscription.

##### topic

```ts
topic: string;
```

Defined in: [req\_meta.ts:95](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L95)

The name of the Pub/Sub topic.

##### type

```ts
type: "pubsub-message";
```

Defined in: [req\_meta.ts:89](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L89)

Specifies that the request is a Pub/Sub message.

***

### TraceData

Defined in: [req\_meta.ts:120](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L120)

Provides information about the active trace.

#### Properties

##### extCorrelationId?

```ts
optional extCorrelationId?: string;
```

Defined in: [req\_meta.ts:141](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L141)

The external correlation id provided when the trace
was created, if any.
For example via the `Request-Id` or `X-Correlation-Id` headers.

##### parentSpanId?

```ts
optional parentSpanId?: string;
```

Defined in: [req\_meta.ts:134](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L134)

The span that initiated this span, if any.

##### parentTraceId?

```ts
optional parentTraceId?: string;
```

Defined in: [req\_meta.ts:129](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L129)

The trace id that initiated this trace, if any.

##### spanId

```ts
spanId: string;
```

Defined in: [req\_meta.ts:124](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L124)

The current span id.

##### traceId

```ts
traceId: string;
```

Defined in: [req\_meta.ts:122](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L122)

The trace id.

## Type Aliases

### CloudProvider

```ts
type CloudProvider = "aws" | "gcp" | "azure" | "encore" | "local";
```

Defined in: [app\_meta.ts:64](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L64)

Describes what cloud provider the application is running in.

***

### EnvironmentType

```ts
type EnvironmentType = "production" | "development" | "ephemeral" | "test";
```

Defined in: [app\_meta.ts:53](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L53)

Describes what type of environment the application is running in.

***

### Method

```ts
type Method = 
  | "GET"
  | "POST"
  | "PUT"
  | "PATCH"
  | "DELETE"
  | "HEAD"
  | "OPTIONS"
  | "CONNECT"
  | "TRACE";
```

Defined in: [req\_meta.ts:21](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L21)

***

### RequestMeta

```ts
type RequestMeta = APICallMeta | PubSubMessageMeta & BaseRequestMeta;
```

Defined in: [req\_meta.ts:151](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L151)

Describes an API call or Pub/Sub message being processed.

## Functions

### appMeta()

```ts
function appMeta(): AppMeta;
```

Defined in: [app\_meta.ts:100](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/app_meta.ts#L100)

Returns metadata about the running Encore application.

The metadata is cached and is the same object each call,
and therefore must not be modified by the caller.

#### Returns

[`AppMeta`](#appmeta)

***

### currentRequest()

```ts
function currentRequest(): RequestMeta | undefined;
```

Defined in: [req\_meta.ts:160](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/req_meta.ts#L160)

Returns information about the running Encore request,
such as API calls and Pub/Sub messages being processed.

Returns undefined only if no request is being processed,
such as during system initialization.

#### Returns

[`RequestMeta`](#requestmeta) \| `undefined`
