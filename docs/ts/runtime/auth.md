---
title: encore.dev/auth
lang: ts
toc: true
---

## Type Aliases

<!-- symbol-start: AuthHandler -->
### AuthHandler

`type AuthHandler<Params, AuthData> = (params) => Promise<AuthData | null> & AuthHandlerBrand`

<!-- source: auth/mod.ts:1 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/auth/mod.ts#L1)

#### Type Parameters

##### Params

`Params` *extends* `object`

##### AuthData

`AuthData` *extends* \{
  `userID`: `string`;
\}

***

<!-- symbol-end -->

<!-- symbol-start: AuthHandlerBrand -->
### AuthHandlerBrand

```ts
type AuthHandlerBrand = {
  __authHandlerBrand: unique symbol;
};
```

<!-- source: auth/mod.ts:6 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/auth/mod.ts#L6)

#### Properties

##### \_\_authHandlerBrand

`readonly __authHandlerBrand: unique symbol`

<!-- symbol-end -->

## Functions

<!-- symbol-start: authHandler() -->
### authHandler()

`function authHandler<Params, AuthData>(fn): AuthHandler<Params, AuthData>`

<!-- source: auth/mod.ts:8 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/auth/mod.ts#L8)

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


<!-- symbol-end -->