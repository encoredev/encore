---
title: encore.dev/service
lang: ts
toc: true
---

# encore.dev/service

## Classes

### Service

Defined in: [service/mod.ts:12](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/service/mod.ts#L12)

Defines an Encore backend service.

Use this class to define a new backend service with the given name.
The scope of the service is its containing directory, and all subdirectories.

It must be called from files named `encore.service.ts`, to enable Encore to
efficiently identify possible service definitions.

#### Constructors

##### Constructor

```ts
new Service(name, cfg?): Service;
```

Defined in: [service/mod.ts:16](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/service/mod.ts#L16)

###### Parameters

###### name

`string`

###### cfg?

[`ServiceConfig`](#serviceconfig)

###### Returns

[`Service`](#service)

#### Properties

##### cfg

```ts
readonly cfg: ServiceConfig;
```

Defined in: [service/mod.ts:14](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/service/mod.ts#L14)

##### name

```ts
readonly name: string;
```

Defined in: [service/mod.ts:13](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/service/mod.ts#L13)

## Interfaces

### ServiceConfig

Defined in: [service/mod.ts:22](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/service/mod.ts#L22)

#### Properties

##### middlewares?

```ts
optional middlewares?: Middleware[];
```

Defined in: [service/mod.ts:23](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/service/mod.ts#L23)
