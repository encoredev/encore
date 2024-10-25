import { APIError } from "../../api/error";
import { Gateway } from "../../api/gateway";
import { Middleware } from "../../api/mod";
import { APICallMeta, RequestMeta, currentRequest } from "../../req_meta";
import { RawRequest, RawResponse } from "../api/node_http";
import { setCurrentRequest } from "../reqtrack/mod";
import * as runtime from "../runtime/mod";

export type Handler = {
  apiRoute: runtime.ApiRoute;
  middlewares: Middleware[];
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
  req: RequestMeta,
  mws: Middleware[]
): Promise<any> {
  while (true) {
    const middleware = mws.shift();

    if (!middleware) {
      throw APIError.internal(
        "no middlewares to call, was the handler not added to the chain?"
      );
    }

    if (middleware.options) {
      const options = middleware.options;
      const apiMeta = req as APICallMeta;

      if (options.requiresAuth !== undefined) {
        if (options.requiresAuth !== apiMeta.api.requiresAuth) {
          continue;
        }
      }

      if (options.exposed !== undefined) {
        if (options.exposed !== apiMeta.api.exposed) {
          continue;
        }
      }
    }

    return middleware(req, async () => {
      return await invokeMiddlewareChain(req, [...mws]);
    });
  }
}

function transformHandler(h: Handler): runtime.ApiRoute {
  if (h.apiRoute.streamingResponse || h.apiRoute.streamingRequest) {
    return {
      ...h.apiRoute,
      // req is the upgrade request.
      // stream is either a bidirectional stream, in stream or out stream.
      handler: (req: runtime.Request, stream: unknown) => {
        setCurrentRequest(req);
        const cur = currentRequest() as RequestMeta;

        // make readable streams async iterators
        if (stream instanceof runtime.Stream) {
          stream = new IterableStream(stream);
        }
        if (stream instanceof runtime.Socket) {
          stream = new IterableSocket(stream);
        }

        const handler = async () => {
          // handshake payload
          const payload = req.payload();
          return payload !== null
            ? h.apiRoute.handler(payload, stream)
            : h.apiRoute.handler(stream);
        };
        return invokeMiddlewareChain(cur, [...h.middlewares, handler]);
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
        const cur = currentRequest() as RequestMeta;

        const rawReq = new RawRequest(req, body);
        const rawResp = new RawResponse(rawReq, resp);

        const handler = async () => {
          return h.apiRoute.handler(rawReq, rawResp);
        };
        return invokeMiddlewareChain(cur, [...h.middlewares, handler]);
      }
    };
  }

  return {
    ...h.apiRoute,
    handler: (req: runtime.Request) => {
      setCurrentRequest(req);
      const cur = currentRequest() as RequestMeta;

      const handler = async () => {
        const payload = req.payload();
        return payload !== null
          ? h.apiRoute.handler(payload)
          : h.apiRoute.handler();
      };
      return invokeMiddlewareChain(cur, [...h.middlewares, handler]);
    }
  };
}
