---
title: encore.dev/metrics
lang: ts
toc: true
---

# encore.dev/metrics

## Classes

### Counter

Defined in: [metrics/mod.ts:71](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L71)

A Counter tracks cumulative values that only increase.
Use counters for metrics like request counts, errors, etc.

#### Constructors

##### Constructor

```ts
new Counter(name, cfg?): Counter;
```

Defined in: [metrics/mod.ts:77](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L77)

###### Parameters

###### name

`string`

###### cfg?

[`MetricConfig`](#metricconfig)

###### Returns

[`Counter`](#counter)

#### Methods

##### increment()

```ts
increment(value?): void;
```

Defined in: [metrics/mod.ts:87](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L87)

Increment the counter by the given value (default 1).

###### Parameters

###### value?

`number` = `1`

###### Returns

`void`

##### ref()

```ts
ref(): Counter;
```

Defined in: [metrics/mod.ts:116](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L116)

###### Returns

[`Counter`](#counter)

***

### CounterGroup

Defined in: [metrics/mod.ts:128](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L128)

A CounterGroup tracks counters with labels.
Each unique combination of label values creates a separate counter time series.

#### Type Parameters

##### L

`L` *extends* `Record`\<keyof `L`, `string` \| `number` \| `boolean`\>

The label interface (must have string/number/boolean fields)
Note: Number values in labels are converted to integers using Math.floor().

#### Constructors

##### Constructor

```ts
new CounterGroup<L>(name, cfg?): CounterGroup<L>;
```

Defined in: [metrics/mod.ts:135](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L135)

###### Parameters

###### name

`string`

###### cfg?

[`MetricConfig`](#metricconfig)

###### Returns

[`CounterGroup`](#countergroup)\<`L`\>

#### Methods

##### ref()

```ts
ref(): CounterGroup<L>;
```

Defined in: [metrics/mod.ts:163](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L163)

###### Returns

[`CounterGroup`](#countergroup)\<`L`\>

##### with()

```ts
with(labels): Counter;
```

Defined in: [metrics/mod.ts:146](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L146)

Get a counter for the given label values.

Note: Number values in labels are converted to integers using Math.floor().

###### Parameters

###### labels

`L`

###### Returns

[`Counter`](#counter)

***

### Gauge

Defined in: [metrics/mod.ts:172](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L172)

A Gauge tracks values that can go up or down.
Use gauges for metrics like memory usage, active connections, temperature, etc.

#### Constructors

##### Constructor

```ts
new Gauge(name, cfg?): Gauge;
```

Defined in: [metrics/mod.ts:178](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L178)

###### Parameters

###### name

`string`

###### cfg?

[`MetricConfig`](#metricconfig)

###### Returns

[`Gauge`](#gauge)

#### Methods

##### ref()

```ts
ref(): Gauge;
```

Defined in: [metrics/mod.ts:217](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L217)

###### Returns

[`Gauge`](#gauge)

##### set()

```ts
set(value): void;
```

Defined in: [metrics/mod.ts:188](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L188)

Set the gauge to the given value.

###### Parameters

###### value

`number`

###### Returns

`void`

***

### GaugeGroup

Defined in: [metrics/mod.ts:222](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L222)

#### Type Parameters

##### L

`L` *extends* `Record`\<keyof `L`, `string` \| `number` \| `boolean`\>

#### Constructors

##### Constructor

```ts
new GaugeGroup<L>(name, cfg?): GaugeGroup<L>;
```

Defined in: [metrics/mod.ts:227](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L227)

###### Parameters

###### name

`string`

###### cfg?

[`MetricConfig`](#metricconfig)

###### Returns

[`GaugeGroup`](#gaugegroup)\<`L`\>

#### Methods

##### ref()

```ts
ref(): GaugeGroup<L>;
```

Defined in: [metrics/mod.ts:255](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L255)

###### Returns

[`GaugeGroup`](#gaugegroup)\<`L`\>

##### with()

```ts
with(labels): Gauge;
```

Defined in: [metrics/mod.ts:238](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L238)

Get a gauge for the given label values.

Note: Number values in labels are converted to integers using Math.floor().

###### Parameters

###### labels

`L`

###### Returns

[`Gauge`](#gauge)

## Interfaces

### MetricConfig

Defined in: [metrics/mod.ts:41](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/metrics/mod.ts#L41)
