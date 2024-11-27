import { Gateway } from "../../api/gateway";
import { Middleware, MiddlewareRequest, HandlerResponse } from "../../api/mod";
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
  requiresAuth: boolean;
  exposed: boolean;
  isRaw: boolean;
  isStream: boolean;
}

class IterableStream {
  private stream: runtime.Stream;

  constructor(stream: runtime.Stream) {
    this.stream = stream;
  }

  recv(): Promise<Record<string, any>> {
    return this.stream.recv();
  }

  async *[Symbol.asyncIterator]() {
    while (true) {
      try {
        yield await this.stream.recv();
      } catch (e) {
        break;
      }
    }
  }
}

class IterableSocket {
  private socket: runtime.Socket;

  constructor(socket: runtime.Socket) {
    this.socket = socket;
  }

  send(msg: Record<string, any>): void {
    return this.socket.send(msg);
  }
  recv(): Promise<Record<string, any>> {
    return this.socket.recv();
  }

  close(): void {
    this.socket.close();
  }

  async *[Symbol.asyncIterator]() {
    while (true) {
      try {
        yield await this.socket.recv();
      } catch (e) {
        break;
      }
    }
  }
}

// recursively calls all middlewares
function invokeMiddlewareChain(
  req: MiddlewareRequest,
  chain: Middleware[],
  handler: () => Promise<any>
): Promise<any> {
  const execute = async (index: number): Promise<HandlerResponse> => {
    const currentMiddleware = chain.at(index);

    // no more middlewares, execute the handler
    if (currentMiddleware === undefined) {
      return new HandlerResponse(await handler());
    }

    // execute current middleare middleware
    return currentMiddleware(req, () => {
      return execute(index + 1);
    });
  };

  return execute(0);
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
      if (
        target.requiresAuth !== undefined &&
        target.requiresAuth !== endpointOptions.requiresAuth
      ) {
        continue;
      }

      if (
        target.exposed !== undefined &&
        target.exposed !== endpointOptions.exposed
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
        if (stream instanceof runtime.Stream) {
          stream = new IterableStream(stream);
        }
        if (stream instanceof runtime.Socket) {
          stream = new IterableSocket(stream);
        }

        if (middlewares.length === 0) {
          const payload = req.payload();
          return payload !== null
            ? h.apiRoute.handler(payload, stream)
            : h.apiRoute.handler(stream);
        }

        const handler = async () => {
          // handshake payload
          const payload = req.payload();
          return payload !== null
            ? h.apiRoute.handler(payload, stream)
            : h.apiRoute.handler(stream);
        };

        const mwRequest = new MiddlewareRequest(stream, undefined, undefined);
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
          return h.apiRoute.handler(rawReq, rawResp);
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
        return payload !== null
          ? h.apiRoute.handler(payload)
          : h.apiRoute.handler();
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
