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

export interface MetricConfig {}

/**
 * A Counter tracks cumulative values that only increase.
 * Use counters for metrics like request counts, errors, etc.
 */
export class Counter {
  private name: string;
  private slot: number | undefined;
  private metric: AtomicCounter | undefined;
  private serviceName: string | undefined;
  private labelPairs: [string, string][];
  private cfg: MetricConfig;

  constructor(name: string, cfg?: MetricConfig) {
    this.name = name;
    this.cfg = cfg ?? {};
    this.labelPairs = [];
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

  private get registry(): runtime.MetricsRegistry | undefined {
    return getRegistry();
  }

  private get buffer(): SharedArrayBuffer | undefined {
    return getBuffer();
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
        this.labelPairs,
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
 * Note: Number values in labels are converted to integers using Math.floor().
 */
export class CounterGroup<
  L extends Record<keyof L, string | number | boolean>
> {
  private name: string;
  private labelCache: Map<string, Counter>;
  private serviceName: string | undefined;
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

      if (this.serviceName) {
        cached.__internalSetServiceName(this.serviceName);
      }

      const labelPairs = processLabelsToPairs(labels);
      (cached as any).labelPairs = labelPairs;

      this.labelCache.set(labelKey, cached);
    }

    return cached;
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
  private labelPairs: [string, string][];
  private cfg: MetricConfig;

  constructor(name: string, cfg?: MetricConfig) {
    this.name = name;
    this.cfg = cfg ?? {};
    this.labelPairs = [];
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
        this.labelPairs,
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
 * Note: Number values in labels are converted to integers using Math.floor().
 */
export class GaugeGroup<L extends Record<keyof L, string | number | boolean>> {
  private name: string;
  private labelCache: Map<string, Gauge>;
  private serviceName: string | undefined;
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

      if (this.serviceName) {
        cached.__internalSetServiceName(this.serviceName);
      }

      const labelPairs = processLabelsToPairs(labels);
      (cached as any).labelPairs = labelPairs;

      this.labelCache.set(labelKey, cached);
    }

    return cached;
  }

  /**
   * Internal method called by generated code to associate this gauge group with a service.
   * @internal
   */
  __internalSetServiceName(serviceName: string): void {
    this.serviceName = serviceName;
  }
}
