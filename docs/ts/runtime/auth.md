---
title: encore.dev/auth
lang: ts
toc: true
---

# encore.dev/auth

## Type Aliases

### AuthHandler

`type AuthHandler<Params, AuthData> = (params) => Promise<AuthData | null> & AuthHandlerBrand;`

<!-- source: auth/mod.ts:1 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/auth/mod.ts#L1 -->

#### Type Parameters

##### Params

`Params` *extends* `object`

##### AuthData

`AuthData` *extends* \{
  `userID`: `string`;
\}

***

### AuthHandlerBrand

```ts
type AuthHandlerBrand = {
  __authHandlerBrand: unique symbol;
};
```

<!-- source: auth/mod.ts:6 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/auth/mod.ts#L6 -->

#### Properties

##### \_\_authHandlerBrand

`readonly __authHandlerBrand: unique symbol;`

<!-- source: auth/mod.ts:6 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/auth/mod.ts#L6 -->

## Functions

### authHandler()

`function authHandler<Params, AuthData>(fn): AuthHandler<Params, AuthData>;`

<!-- source: auth/mod.ts:8 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/auth/mod.ts#L8 -->

#### Type Parameters

##### Params

`Params` *extends* `object`

##### AuthData

`AuthData` *extends* \{
  `userID`: `string`;
\}

#### Parameters

##### fn

(`params`) => `Promise`\<`AuthData` \| `null`\>

#### Returns

[`AuthHandler`](#authhandler)\<`Params`, `AuthData`\>
