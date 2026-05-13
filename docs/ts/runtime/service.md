---
title: encore.dev/service
lang: ts
toc: true
---

## Classes

<!-- symbol-start: Service -->
### Service <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/service/mod.ts#L12" target="_blank" rel="noopener">source</a>

Defines an Encore backend service.

Use this class to define a new backend service with the given name.
The scope of the service is its containing directory, and all subdirectories.

It must be called from files named `encore.service.ts`, to enable Encore to
efficiently identify possible service definitions.

#### Constructors

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/service/mod.ts#L16" target="_blank" rel="noopener">source</a>

`new Service(name, cfg?): Service`

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
### ServiceConfig <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/service/mod.ts#L22" target="_blank" rel="noopener">source</a>

#### Properties

##### middlewares?

`optional middlewares?: Middleware[]`


<!-- symbol-end -->