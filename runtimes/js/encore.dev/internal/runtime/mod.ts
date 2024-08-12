import { isMainThread } from "node:worker_threads";
import { Runtime } from "./napi/napi.cjs";

export * from "./napi/napi.cjs";

export const RT = new Runtime({
  testMode: process.env.NODE_ENV === "test",
  isWorker: !isMainThread,
});
