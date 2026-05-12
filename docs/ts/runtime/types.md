---
title: encore.dev/types
lang: ts
toc: true
---

# encore.dev/types

## Classes

### Decimal

Defined in: [types/mod.ts:10](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/types/mod.ts#L10)

A decimal type that can hold values with arbitrary precision.
Unlike JavaScript's native number type, this can accurately represent
decimal values without floating-point precision errors.

#### Constructors

##### Constructor

```ts
new Decimal(value): Decimal;
```

Defined in: [types/mod.ts:13](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/types/mod.ts#L13)

###### Parameters

###### value

[`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

#### Accessors

##### value

###### Get Signature

```ts
get value(): string;
```

Defined in: [types/mod.ts:57](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/types/mod.ts#L57)

###### Returns

`string`

#### Methods

##### \[toPrimitive\]()

```ts
toPrimitive: string | number;
```

Defined in: [types/mod.ts:68](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/types/mod.ts#L68)

###### Parameters

###### hint

`string`

###### Returns

`string` \| `number`

##### add()

```ts
add(d): Decimal;
```

Defined in: [types/mod.ts:32](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/types/mod.ts#L32)

Adds this decimal to another decimal value.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### div()

```ts
div(d): Decimal;
```

Defined in: [types/mod.ts:53](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/types/mod.ts#L53)

Divides this decimal by another decimal value.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### mul()

```ts
mul(d): Decimal;
```

Defined in: [types/mod.ts:46](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/types/mod.ts#L46)

Multiplies this decimal by another decimal value.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### sub()

```ts
sub(d): Decimal;
```

Defined in: [types/mod.ts:39](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/types/mod.ts#L39)

Subtracts another decimal value from this decimal.

###### Parameters

###### d

[`Decimal`](#decimal) \| [`ToDecimal`](#todecimal)

###### Returns

[`Decimal`](#decimal)

##### toJSON()

```ts
toJSON(): string;
```

Defined in: [types/mod.ts:61](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/types/mod.ts#L61)

###### Returns

`string`

##### toString()

```ts
toString(): string;
```

Defined in: [types/mod.ts:64](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/types/mod.ts#L64)

###### Returns

`string`

## Type Aliases

### ToDecimal

```ts
type ToDecimal = string | number | bigint;
```

Defined in: [types/mod.ts:3](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/types/mod.ts#L3)
