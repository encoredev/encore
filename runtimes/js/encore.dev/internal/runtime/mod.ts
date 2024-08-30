import { isMainThread } from "node:worker_threads";
import { Runtime } from "./napi/napi.cjs";

export * from "./napi/napi.cjs";

const testMode = process.env.NODE_ENV === "test";

export const RT = new Runtime({
  testMode,
  isWorker: !isMainThread && !testMode,
});
