---
title: encore.dev/metrics
lang: ts
toc: true
---

## Classes

<!-- symbol-start: Counter -->
### Counter <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L71" target="_blank" rel="noopener">source</a>

A Counter tracks cumulative values that only increase.
Use counters for metrics like request counts, errors, etc.

#### Constructors

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L77" target="_blank" rel="noopener">source</a>

`new Counter(name, cfg?): Counter`

###### Parameters

###### name

`string`

###### cfg?

[`MetricConfig`](#metricconfig)

###### Returns

[`Counter`](#counter)

#### Methods

##### increment() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L87" target="_blank" rel="noopener">source</a>

`increment(value?): void`

Increment the counter by the given value (default 1).

###### Parameters

###### value?

`number` = `1`

###### Returns

`void`

##### ref() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L116" target="_blank" rel="noopener">source</a>

`ref(): Counter`

###### Returns

[`Counter`](#counter)

***

<!-- symbol-end -->

<!-- symbol-start: CounterGroup -->
### CounterGroup <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L128" target="_blank" rel="noopener">source</a>

A CounterGroup tracks counters with labels.
Each unique combination of label values creates a separate counter time series.

#### Type Parameters

##### L

`L` *extends* `Record`\<keyof `L`, `string` \| `number` \| `boolean`\>

The label interface (must have string/number/boolean fields)
Note: Number values in labels are converted to integers using Math.floor().

#### Constructors

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L135" target="_blank" rel="noopener">source</a>

`new CounterGroup<L>(name, cfg?): CounterGroup<L>`

###### Parameters

###### name

`string`

###### cfg?

[`MetricConfig`](#metricconfig)

###### Returns

[`CounterGroup`](#countergroup)\<`L`\>

#### Methods

##### ref() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L163" target="_blank" rel="noopener">source</a>

`ref(): CounterGroup<L>`

###### Returns

[`CounterGroup`](#countergroup)\<`L`\>

##### with() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L146" target="_blank" rel="noopener">source</a>

`with(labels): Counter`

Get a counter for the given label values.

Note: Number values in labels are converted to integers using Math.floor().

###### Parameters

###### labels

`L`

###### Returns

[`Counter`](#counter)

***

<!-- symbol-end -->

<!-- symbol-start: Gauge -->
### Gauge <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L172" target="_blank" rel="noopener">source</a>

A Gauge tracks values that can go up or down.
Use gauges for metrics like memory usage, active connections, temperature, etc.

#### Constructors

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L178" target="_blank" rel="noopener">source</a>

`new Gauge(name, cfg?): Gauge`

###### Parameters

###### name

`string`

###### cfg?

[`MetricConfig`](#metricconfig)

###### Returns

[`Gauge`](#gauge)

#### Methods

##### ref() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L217" target="_blank" rel="noopener">source</a>

`ref(): Gauge`

###### Returns

[`Gauge`](#gauge)

##### set() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L188" target="_blank" rel="noopener">source</a>

`set(value): void`

Set the gauge to the given value.

###### Parameters

###### value

`number`

###### Returns

`void`

***

<!-- symbol-end -->

<!-- symbol-start: GaugeGroup -->
### GaugeGroup <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L222" target="_blank" rel="noopener">source</a>

#### Type Parameters

##### L

`L` *extends* `Record`\<keyof `L`, `string` \| `number` \| `boolean`\>

#### Constructors

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L227" target="_blank" rel="noopener">source</a>

`new GaugeGroup<L>(name, cfg?): GaugeGroup<L>`

###### Parameters

###### name

`string`

###### cfg?

[`MetricConfig`](#metricconfig)

###### Returns

[`GaugeGroup`](#gaugegroup)\<`L`\>

#### Methods

##### ref() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L255" target="_blank" rel="noopener">source</a>

`ref(): GaugeGroup<L>`

###### Returns

[`GaugeGroup`](#gaugegroup)\<`L`\>

##### with() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L238" target="_blank" rel="noopener">source</a>

`with(labels): Gauge`

Get a gauge for the given label values.

Note: Number values in labels are converted to integers using Math.floor().

###### Parameters

###### labels

`L`

###### Returns

[`Gauge`](#gauge)

<!-- symbol-end -->

## Interfaces

<!-- symbol-start: MetricConfig -->
### MetricConfig <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L41" target="_blank" rel="noopener">source</a>


<!-- symbol-end -->