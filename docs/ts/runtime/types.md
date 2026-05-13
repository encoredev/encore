---
title: encore.dev/types
lang: ts
toc: true
---

## Classes

<!-- symbol-start: Decimal -->
### Decimal <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L23" target="_blank" rel="noopener">source</a>

A decimal type that can hold values with arbitrary precision.
Unlike JavaScript's native number type, this can accurately represent
decimal values without floating-point precision errors.

#### Constructors

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L26" target="_blank" rel="noopener">source</a>

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

##### \[toPrimitive\]() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L81" target="_blank" rel="noopener">source</a>

`toPrimitive: string | number`

###### Parameters

###### hint

`string`

###### Returns

`string` \| `number`

##### add() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L45" target="_blank" rel="noopener">source</a>

`add(d): Decimal`

Adds this decimal to another decimal value.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### div() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L66" target="_blank" rel="noopener">source</a>

`div(d): Decimal`

Divides this decimal by another decimal value.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### mul() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L59" target="_blank" rel="noopener">source</a>

`mul(d): Decimal`

Multiplies this decimal by another decimal value.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### sub() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L52" target="_blank" rel="noopener">source</a>

`sub(d): Decimal`

Subtracts another decimal value from this decimal.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### toJSON() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L74" target="_blank" rel="noopener">source</a>

`toJSON(): string`

###### Returns

`string`

##### toString() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L77" target="_blank" rel="noopener">source</a>

`toString(): string`

###### Returns

`string`

<!-- symbol-end -->

## Type Aliases

<!-- symbol-start: DurationString -->
### DurationString <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L11" target="_blank" rel="noopener">source</a>

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
### ToDecimal <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L16" target="_blank" rel="noopener">source</a>

`type ToDecimal = string | number | bigint`


<!-- symbol-end -->