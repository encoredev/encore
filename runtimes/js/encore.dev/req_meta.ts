import { getCurrentRequest } from "./internal/reqtrack/mod";

/** Describes an API endpoint. */
export interface APIDesc {
  /** The name of the service that the endpoint belongs to. */
  service: string;

  /** The name of the endpoint itself. */
  endpoint: string;

  /** Whether the endpoint is a raw endpoint. */
  raw: boolean;

  /** Whether the endpoint requires auth. */
  auth: boolean;

  /** Tags specified on the endpoint. */
  tags: string[];
}

export type Method =
  | "GET"
  | "POST"
  | "PUT"
  | "PATCH"
  | "DELETE"
  | "HEAD"
  | "OPTIONS"
  | "CONNECT"
  | "TRACE";

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

  /**
   * Contains values set in middlewares via `MiddlewareRequest.data`.
   */
  middlewareData?: Record<string, any>;
}

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

/** Provides information about the active trace. */
export interface TraceData {
  /** The trace id. */
  traceId: string;
  /** The current span id. */
  spanId: string;

  /**
   * The trace id that initiated this trace, if any.
   */
  parentTraceId?: string;

  /**
   * The span that initiated this span, if any.
   */
  parentSpanId?: string;

  /**
   * The external correlation id provided when the trace
   * was created, if any.
   * For example via the `Request-Id` or `X-Correlation-Id` headers.
   */
  extCorrelationId?: string;
}

interface BaseRequestMeta {
  /** Information about the trace, if the request is being traced */
  trace?: TraceData;
}

/** Describes an API call or Pub/Sub message being processed. */
export type RequestMeta = (APICallMeta | PubSubMessageMeta) & BaseRequestMeta;

/**
 * Returns information about the running Encore request,
 * such as API calls and Pub/Sub messages being processed.
 *
 * Returns undefined only if no request is being processed,
 * such as during system initialization.
 */
export function currentRequest(): RequestMeta | undefined {
  const req = getCurrentRequest();
  if (!req) {
    return undefined;
  }
  const meta = req.meta();

  const base: BaseRequestMeta = {
    trace: meta.trace
  };

  if (meta.apiCall) {
    const api: APICallMeta = {
      type: "api-call",
      api: {
        service: meta.apiCall.api.service,
        endpoint: meta.apiCall.api.endpoint,
        raw: meta.apiCall.api.raw,
        auth: meta.apiCall.api.requiresAuth,
        tags: meta.apiCall.api.tags
      },
      method: meta.apiCall.method as Method,
      path: meta.apiCall.path,
      pathAndQuery: meta.apiCall.pathAndQuery,
      pathParams: meta.apiCall.pathParams ?? {},
      parsedPayload: meta.apiCall.parsedPayload,
      headers: meta.apiCall.headers,
      middlewareData: (req as any).middlewareData
    };
    return { ...base, ...api };
  } else if (meta.pubsubMessage) {
    const msg: PubSubMessageMeta = {
      type: "pubsub-message",
      service: meta.pubsubMessage.service,
      topic: meta.pubsubMessage.topic,
      subscription: meta.pubsubMessage.subscription,
      messageId: meta.pubsubMessage.id,
      deliveryAttempt: meta.pubsubMessage.deliveryAttempt,
      parsedPayload: meta.pubsubMessage.parsedPayload
    };
    return { ...base, ...msg };
  } else {
    return undefined;
  }
}
