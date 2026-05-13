---
title: encore.dev/metrics
lang: ts
toc: true
---

## Classes

<!-- symbol-start: Counter -->
### Counter [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L71)

A Counter tracks cumulative values that only increase.
Use counters for metrics like request counts, errors, etc.

#### Constructors

##### Constructor [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L77)

`new Counter(name, cfg?): Counter`

###### Parameters

###### name

`string`

###### cfg?

[`MetricConfig`](#metricconfig)

###### Returns

[`Counter`](#counter)

#### Methods

##### increment() [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L87)

`increment(value?): void`

Increment the counter by the given value (default 1).

###### Parameters

###### value?

`number` = `1`

###### Returns

`void`

##### ref() [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L116)

`ref(): Counter`

###### Returns

[`Counter`](#counter)

***

<!-- symbol-end -->

<!-- symbol-start: CounterGroup -->
### CounterGroup [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L128)

A CounterGroup tracks counters with labels.
Each unique combination of label values creates a separate counter time series.

#### Type Parameters

##### L

`L` *extends* `Record`\<keyof `L`, `string` \| `number` \| `boolean`\>

The label interface (must have string/number/boolean fields)
Note: Number values in labels are converted to integers using Math.floor().

#### Constructors

##### Constructor [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L135)

`new CounterGroup<L>(name, cfg?): CounterGroup<L>`

###### Parameters

###### name

`string`

###### cfg?

[`MetricConfig`](#metricconfig)

###### Returns

[`CounterGroup`](#countergroup)\<`L`\>

#### Methods

##### ref() [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L163)

`ref(): CounterGroup<L>`

###### Returns

[`CounterGroup`](#countergroup)\<`L`\>

##### with() [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L146)

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
### Gauge [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L172)

A Gauge tracks values that can go up or down.
Use gauges for metrics like memory usage, active connections, temperature, etc.

#### Constructors

##### Constructor [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L178)

`new Gauge(name, cfg?): Gauge`

###### Parameters

###### name

`string`

###### cfg?

[`MetricConfig`](#metricconfig)

###### Returns

[`Gauge`](#gauge)

#### Methods

##### ref() [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L217)

`ref(): Gauge`

###### Returns

[`Gauge`](#gauge)

##### set() [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L188)

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
### GaugeGroup [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L222)

#### Type Parameters

##### L

`L` *extends* `Record`\<keyof `L`, `string` \| `number` \| `boolean`\>

#### Constructors

##### Constructor [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L227)

`new GaugeGroup<L>(name, cfg?): GaugeGroup<L>`

###### Parameters

###### name

`string`

###### cfg?

[`MetricConfig`](#metricconfig)

###### Returns

[`GaugeGroup`](#gaugegroup)\<`L`\>

#### Methods

##### ref() [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L255)

`ref(): GaugeGroup<L>`

###### Returns

[`GaugeGroup`](#gaugegroup)\<`L`\>

##### with() [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L238)

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
### MetricConfig [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L41)


<!-- symbol-end -->