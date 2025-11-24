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

export interface MetricConfig {}

/**
 * Serialize labels to a consistent string key for map lookups.
 * @internal
 */
function serializeLabels(
  labels: Record<string, string | number | boolean>
): string {
  const sorted = Object.entries(labels).sort(([a], [b]) => a.localeCompare(b));
  return JSON.stringify(sorted);
}

/**
 * Internal class that handles atomic counter operations on SharedArrayBuffer.
 */
class AtomicCounter {
  private view: BigUint64Array;
  private slot: number;

  constructor(buffer: SharedArrayBuffer, slot: number) {
    this.view = new BigUint64Array(buffer);
    this.slot = slot;
  }

  increment(value: number = 1): void {
    const v = BigInt(Math.floor(value));
    Atomics.add(this.view, this.slot, v);
  }
}

/**
 * No-op counter for when metrics are not initialized (e.g., in tests).
 * @internal
 */
class NoOpCounter {
  increment(_value: number = 1): void {
    // No-op
  }
}

/**
 * Internal class that handles atomic gauge operations on SharedArrayBuffer.
 */
class AtomicGauge {
  private view: BigUint64Array;
  private slot: number;

  constructor(buffer: SharedArrayBuffer, slot: number) {
    this.view = new BigUint64Array(buffer);
    this.slot = slot;
  }

  set(value: number): void {
    // For gauges, store f64 bits as u64
    const float64 = new Float64Array(1);
    float64[0] = value;
    const uint64View = new BigUint64Array(float64.buffer);
    const v = uint64View[0];
    Atomics.store(this.view, this.slot, v);
  }
}

/**
 * No-op gauge for when metrics are not initialized (e.g., in tests).
 * @internal
 */
class NoOpGauge {
  set(_value: number): void {
    // No-op
  }
}

/**
 * A Counter tracks cumulative values that only increase.
 * Use counters for metrics like request counts, errors, etc.
 */
export class Counter {
  private name: string;
  private slot: number | undefined;
  private metric: AtomicCounter | undefined;
  private serviceName: string | undefined;
  private cfg: MetricConfig;

  constructor(name: string, cfg?: MetricConfig) {
    this.name = name;
    this.cfg = cfg ?? {};
  }

  private get registry(): runtime.MetricsRegistry | undefined {
    return getRegistry();
  }

  private get buffer(): SharedArrayBuffer | undefined {
    return getBuffer();
  }

  /**
   * Increment the counter by the given value (default 1).
   */
  increment(value: number = 1): void {
    if (!this.metric) {
      this.ensureInitialized();
    }
    this.metric?.increment(value);
  }

  private ensureInitialized(): void {
    const registry = this.registry;
    const buffer = this.buffer;

    // If registry or buffer are not initialized, silently skip
    if (!registry || !buffer) {
      return;
    }

    if (this.slot === undefined) {
      // Allocate slot for this metric with service name
      this.slot = registry.allocateSlot(
        this.name,
        [],
        this.serviceName,
        MetricType.Counter
      );
    }
    if (!this.metric) {
      this.metric = new AtomicCounter(buffer, this.slot);
    }
  }

  /**
   * Internal method called by generated code to associate this counter with a service.
   * @internal
   */
  __internalSetServiceName(serviceName: string): void {
    this.serviceName = serviceName;
  }
}

/**
 * A CounterGroup tracks counters with labels.
 * Each unique combination of label values creates a separate counter time series.
 *
 * @typeParam L - The label interface (must have string/number/boolean fields)
 */
export class CounterGroup<
  L extends Record<keyof L, string | number | boolean>
> {
  private name: string;
  private labelCache: Map<string, { slot: number; metric: AtomicCounter }>;
  private serviceName: string | undefined;
  private cfg: MetricConfig;

  constructor(name: string, cfg?: MetricConfig) {
    this.name = name;
    this.labelCache = new Map();
    this.cfg = cfg ?? {};
  }

  private get registry(): runtime.MetricsRegistry | undefined {
    return getRegistry();
  }

  private get buffer(): SharedArrayBuffer | undefined {
    return getBuffer();
  }

  /**
   * Get a counter for the given label values.
   *
   * Note: Number values in labels are converted to integers using Math.floor().
   */
  with(labels: L): AtomicCounter | NoOpCounter {
    const labelKey = serializeLabels(labels);

    let cached = this.labelCache.get(labelKey);
    if (!cached) {
      const registry = this.registry;
      const buffer = this.buffer;

      // If registry or buffer are not initialized, return no-op counter
      if (!registry || !buffer) {
        return new NoOpCounter();
      }

      // Allocate slot for this label combination
      const labelMap: Record<string, string> = {};
      for (const [key, value] of Object.entries(labels)) {
        if (typeof value === "number") {
          labelMap[key] = String(Math.floor(value));
        } else {
          labelMap[key] = String(value);
        }
      }

      const labelPairs = Object.entries(labelMap);
      const slot = registry.allocateSlot(
        this.name,
        labelPairs,
        this.serviceName,
        MetricType.Counter
      );

      const metric = new AtomicCounter(buffer, slot);
      cached = { slot, metric };
      this.labelCache.set(labelKey, cached);
    }

    return cached.metric;
  }

  /**
   * Internal method called by generated code to associate this counter group with a service.
   * @internal
   */
  __internalSetServiceName(serviceName: string): void {
    this.serviceName = serviceName;
  }
}

/**
 * A Gauge tracks values that can go up or down.
 * Use gauges for metrics like memory usage, active connections, temperature, etc.
 */
export class Gauge {
  private name: string;
  private slot: number | undefined;
  private metric: AtomicGauge | undefined;
  private serviceName: string | undefined;
  private cfg: MetricConfig;

  constructor(name: string, cfg?: MetricConfig) {
    this.name = name;
    this.cfg = cfg ?? {};
  }

  private get registry(): runtime.MetricsRegistry | undefined {
    return getRegistry();
  }

  private get buffer(): SharedArrayBuffer | undefined {
    return getBuffer();
  }

  /**
   * Set the gauge to the given value.
   */
  set(value: number): void {
    if (!this.metric) {
      this.ensureInitialized();
    }
    this.metric?.set(value);
  }

  private ensureInitialized(): void {
    const registry = this.registry;
    const buffer = this.buffer;

    // If registry or buffer are not initialized, silently skip
    if (!registry || !buffer) {
      return;
    }

    if (this.slot === undefined) {
      // Allocate slot for this metric with service name
      this.slot = registry.allocateSlot(
        this.name,
        [],
        this.serviceName,
        MetricType.Gauge
      );
    }
    if (!this.metric) {
      this.metric = new AtomicGauge(buffer, this.slot);
    }
  }

  /**
   * Internal method called by generated code to associate this gauge with a service.
   * @internal
   */
  __internalSetServiceName(serviceName: string): void {
    this.serviceName = serviceName;
  }
}

/**
 * A GaugeGroup tracks gauges with labels.
 * Each unique combination of label values creates a separate gauge time series.
 *
 * @typeParam L - The label interface (must have string/number/boolean fields)
 */
export class GaugeGroup<L extends Record<keyof L, string | number | boolean>> {
  private name: string;
  private labelCache: Map<string, { slot: number; metric: AtomicGauge }>;
  private serviceName: string | undefined;
  private cfg: MetricConfig;

  constructor(name: string, cfg?: MetricConfig) {
    this.name = name;
    this.labelCache = new Map();
    this.cfg = cfg ?? {};
  }

  private get registry(): runtime.MetricsRegistry | undefined {
    return getRegistry();
  }

  private get buffer(): SharedArrayBuffer | undefined {
    return getBuffer();
  }

  /**
   * Get a gauge for the given label values.
   *
   * Note: Number values in labels are converted to integers using Math.floor().
   */
  with(labels: L): AtomicGauge | NoOpGauge {
    const labelKey = serializeLabels(labels);

    let cached = this.labelCache.get(labelKey);
    if (!cached) {
      const registry = this.registry;
      const buffer = this.buffer;

      // If registry or buffer are not initialized, return no-op gauge
      if (!registry || !buffer) {
        return new NoOpGauge();
      }

      // Allocate slot for this label combination
      const labelMap: Record<string, string> = {};
      for (const [key, value] of Object.entries(labels)) {
        if (typeof value === "number") {
          labelMap[key] = String(Math.floor(value));
        } else {
          labelMap[key] = String(value);
        }
      }

      const labelPairs = Object.entries(labelMap);
      const slot = registry.allocateSlot(
        this.name,
        labelPairs,
        this.serviceName,
        MetricType.Gauge
      );

      const metric = new AtomicGauge(buffer, slot);
      cached = { slot, metric };
      this.labelCache.set(labelKey, cached);
    }

    return cached.metric;
  }

  /**
   * Internal method called by generated code to associate this gauge group with a service.
   * @internal
   */
  __internalSetServiceName(serviceName: string): void {
    this.serviceName = serviceName;
  }
}
