---
title: encore.dev/service
lang: ts
toc: true
---

## Classes

<!-- symbol-start: Service -->
### Service

<!-- source: service/mod.ts:12 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/service/mod.ts#L12)

Defines an Encore backend service.

Use this class to define a new backend service with the given name.
The scope of the service is its containing directory, and all subdirectories.

It must be called from files named `encore.service.ts`, to enable Encore to
efficiently identify possible service definitions.

#### Constructors

##### Constructor

`new Service(name, cfg?): Service`

<!-- source: service/mod.ts:16 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/service/mod.ts#L16)

###### Parameters

###### name

`string`

###### cfg?

[`ServiceConfig`](#serviceconfig)

###### Returns

[`Service`](#service)

#### Properties

##### cfg

`readonly cfg: ServiceConfig`

##### name

`readonly name: string`

<!-- symbol-end -->

## Interfaces

<!-- symbol-start: ServiceConfig -->
### ServiceConfig

<!-- source: service/mod.ts:22 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/service/mod.ts#L22)

#### Properties

##### middlewares?

`optional middlewares?: Middleware[]`


<!-- symbol-end -->