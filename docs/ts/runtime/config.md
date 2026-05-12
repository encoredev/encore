---
title: encore.dev/config
lang: ts
toc: true
---

# encore.dev/config

## Interfaces

### Secret()

Defined in: [config/secrets.ts:18](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/config/secrets.ts#L18)

Secret represents a single secret value that is loaded
into the application. It is strongly typed for that secret,
so that you can write functions which expect a specific one.

You can use [AnySecret](#anysecret) to represent any secret without knowing
it's name.

#### Example

```ts
function doFoo(s: Secret<"foo">): void {
  const foo = s();
}
```

#### Type Parameters

##### Name

`Name` *extends* `string`

```ts
Secret(): string;
```

Defined in: [config/secrets.ts:27](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/config/secrets.ts#L27)

Returns the current value of the secret.

Encore will periodically refresh the value of the secret, so this
value may change over time and could be stale for upto a couple of
minutes. If you need to ensure you have the latest value, use
`latest`.

#### Returns

`string`

#### Properties

##### name

```ts
readonly name: Name;
```

Defined in: [config/secrets.ts:32](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/config/secrets.ts#L32)

The name of the secret.

## Type Aliases

### AnySecret

```ts
type AnySecret = Secret<string>;
```

Defined in: [config/secrets.ts:39](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/config/secrets.ts#L39)

AnySecret is a type which can be used to represent any [Secret](#secret)
without knowing its name.

## Functions

### secret()

```ts
function secret<Name>(name): Secret<Name>;
```

Defined in: [config/secrets.ts:50](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/config/secrets.ts#L50)

secret is used to load a single [Secret](#secret) into the application.

If you wish to load multiple secrets at once, see `secrets`.

#### Type Parameters

##### Name

`Name` *extends* `string`

#### Parameters

##### name

`StringLiteral`\<`Name`\>

#### Returns

[`Secret`](#secret)\<`Name`\>

#### Example

```ts
loading a single secret
 import {secret} from "encore.dev/config/secrets";
 const foo = secret<"foo">();
```
