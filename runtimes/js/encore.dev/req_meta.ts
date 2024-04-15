import { getCurrentRequest } from "./internal/reqtrack/mod";

export interface APIDesc {
  service: string;
  endpoint: string;
  raw: boolean;
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

export interface APICallMeta {
  type: "api-call";
  api: APIDesc;
  method: Method;
  path: string;
  pathAndQuery: string;
  pathParams: Record<string, any>;
  headers: Record<string, string | string[]>;
  parsedPayload?: Record<string, any>;
}

export interface PubSubMessageMeta {
  type: "pubsub-message";
  service: string;
  topic: string;
  subscription: string;
  messageId: string;
  deliveryAttempt: number;
  parsedPayload?: Record<string, any>;
}

export interface TraceData {
  traceId: string;
  spanId: string;
  parentTraceId?: string;
  parentSpanId?: string;
  extCorrelationId?: string;
}

interface BaseRequestMeta {
  trace?: TraceData;
}

export type RequestMeta = (APICallMeta | PubSubMessageMeta) & BaseRequestMeta;

// Returns metadata about the running Encore application.
//
// The metadata is cached and is the same object each call,
// and therefore must not be modified by the caller.
export function currentRequest(): RequestMeta | undefined {
  const req = getCurrentRequest();
  if (!req) {
    return undefined;
  }
  const meta = req.meta();

  const base: BaseRequestMeta = {
    trace: meta.trace,
  };

  if (meta.apiCall) {
    const api: APICallMeta = {
      type: "api-call",
      api: {
        service: meta.apiCall.api.service,
        endpoint: meta.apiCall.api.endpoint,
        raw: meta.apiCall.api.raw,
      },
      method: meta.apiCall.method as Method,
      path: meta.apiCall.path,
      pathAndQuery: meta.apiCall.pathAndQuery,
      pathParams: meta.apiCall.pathParams ?? {},
      parsedPayload: meta.apiCall.parsedPayload,
      headers: meta.apiCall.headers,
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
      parsedPayload: meta.pubsubMessage.parsedPayload,
    };
    return { ...base, ...msg };
  } else {
    return undefined;
  }
}
