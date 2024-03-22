import * as runtime from "../runtime/mod";
import { setCurrentRequest } from "../reqtrack/mod";
import { Gateway } from "../../api/gateway";

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
  return {
    ...h,
    handler: (req: runtime.Request) => {
      setCurrentRequest(req);
      const payload = req.payload();
      return payload !== null ? h.handler(payload) : h.handler();
    },
  };
}
