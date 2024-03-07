import * as runtime from "../runtime/mod";
import { executionAsyncResource, createHook } from "node:async_hooks";

// Private symbol to avoid pollution
const sym = Symbol("request");

createHook({
  init(asyncId: any, type: any, triggerAsyncId: any, resource: any) {
    const cr: any = executionAsyncResource();
    if (cr) {
      resource[sym] = cr[sym];
    }
  },
}).enable();

export function setCurrentRequest(req: runtime.Request) {
  (executionAsyncResource() as any)[sym] = req;
}

export function getCurrentRequest(): runtime.Request | null {
  return (executionAsyncResource() as any)[sym] ?? null;
}
