import { Decimal } from "../../types/mod";
import { Runtime } from "./napi/napi.cjs";

export * from "./napi/napi.cjs";

const testMode = process.env.NODE_ENV === "test";

export const RT = new Runtime({
  testMode,
  typeConstructors: {
    decimal: (val: string | number | bigint) => new Decimal(val)
  }
});

export interface Metric {
  name: string;
  services: string[];
}

export interface RuntimeConfig {
  metrics: Record<string, Metric>;
}

let cached: RuntimeConfig | null = null;
export function runtimeConfig(): RuntimeConfig {
  if (cached === null) {
    let cfg = RT.runtimeConfig();
    cached = {
      metrics: cfg.metrics
    };
  }
  return cached;
}
