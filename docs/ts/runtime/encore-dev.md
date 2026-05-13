---
title: encore.dev
lang: ts
toc: true
---

# encore.dev

## Interfaces

### APICallMeta

<!-- source: req\_meta.ts:33 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L33 -->

Describes an API call being processed.

#### Properties

##### api

`api: APIDesc;`

<!-- source: req\_meta.ts:38 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L38 -->

Describes the API Endpoint being called.

##### headers

`headers: Record<string, string | string[]>;`

<!-- source: req\_meta.ts:71 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L71 -->

The request headers from the HTTP request.
The values are arrays if the header contains multiple values,
either separated by ";" or when the header key appears more than once.

##### method

`method: Method;`

<!-- source: req\_meta.ts:41 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L41 -->

The HTTP method used in the API call.

##### middlewareData?

`optional middlewareData?: Record<string, any>;`

<!-- source: req\_meta.ts:83 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L83 -->

Contains values set in middlewares via `MiddlewareRequest.data`.

##### parsedPayload?

`optional parsedPayload?: Record<string, any>;`

<!-- source: req\_meta.ts:78 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L78 -->

The parsed request payload, as expected by the application code.
Not provided for raw endpoints or when the API endpoint expects no
request data.

##### path

`path: string;`

<!-- source: req\_meta.ts:48 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L48 -->

The request URL path used in the API call,
excluding any query string parameters.
For example "/path/to/endpoint".

##### pathAndQuery

`pathAndQuery: string;`

<!-- source: req\_meta.ts:55 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L55 -->

The request URL path used in the API call,
including any query string parameters.
For example "/path/to/endpoint?with=querystring".

##### pathParams

`pathParams: Record<string, any>;`

<!-- source: req\_meta.ts:64 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L64 -->

The parsed path parameters for the API endpoint.
The keys are the names of the path parameters,
from the API definition.

For example {id: 5}.

##### type

`type: "api-call";`

<!-- source: req\_meta.ts:35 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L35 -->

Specifies that the request is an API call.

***

### APIDesc

<!-- source: req\_meta.ts:4 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L4 -->

Describes an API endpoint.

#### Properties

##### auth

`auth: boolean;`

<!-- source: req\_meta.ts:15 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L15 -->

Whether the endpoint requires auth.

##### endpoint

`endpoint: string;`

<!-- source: req\_meta.ts:9 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L9 -->

The name of the endpoint itself.

##### raw

`raw: boolean;`

<!-- source: req\_meta.ts:12 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L12 -->

Whether the endpoint is a raw endpoint.

##### service

`service: string;`

<!-- source: req\_meta.ts:6 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L6 -->

The name of the service that the endpoint belongs to.

##### tags

`tags: string[];`

<!-- source: req\_meta.ts:18 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L18 -->

Tags specified on the endpoint.

***

### AppMeta

<!-- source: app\_meta.ts:4 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L4 -->

Describes the running Encore application.

#### Properties

##### apiBaseUrl

`apiBaseUrl: string;`

<!-- source: app\_meta.ts:19 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L19 -->

The base URL which can be used to call the API of this running application.

For local development it is "http://localhost:<port>", typically "http://localhost:4000".

If a custom domain is used for this environment it is returned here, but note that
changes only take effect at the time of deployment while custom domains can be updated at any time.

##### appId

`appId: string;`

<!-- source: app\_meta.ts:9 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L9 -->

The Encore application ID. If the application is not linked to the Encore platform this will be an empty string.
To link to the Encore platform run `encore app link` from your terminal in the root directory of the Encore app.

##### build

`build: BuildMeta;`

<!-- source: app\_meta.ts:25 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L25 -->

Information about the build.

##### deploy

`deploy: DeployMeta;`

<!-- source: app\_meta.ts:28 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L28 -->

Information about the deployment.

##### environment

`environment: EnvironmentMeta;`

<!-- source: app\_meta.ts:22 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L22 -->

Information about the environment the app is running in.

***

### BaseRequestMeta

<!-- source: req\_meta.ts:145 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L145 -->

Common fields shared by all request meta types.

#### Properties

##### trace?

`optional trace?: TraceData;`

<!-- source: req\_meta.ts:147 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L147 -->

Information about the trace, if the request is being traced

***

### BuildMeta

<!-- source: app\_meta.ts:72 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L72 -->

Information about the build that formed the running application.

#### Properties

##### revision

`revision: string;`

<!-- source: app\_meta.ts:74 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L74 -->

The git commit that formed the base of this build.

##### uncommittedChanges

`uncommittedChanges: boolean;`

<!-- source: app\_meta.ts:77 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L77 -->

Whether there were uncommitted changes on top of the commit.

***

### DeployMeta

<!-- source: app\_meta.ts:81 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L81 -->

Information about the deployment of the running application.

#### Properties

##### hostedServices

`hostedServices: Record<string, HostedService>;`

<!-- source: app\_meta.ts:86 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L86 -->

The services hosted by this deployment, keyed by the service name.

##### id

`id: string;`

<!-- source: app\_meta.ts:83 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L83 -->

The unique id of the deployment. Generated by the Encore Platform.

***

### EnvironmentMeta

<!-- source: app\_meta.ts:32 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L32 -->

Describes the environment the Encore application is running in.

#### Properties

##### cloud

`cloud: CloudProvider;`

<!-- source: app\_meta.ts:49 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L49 -->

The cloud this is running in.
For local development it is "local".

##### name

`name: string;`

<!-- source: app\_meta.ts:37 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L37 -->

The name of environment that this application.
For local development it is "local".

##### type

`type: EnvironmentType;`

<!-- source: app\_meta.ts:43 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L43 -->

The type of environment is this application running in.
For local development it is "development".

***

### HostedService

<!-- source: app\_meta.ts:89 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L89 -->

#### Properties

##### name

`name: string;`

<!-- source: app\_meta.ts:91 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L91 -->

The name of the service

***

### PubSubMessageMeta

<!-- source: req\_meta.ts:87 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L87 -->

Describes a Pub/Sub message being processed.

#### Properties

##### deliveryAttempt

`deliveryAttempt: number;`

<!-- source: req\_meta.ts:111 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L111 -->

The delivery attempt. The first attempt starts at 1,
and increases by 1 for each retry.

##### messageId

`messageId: string;`

<!-- source: req\_meta.ts:105 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L105 -->

The unique id of the Pub/Sub message.
It is the same id returned by `topic.publish()`.
The message id stays the same across delivery attempts.

##### parsedPayload?

`optional parsedPayload?: Record<string, any>;`

<!-- source: req\_meta.ts:116 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L116 -->

The parsed request payload, as expected by the application code.

##### service

`service: string;`

<!-- source: req\_meta.ts:92 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L92 -->

The service processing the message.

##### subscription

`subscription: string;`

<!-- source: req\_meta.ts:98 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L98 -->

The name of the Pub/Sub subscription.

##### topic

`topic: string;`

<!-- source: req\_meta.ts:95 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L95 -->

The name of the Pub/Sub topic.

##### type

`type: "pubsub-message";`

<!-- source: req\_meta.ts:89 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L89 -->

Specifies that the request is a Pub/Sub message.

***

### TraceData

<!-- source: req\_meta.ts:120 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L120 -->

Provides information about the active trace.

#### Properties

##### extCorrelationId?

`optional extCorrelationId?: string;`

<!-- source: req\_meta.ts:141 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L141 -->

The external correlation id provided when the trace
was created, if any.
For example via the `Request-Id` or `X-Correlation-Id` headers.

##### parentSpanId?

`optional parentSpanId?: string;`

<!-- source: req\_meta.ts:134 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L134 -->

The span that initiated this span, if any.

##### parentTraceId?

`optional parentTraceId?: string;`

<!-- source: req\_meta.ts:129 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L129 -->

The trace id that initiated this trace, if any.

##### spanId

`spanId: string;`

<!-- source: req\_meta.ts:124 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L124 -->

The current span id.

##### traceId

`traceId: string;`

<!-- source: req\_meta.ts:122 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L122 -->

The trace id.

## Type Aliases

### CloudProvider

`type CloudProvider = "aws" | "gcp" | "azure" | "encore" | "local";`

<!-- source: app\_meta.ts:64 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L64 -->

Describes what cloud provider the application is running in.

***

### EnvironmentType

`type EnvironmentType = "production" | "development" | "ephemeral" | "test";`

<!-- source: app\_meta.ts:53 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L53 -->

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

<!-- source: req\_meta.ts:21 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L21 -->

***

### RequestMeta

`type RequestMeta = APICallMeta | PubSubMessageMeta & BaseRequestMeta;`

<!-- source: req\_meta.ts:151 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L151 -->

Describes an API call or Pub/Sub message being processed.

## Functions

### appMeta()

`function appMeta(): AppMeta;`

<!-- source: app\_meta.ts:100 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/app_meta.ts#L100 -->

Returns metadata about the running Encore application.

The metadata is cached and is the same object each call,
and therefore must not be modified by the caller.

#### Returns

[`AppMeta`](#appmeta)

***

### currentRequest()

`function currentRequest(): RequestMeta | undefined;`

<!-- source: req\_meta.ts:160 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/req_meta.ts#L160 -->

Returns information about the running Encore request,
such as API calls and Pub/Sub messages being processed.

Returns undefined only if no request is being processed,
such as during system initialization.

#### Returns

[`RequestMeta`](#requestmeta) \| `undefined`
