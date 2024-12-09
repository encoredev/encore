import { Gateway } from "../../api/gateway";
import { Middleware, MiddlewareRequest, HandlerResponse } from "../../api/mod";
import { IterableSocket, IterableStream, Sink } from "../../api/stream";
import { RawRequest, RawResponse } from "../api/node_http";
import { setCurrentRequest } from "../reqtrack/mod";
import * as runtime from "../runtime/mod";

export type Handler = {
  apiRoute: runtime.ApiRoute;
  middlewares: Middleware[];
  endpointOptions: EndpointOptions;
};

export function registerHandlers(handlers: Handler[]) {
  runtime.RT.registerHandlers(handlers.map((h) => transformHandler(h)));
}

export function registerTestHandler(handler: Handler) {
  runtime.RT.registerTestHandler(transformHandler(handler));
}

export function registerGateways(gateways: Gateway[]) {
  // This function exists to ensure gateways are imported and executed.
  // It intentionally doesn't need to do anything.
}

export async function run() {
  return runtime.RT.runForever();
}

interface EndpointOptions {
  expose: boolean;
  auth: boolean;
  isRaw: boolean;
  isStream: boolean;
}

export interface InternalHandlerResponse {
  payload: any;
  extraHeaders?: Record<string, string | string[]>;
}

// recursively calls all middlewares
async function invokeMiddlewareChain(
  req: MiddlewareRequest,
  chain: Middleware[],
  handler: () => Promise<any>
): Promise<InternalHandlerResponse> {
  const execute = async (
    index: number,
    req: MiddlewareRequest
  ): Promise<HandlerResponse> => {
    const currentMiddleware = chain.at(index);

    // no more middlewares, execute the handler
    if (currentMiddleware === undefined) {
      return new HandlerResponse(await handler());
    }

    // execute current middleware
    return currentMiddleware(req, (req) => {
      return execute(index + 1, req);
    });
  };

  return (await execute(0, req)).__internalToResponse();
}

// calculate what middlewares should run for an endpoint
function calculateMiddlewareChain(
  endpointOptions: EndpointOptions,
  ms: Middleware[]
): Middleware[] {
  let middlewares = [];

  for (const m of ms) {
    if (m.options === undefined || m.options.target === undefined) {
      middlewares.push(m);
    } else {
      const target = m.options.target;
      // check if options are set and if they match the endpoint options
      if (target.auth !== undefined && target.auth !== endpointOptions.auth) {
        continue;
      }

      if (
        target.expose !== undefined &&
        target.expose !== endpointOptions.expose
      ) {
        continue;
      }

      if (
        target.isRaw !== undefined &&
        target.isRaw !== endpointOptions.isRaw
      ) {
        continue;
      }

      if (
        target.isStream !== undefined &&
        target.isStream !== endpointOptions.isStream
      ) {
        continue;
      }

      middlewares.push(m);
    }
  }

  return middlewares;
}

function transformHandler(h: Handler): runtime.ApiRoute {
  const middlewares = calculateMiddlewareChain(
    h.endpointOptions,
    h.middlewares
  );

  if (h.apiRoute.streamingResponse || h.apiRoute.streamingRequest) {
    return {
      ...h.apiRoute,
      // req is the upgrade request.
      // stream is either a bidirectional stream, in stream or out stream.
      handler: (
        req: runtime.Request,
        stream: runtime.Sink | runtime.Stream | runtime.Socket
      ) => {
        setCurrentRequest(req);

        // make readable streams async iterators
        const streamArg: IterableStream | IterableSocket | Sink =
          stream instanceof runtime.Stream
            ? new IterableStream(stream)
            : stream instanceof runtime.Socket
              ? new IterableSocket(stream)
              : new Sink(stream);

        if (middlewares.length === 0) {
          const payload = req.payload();
          return toResponse(
            payload !== null
              ? h.apiRoute.handler(payload, streamArg)
              : h.apiRoute.handler(streamArg)
          );
        }

        const handler = async () => {
          // handshake payload
          const payload = req.payload();
          return payload !== null
            ? h.apiRoute.handler(payload, streamArg)
            : h.apiRoute.handler(streamArg);
        };

        const mwRequest = new MiddlewareRequest(
          streamArg,
          undefined,
          undefined
        );
        return invokeMiddlewareChain(mwRequest, middlewares, handler);
      }
    };
  }

  if (h.apiRoute.raw) {
    return {
      ...h.apiRoute,
      handler: (
        req: runtime.Request,
        resp: runtime.ResponseWriter,
        body: runtime.BodyReader
      ) => {
        setCurrentRequest(req);

        const rawReq = new RawRequest(req, body);
        const rawResp = new RawResponse(rawReq, resp);

        if (middlewares.length === 0) {
          return toResponse(h.apiRoute.handler(rawReq, rawResp));
        }

        const handler = async () => {
          return h.apiRoute.handler(rawReq, rawResp);
        };

        const mwRequest = new MiddlewareRequest(undefined, rawReq, rawResp);
        return invokeMiddlewareChain(mwRequest, middlewares, handler);
      }
    };
  }

  return {
    ...h.apiRoute,
    handler: (req: runtime.Request) => {
      setCurrentRequest(req);

      if (middlewares.length === 0) {
        const payload = req.payload();
        return toResponse(
          payload !== null ? h.apiRoute.handler(payload) : h.apiRoute.handler()
        );
      }

      const handler = async () => {
        const payload = req.payload();
        return payload !== null
          ? h.apiRoute.handler(payload)
          : h.apiRoute.handler();
      };

      const mwRequest = new MiddlewareRequest(undefined, undefined, undefined);
      return invokeMiddlewareChain(mwRequest, middlewares, handler);
    }
  };
}

function toResponse(
  payload: any
): InternalHandlerResponse | Promise<InternalHandlerResponse> {
  if (payload instanceof Promise) {
    return payload.then((payload) => {
      return new HandlerResponse(payload).__internalToResponse();
    });
  } else {
    return new HandlerResponse(payload).__internalToResponse();
  }
}
