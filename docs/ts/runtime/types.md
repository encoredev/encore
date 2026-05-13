---
title: encore.dev/types
lang: ts
toc: true
---

# encore.dev/types

## Classes

### Decimal

<!-- source: types/mod.ts:10 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L10 -->

A decimal type that can hold values with arbitrary precision.
Unlike JavaScript's native number type, this can accurately represent
decimal values without floating-point precision errors.

#### Constructors

##### Constructor

`new Decimal(value): Decimal;`

<!-- source: types/mod.ts:13 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L13 -->

###### Parameters

###### value

[`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

#### Accessors

##### value

###### Get Signature

`get value(): string;`

<!-- source: types/mod.ts:57 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L57 -->

###### Returns

`string`

#### Methods

##### \[toPrimitive\]()

`toPrimitive: string | number;`

<!-- source: types/mod.ts:68 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L68 -->

###### Parameters

###### hint

`string`

###### Returns

`string` \| `number`

##### add()

`add(d): Decimal;`

<!-- source: types/mod.ts:32 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L32 -->

Adds this decimal to another decimal value.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### div()

`div(d): Decimal;`

<!-- source: types/mod.ts:53 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L53 -->

Divides this decimal by another decimal value.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### mul()

`mul(d): Decimal;`

<!-- source: types/mod.ts:46 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L46 -->

Multiplies this decimal by another decimal value.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### sub()

`sub(d): Decimal;`

<!-- source: types/mod.ts:39 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L39 -->

Subtracts another decimal value from this decimal.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### toJSON()

`toJSON(): string;`

<!-- source: types/mod.ts:61 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L61 -->

###### Returns

`string`

##### toString()

`toString(): string;`

<!-- source: types/mod.ts:64 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L64 -->

###### Returns

`string`

## Type Aliases

### ToDecimal

`type ToDecimal = string | number | bigint;`

<!-- source: types/mod.ts:3 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/types/mod.ts#L3 -->
