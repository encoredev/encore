---
title: encore.dev/types
lang: ts
toc: true
---

## Classes

<!-- symbol-start: Decimal -->
### Decimal

<!-- source: types/mod.ts:23 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L23)

A decimal type that can hold values with arbitrary precision.
Unlike JavaScript's native number type, this can accurately represent
decimal values without floating-point precision errors.

#### Constructors

##### Constructor

`new Decimal(value): Decimal`

<!-- source: types/mod.ts:26 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L26)

###### Parameters

###### value

[`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

#### Accessors

##### value

###### Get Signature

`get value(): string`

###### Returns

`string`

#### Methods

##### \[toPrimitive\]()

`toPrimitive: string | number`

<!-- source: types/mod.ts:81 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L81)

###### Parameters

###### hint

`string`

###### Returns

`string` \| `number`

##### add()

`add(d): Decimal`

<!-- source: types/mod.ts:45 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L45)

Adds this decimal to another decimal value.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### div()

`div(d): Decimal`

<!-- source: types/mod.ts:66 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L66)

Divides this decimal by another decimal value.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### mul()

`mul(d): Decimal`

<!-- source: types/mod.ts:59 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L59)

Multiplies this decimal by another decimal value.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### sub()

`sub(d): Decimal`

<!-- source: types/mod.ts:52 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L52)

Subtracts another decimal value from this decimal.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### toJSON()

`toJSON(): string`

<!-- source: types/mod.ts:74 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L74)

###### Returns

`string`

##### toString()

`toString(): string`

<!-- source: types/mod.ts:77 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L77)

###### Returns

`string`

<!-- symbol-end -->

## Type Aliases

<!-- symbol-start: DurationString -->
### DurationString

```ts
type DurationString = 
  | durationComponent
  | `${durationComponent}${durationComponent}`
  | `${durationComponent} ${durationComponent}`;
```

<!-- source: types/mod.ts:11 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L11)

A duration is a string representing a length of time.

Examples: `"10s"`, `"500ms"`, `"5m"`, `"1h30m"`, `"1h 30m"`.

***

<!-- symbol-end -->

<!-- symbol-start: ToDecimal -->
### ToDecimal

`type ToDecimal = string | number | bigint`

<!-- source: types/mod.ts:16 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L16)


<!-- symbol-end -->