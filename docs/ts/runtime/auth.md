---
title: encore.dev/auth
lang: ts
toc: true
---

## Type Aliases

<!-- symbol-start: AuthHandler -->
### AuthHandler <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/auth/mod.ts#L1" target="_blank" rel="noopener">source</a>

`type AuthHandler<Params, AuthData> = (params) => Promise<AuthData | null> & AuthHandlerBrand`

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
### AuthHandlerBrand <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/auth/mod.ts#L6" target="_blank" rel="noopener">source</a>

```ts
type AuthHandlerBrand = {
  __authHandlerBrand: unique symbol;
};
```

#### Properties

##### \_\_authHandlerBrand

`readonly __authHandlerBrand: unique symbol`

<!-- symbol-end -->

## Functions

<!-- symbol-start: authHandler() -->
### authHandler() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/auth/mod.ts#L8" target="_blank" rel="noopener">source</a>

`function authHandler<Params, AuthData>(fn): AuthHandler<Params, AuthData>`

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