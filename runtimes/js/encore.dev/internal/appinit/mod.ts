import { Gateway } from "../../api/gateway";
import { RawRequest, RawResponse } from "../api/node_http";
import { setCurrentRequest } from "../reqtrack/mod";
import * as runtime from "../runtime/mod";

export type Handler = runtime.ApiRoute;

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

function transformHandler(h: Handler): Handler {
  if (h.raw) {
    return {
      ...h,
      handler: (
        req: runtime.Request,
        resp: runtime.ResponseWriter,
        body: runtime.BodyReader
      ) => {
        setCurrentRequest(req);
        const rawReq = new RawRequest(req, body);
        const rawResp = new RawResponse(rawReq, resp);
        h.handler(rawReq, rawResp);
      },
    };
  }
  return {
    ...h,
    handler: (req: runtime.Request) => {
      setCurrentRequest(req);
      const payload = req.payload();
      return payload !== null ? h.handler(payload) : h.handler();
    },
  };
}
