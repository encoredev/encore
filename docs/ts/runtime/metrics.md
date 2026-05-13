---
title: encore.dev/metrics
lang: ts
toc: true
---

## Classes

<!-- symbol-start: Counter -->
### Counter

<!-- source: metrics/mod.ts:71 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L71)

A Counter tracks cumulative values that only increase.
Use counters for metrics like request counts, errors, etc.

#### Constructors

##### Constructor

`new Counter(name, cfg?): Counter`

<!-- source: metrics/mod.ts:77 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L77)

###### Parameters

###### name

`string`

###### cfg?

[`MetricConfig`](#metricconfig)

###### Returns

[`Counter`](#counter)

#### Methods

##### increment()

`increment(value?): void`

<!-- source: metrics/mod.ts:87 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L87)

Increment the counter by the given value (default 1).

###### Parameters

###### value?

`number` = `1`

###### Returns

`void`

##### ref()

`ref(): Counter`

<!-- source: metrics/mod.ts:116 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L116)

###### Returns

[`Counter`](#counter)

***

<!-- symbol-end -->

<!-- symbol-start: CounterGroup -->
### CounterGroup

<!-- source: metrics/mod.ts:128 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L128)

A CounterGroup tracks counters with labels.
Each unique combination of label values creates a separate counter time series.

#### Type Parameters

##### L

`L` *extends* `Record`\<keyof `L`, `string` \| `number` \| `boolean`\>

The label interface (must have string/number/boolean fields)
Note: Number values in labels are converted to integers using Math.floor().

#### Constructors

##### Constructor

`new CounterGroup<L>(name, cfg?): CounterGroup<L>`

<!-- source: metrics/mod.ts:135 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L135)

###### Parameters

###### name

`string`

###### cfg?

[`MetricConfig`](#metricconfig)

###### Returns

[`CounterGroup`](#countergroup)\<`L`\>

#### Methods

##### ref()

`ref(): CounterGroup<L>`

<!-- source: metrics/mod.ts:163 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L163)

###### Returns

[`CounterGroup`](#countergroup)\<`L`\>

##### with()

`with(labels): Counter`

<!-- source: metrics/mod.ts:146 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L146)

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
### Gauge

<!-- source: metrics/mod.ts:172 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L172)

A Gauge tracks values that can go up or down.
Use gauges for metrics like memory usage, active connections, temperature, etc.

#### Constructors

##### Constructor

`new Gauge(name, cfg?): Gauge`

<!-- source: metrics/mod.ts:178 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L178)

###### Parameters

###### name

`string`

###### cfg?

[`MetricConfig`](#metricconfig)

###### Returns

[`Gauge`](#gauge)

#### Methods

##### ref()

`ref(): Gauge`

<!-- source: metrics/mod.ts:217 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L217)

###### Returns

[`Gauge`](#gauge)

##### set()

`set(value): void`

<!-- source: metrics/mod.ts:188 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L188)

Set the gauge to the given value.

###### Parameters

###### value

`number`

###### Returns

`void`

***

<!-- symbol-end -->

<!-- symbol-start: GaugeGroup -->
### GaugeGroup

<!-- source: metrics/mod.ts:222 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L222)

#### Type Parameters

##### L

`L` *extends* `Record`\<keyof `L`, `string` \| `number` \| `boolean`\>

#### Constructors

##### Constructor

`new GaugeGroup<L>(name, cfg?): GaugeGroup<L>`

<!-- source: metrics/mod.ts:227 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L227)

###### Parameters

###### name

`string`

###### cfg?

[`MetricConfig`](#metricconfig)

###### Returns

[`GaugeGroup`](#gaugegroup)\<`L`\>

#### Methods

##### ref()

`ref(): GaugeGroup<L>`

<!-- source: metrics/mod.ts:255 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L255)

###### Returns

[`GaugeGroup`](#gaugegroup)\<`L`\>

##### with()

`with(labels): Gauge`

<!-- source: metrics/mod.ts:238 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L238)

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
### MetricConfig

<!-- source: metrics/mod.ts:41 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L41)


<!-- symbol-end -->