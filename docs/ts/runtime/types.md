---
title: encore.dev/types
lang: ts
toc: true
---

## Classes

<!-- symbol-start: Decimal -->
### Decimal [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L23)

A decimal type that can hold values with arbitrary precision.
Unlike JavaScript's native number type, this can accurately represent
decimal values without floating-point precision errors.

#### Constructors

##### Constructor [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L26)

`new Decimal(value): Decimal`

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

##### \[toPrimitive\]() [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L81)

`toPrimitive: string | number`

###### Parameters

###### hint

`string`

###### Returns

`string` \| `number`

##### add() [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L45)

`add(d): Decimal`

Adds this decimal to another decimal value.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### div() [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L66)

`div(d): Decimal`

Divides this decimal by another decimal value.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### mul() [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L59)

`mul(d): Decimal`

Multiplies this decimal by another decimal value.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### sub() [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L52)

`sub(d): Decimal`

Subtracts another decimal value from this decimal.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### toJSON() [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L74)

`toJSON(): string`

###### Returns

`string`

##### toString() [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L77)

`toString(): string`

###### Returns

`string`

<!-- symbol-end -->

## Type Aliases

<!-- symbol-start: DurationString -->
### DurationString [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L11)

```ts
type DurationString = 
  | durationComponent
  | `${durationComponent}${durationComponent}`
  | `${durationComponent} ${durationComponent}`;
```

A duration is a string representing a length of time.

Examples: `"10s"`, `"500ms"`, `"5m"`, `"1h30m"`, `"1h 30m"`.

***

<!-- symbol-end -->

<!-- symbol-start: ToDecimal -->
### ToDecimal [source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L16)

`type ToDecimal = string | number | bigint`


<!-- symbol-end -->