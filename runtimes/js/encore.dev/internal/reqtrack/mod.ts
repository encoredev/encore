import * as runtime from "../runtime/mod";
import { AsyncLocalStorage } from "node:async_hooks";

const asyncLocalStorage = new AsyncLocalStorage();

export function setCurrentRequest(req: runtime.Request) {
  asyncLocalStorage.enterWith(req);
}

export function getCurrentRequest(): runtime.Request | null {
  return (asyncLocalStorage.getStore() as runtime.Request) ?? null;
}
