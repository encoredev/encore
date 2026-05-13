---
title: encore.dev/service
lang: ts
toc: true
---

# encore.dev/service

## Classes

### Service

<!-- source: service/mod.ts:12 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/service/mod.ts#L12 -->

Defines an Encore backend service.

Use this class to define a new backend service with the given name.
The scope of the service is its containing directory, and all subdirectories.

It must be called from files named `encore.service.ts`, to enable Encore to
efficiently identify possible service definitions.

#### Constructors

##### Constructor

`new Service(name, cfg?): Service;`

<!-- source: service/mod.ts:16 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/service/mod.ts#L16 -->

###### Parameters

###### name

`string`

###### cfg?

[`ServiceConfig`](#serviceconfig)

###### Returns

[`Service`](#service)

#### Properties

##### cfg

`readonly cfg: ServiceConfig;`

<!-- source: service/mod.ts:14 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/service/mod.ts#L14 -->

##### name

`readonly name: string;`

<!-- source: service/mod.ts:13 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/service/mod.ts#L13 -->

## Interfaces

### ServiceConfig

<!-- source: service/mod.ts:22 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/service/mod.ts#L22 -->

#### Properties

##### middlewares?

`optional middlewares?: Middleware[];`

<!-- source: service/mod.ts:23 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/service/mod.ts#L23 -->
