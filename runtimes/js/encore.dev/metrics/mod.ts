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
 * const OrdersProcessed = new Counter<number>("orders_processed", {});
 *
 * OrdersProcessed.increment();
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
 * const OrdersProcessed = new CounterGroup<Labels, number>("orders_processed", {});
 *
 * OrdersProcessed.with({ success: true }).increment();
 * ```
 */

import { RT, MetricType } from "../internal/runtime/mod";
import type { MetricsRegistry as NativeMetricsRegistry } from "../internal/runtime/mod";

// Internal registry management
let globalRegistry: NativeMetricsRegistry | undefined;
let globalBuffer: SharedArrayBuffer | undefined;

function getOrCreateRegistry(): NativeMetricsRegistry {
  if (!globalRegistry) {
    const INITIAL_SLOTS = 10_000;
    globalBuffer = new SharedArrayBuffer(INITIAL_SLOTS * 8);
    globalRegistry = RT.createMetricsRegistry(globalBuffer);
  }
  return globalRegistry;
}

function getBuffer(): SharedArrayBuffer {
  if (!globalBuffer) {
    getOrCreateRegistry();
  }
  return globalBuffer!;
}

/**
 * Configuration for counters and gauges.
 * Currently empty, reserved for future configuration options.
 */
export interface MetricConfig {}

/**
 * Internal class that handles atomic operations on SharedArrayBuffer.
 */
class AtomicMetric {
  private view: BigUint64Array;
  private slot: number;

  constructor(buffer: SharedArrayBuffer, slot: number) {
    this.view = new BigUint64Array(buffer);
    this.slot = slot;
  }

  increment(value: number | bigint = 1): void {
    const v = typeof value === "bigint" ? value : BigInt(value);
    Atomics.add(this.view, this.slot, v);
  }

  set(value: number | bigint): void {
    const v = typeof value === "bigint" ? value : BigInt(value);
    Atomics.store(this.view, this.slot, v);
  }

  get(): bigint {
    return Atomics.load(this.view, this.slot);
  }
}

/**
 * A Counter tracks cumulative values that only increase.
 * Use counters for metrics like request counts, errors, etc.
 *
 * @typeParam T - The numeric type for the counter value (number or bigint)
 */
export class Counter<T extends number | bigint = number> {
  private registry: NativeMetricsRegistry;
  private buffer: SharedArrayBuffer;
  private name: string;
  private slot: number | undefined;
  private metric: AtomicMetric | undefined;

  constructor(name: string, config: MetricConfig) {
    this.registry = getOrCreateRegistry();
    this.buffer = getBuffer();
    this.name = name;
    // Note: config is for parser analysis, not used at runtime yet
  }

  /**
   * Increment the counter by the given value (default 1).
   * This operation is atomic and has ~5-10ns overhead.
   */
  increment(value?: T): void {
    if (!this.metric) {
      this.ensureInitialized();
    }
    this.metric!.increment(value === undefined ? 1 : value);
  }

  /**
   * Get the current value (primarily for testing).
   */
  get(): bigint {
    if (!this.metric) {
      this.ensureInitialized();
    }
    return this.metric!.get();
  }

  private ensureInitialized(): void {
    if (this.slot === undefined) {
      // Allocate slot for this metric
      this.slot = this.registry.allocateSlot(this.name, {}, MetricType.Counter);
    }
    if (!this.metric) {
      this.metric = new AtomicMetric(this.buffer, this.slot);
    }
  }
}

/**
 * A CounterGroup tracks counters with labels.
 * Each unique combination of label values creates a separate counter time series.
 *
 * @typeParam L - The label interface (must have string/number/boolean fields)
 * @typeParam T - The numeric type for the counter value (number or bigint)
 */
export class CounterGroup<
  L extends Record<string, string | number | boolean>,
  T extends number | bigint = number
> {
  private registry: NativeMetricsRegistry;
  private buffer: SharedArrayBuffer;
  private name: string;
  private slots: Map<string, number>;
  private metrics: Map<string, AtomicMetric>;

  constructor(name: string, config: MetricConfig) {
    this.registry = getOrCreateRegistry();
    this.buffer = getBuffer();
    this.name = name;
    this.slots = new Map();
    this.metrics = new Map();
    // Note: config is for parser analysis, not used at runtime yet
  }

  /**
   * Get a counter for the given label values.
   * First call with unique labels allocates a slot (~1-10μs).
   * Subsequent calls use cached slot (~50ns lookup).
   */
  with(labels: L): CounterInstance<T> {
    const labelKey = this.serializeLabels(labels);

    let metric = this.metrics.get(labelKey);
    if (!metric) {
      // Allocate slot for this label combination
      let slot = this.slots.get(labelKey);
      if (slot === undefined) {
        const labelMap: Record<string, string> = {};
        for (const [key, value] of Object.entries(labels)) {
          labelMap[key] = String(value);
        }
        slot = this.registry.allocateSlot(
          this.name,
          labelMap,
          MetricType.Counter
        );
        this.slots.set(labelKey, slot);
      }
      metric = new AtomicMetric(this.buffer, slot);
      this.metrics.set(labelKey, metric);
    }

    return new CounterInstance(metric);
  }

  private serializeLabels(labels: L): string {
    const sorted = Object.entries(labels).sort(([a], [b]) =>
      a.localeCompare(b)
    );
    return JSON.stringify(sorted);
  }
}

/**
 * A counter instance with specific label values.
 */
class CounterInstance<T extends number | bigint> {
  constructor(private metric: AtomicMetric) {}

  increment(value?: T): void {
    this.metric.increment(value === undefined ? 1 : value);
  }

  get(): bigint {
    return this.metric.get();
  }
}

/**
 * A Gauge tracks values that can go up or down.
 * Use gauges for metrics like memory usage, active connections, temperature, etc.
 *
 * @typeParam T - The numeric type for the gauge value (number or bigint)
 */
export class Gauge<T extends number | bigint = number> {
  private registry: NativeMetricsRegistry;
  private buffer: SharedArrayBuffer;
  private name: string;
  private slot: number | undefined;
  private metric: AtomicMetric | undefined;

  constructor(name: string, config: MetricConfig) {
    this.registry = getOrCreateRegistry();
    this.buffer = getBuffer();
    this.name = name;
    // Note: config is for parser analysis, not used at runtime yet
  }

  /**
   * Set the gauge to the given value.
   * This operation is atomic and has ~5-10ns overhead.
   */
  set(value: T): void {
    if (!this.metric) {
      this.ensureInitialized();
    }
    this.metric!.set(value);
  }

  /**
   * Get the current value (primarily for testing).
   */
  get(): bigint {
    if (!this.metric) {
      this.ensureInitialized();
    }
    return this.metric!.get();
  }

  private ensureInitialized(): void {
    if (this.slot === undefined) {
      const metricType =
        typeof (0 as T) === "bigint"
          ? MetricType.GaugeInt
          : MetricType.GaugeInt;
      this.slot = this.registry.allocateSlot(this.name, {}, metricType);
    }
    if (!this.metric) {
      this.metric = new AtomicMetric(this.buffer, this.slot);
    }
  }
}

/**
 * A GaugeGroup tracks gauges with labels.
 * Each unique combination of label values creates a separate gauge time series.
 *
 * @typeParam L - The label interface (must have string/number/boolean fields)
 * @typeParam T - The numeric type for the gauge value (number or bigint)
 */
export class GaugeGroup<
  L extends Record<string, string | number | boolean>,
  T extends number | bigint = number
> {
  private registry: NativeMetricsRegistry;
  private buffer: SharedArrayBuffer;
  private name: string;
  private slots: Map<string, number>;
  private metrics: Map<string, AtomicMetric>;

  constructor(name: string, config: MetricConfig) {
    this.registry = getOrCreateRegistry();
    this.buffer = getBuffer();
    this.name = name;
    this.slots = new Map();
    this.metrics = new Map();
    // Note: config is for parser analysis, not used at runtime yet
  }

  /**
   * Get a gauge for the given label values.
   * First call with unique labels allocates a slot (~1-10μs).
   * Subsequent calls use cached slot (~50ns lookup).
   */
  with(labels: L): GaugeInstance<T> {
    const labelKey = this.serializeLabels(labels);

    let metric = this.metrics.get(labelKey);
    if (!metric) {
      // Allocate slot for this label combination
      let slot = this.slots.get(labelKey);
      if (slot === undefined) {
        const labelMap: Record<string, string> = {};
        for (const [key, value] of Object.entries(labels)) {
          labelMap[key] = String(value);
        }
        const metricType =
          typeof (0 as T) === "bigint"
            ? MetricType.GaugeInt
            : MetricType.GaugeInt;
        slot = this.registry.allocateSlot(this.name, labelMap, metricType);
        this.slots.set(labelKey, slot);
      }
      metric = new AtomicMetric(this.buffer, slot);
      this.metrics.set(labelKey, metric);
    }

    return new GaugeInstance(metric);
  }

  private serializeLabels(labels: L): string {
    const sorted = Object.entries(labels).sort(([a], [b]) =>
      a.localeCompare(b)
    );
    return JSON.stringify(sorted);
  }
}

/**
 * A gauge instance with specific label values.
 */
class GaugeInstance<T extends number | bigint> {
  constructor(private metric: AtomicMetric) {}

  set(value: T): void {
    this.metric.set(value);
  }

  get(): bigint {
    return this.metric.get();
  }
}
