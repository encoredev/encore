/**
 * Custom metrics for Encore applications.
 *
 * This module provides counters and gauges that can be statically analyzed
 * by the Encore compiler and automatically exported to observability backends.
 *
 * @example Simple counter
 * ```typescript
 * import { Counter } from 'encore.dev/metrics';
 *
 * export const ordersProcessed = new Counter("orders_processed");
 *
 * ordersProcessed.increment();
 * ```
 *
 * @example Counter with labels
 * ```typescript
 * import { CounterGroup } from 'encore.dev/metrics';
 *
 * interface Labels {
 *   success: boolean;
 * }
 *
 * export const ordersProcessed = new CounterGroup<Labels>("orders_processed");
 *
 * ordersProcessed.with({ success: true }).increment();
 * ```
 */

import * as runtime from "../internal/runtime/mod";
import { MetricType } from "../internal/runtime/mod";
import { getRegistry, getBuffer } from "../internal/metrics/registry";
import {
  AtomicCounter,
  AtomicGauge,
  processLabelsToPairs,
  serializeLabels
} from "../internal/metrics/mod";
import { currentRequest } from "../req_meta";

export interface MetricConfig {}

/**
 * Resolves the service name for a metric by checking:
 * 1. If there's only one service using this metric in the runtime config
 * 2. Otherwise, looks at the current request context
 */
function resolveServiceName(metricName: string): string | undefined {
  const rtConfig = runtime.runtimeConfig();
  const rtSvcs = rtConfig.metrics[metricName]?.services ?? [];
  if (rtSvcs.length === 1) {
    return rtSvcs[0];
  }

  const currReq = currentRequest();
  if (currReq) {
    if (currReq.type === "api-call") {
      return currReq.api.service;
    } else {
      return currReq.service;
    }
  }

  return undefined;
}

/**
 * A Counter tracks cumulative values that only increase.
 * Use counters for metrics like request counts, errors, etc.
 */
export class Counter {
  private name: string;
  private cache: Map<string, AtomicCounter>;
  private labelPairs: [string, string][];
  private cfg: MetricConfig;

  constructor(name: string, cfg?: MetricConfig) {
    this.name = name;
    this.cfg = cfg ?? {};
    this.labelPairs = [];
    this.cache = new Map();
  }

  /**
   * Increment the counter by the given value (default 1).
   */
  increment(value: number = 1): void {
    const serviceName = resolveServiceName(this.name);
    if (!serviceName) {
      return;
    }

    let metric = this.cache.get(serviceName);
    if (!metric) {
      const registry = getRegistry();
      const buffer = getBuffer();

      // If registry or buffer are not initialized, silently skip
      if (!registry || !buffer) {
        return;
      }

      const slot = registry.allocateSlot(
        this.name,
        this.labelPairs,
        serviceName,
        MetricType.Counter
      );
      metric = new AtomicCounter(buffer, slot);
      this.cache.set(serviceName, metric);
    }

    metric.increment(value);
  }

  ref(): Counter {
    return this;
  }
}

/**
 * A CounterGroup tracks counters with labels.
 * Each unique combination of label values creates a separate counter time series.
 *
 * @typeParam L - The label interface (must have string/number/boolean fields)
 * Note: Number values in labels are converted to integers using Math.floor().
 */
export class CounterGroup<
  L extends Record<keyof L, string | number | boolean>
> {
  private name: string;
  private labelCache: Map<string, Counter>;
  private cfg: MetricConfig;

  constructor(name: string, cfg?: MetricConfig) {
    this.name = name;
    this.labelCache = new Map();
    this.cfg = cfg ?? {};
  }

  /**
   * Get a counter for the given label values.
   *
   * Note: Number values in labels are converted to integers using Math.floor().
   */
  with(labels: L): Counter {
    const labelKey = serializeLabels(labels);

    let cached = this.labelCache.get(labelKey);
    if (!cached) {
      // Create counter instance
      cached = new Counter(this.name, this.cfg);

      const labelPairs = processLabelsToPairs(labels);
      (cached as any).labelPairs = labelPairs;

      this.labelCache.set(labelKey, cached);
    }

    return cached;
  }

  ref(): CounterGroup<L> {
    return this;
  }
}

/**
 * A Gauge tracks values that can go up or down.
 * Use gauges for metrics like memory usage, active connections, temperature, etc.
 */
export class Gauge {
  private name: string;
  private cache: Map<string, AtomicGauge>;
  private labelPairs: [string, string][];
  private cfg: MetricConfig;

  constructor(name: string, cfg?: MetricConfig) {
    this.name = name;
    this.cfg = cfg ?? {};
    this.labelPairs = [];
    this.cache = new Map();
  }

  /**
   * Set the gauge to the given value.
   */
  set(value: number): void {
    const serviceName = resolveServiceName(this.name);
    if (!serviceName) {
      return;
    }

    let metric = this.cache.get(serviceName);
    if (!metric) {
      const registry = getRegistry();
      const buffer = getBuffer();

      // If registry or buffer are not initialized, silently skip
      if (!registry || !buffer) {
        return;
      }

      const slot = registry.allocateSlot(
        this.name,
        this.labelPairs,
        serviceName,
        MetricType.Gauge
      );
      metric = new AtomicGauge(buffer, slot);
      this.cache.set(serviceName, metric);
    }

    metric.set(value);
  }

  ref(): Gauge {
    return this;
  }
}

export class GaugeGroup<L extends Record<keyof L, string | number | boolean>> {
  private name: string;
  private labelCache: Map<string, Gauge>;
  private cfg: MetricConfig;

  constructor(name: string, cfg?: MetricConfig) {
    this.name = name;
    this.labelCache = new Map();
    this.cfg = cfg ?? {};
  }

  /**
   * Get a gauge for the given label values.
   *
   * Note: Number values in labels are converted to integers using Math.floor().
   */
  with(labels: L): Gauge {
    const labelKey = serializeLabels(labels);

    let cached = this.labelCache.get(labelKey);
    if (!cached) {
      // Create gauge instance
      cached = new Gauge(this.name, this.cfg);

      const labelPairs = processLabelsToPairs(labels);
      (cached as any).labelPairs = labelPairs;

      this.labelCache.set(labelKey, cached);
    }

    return cached;
  }

  ref(): GaugeGroup<L> {
    return this;
  }
}
