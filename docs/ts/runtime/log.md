---
title: encore.dev/log
lang: ts
toc: true
---

## Enumerations

<!-- symbol-start: LogLevel -->
### LogLevel

<!-- source: log/mod.ts:28 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L28)

#### Enumeration Members

##### Debug

`Debug: number`

##### Error

`Error: number`

##### Info

`Info: number`

##### Trace

`Trace: number`

##### Warn

`Warn: number`

<!-- symbol-end -->

## Interfaces

<!-- symbol-start: Logger -->
### Logger

<!-- source: log/mod.ts:36 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L36)

#### Methods

##### debug()

`debug(msg, fields?): void`

<!-- source: log/mod.ts:67 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L67)

Debug logs a message at the debug level.

###### Parameters

###### msg

`string`

###### fields?

`any`

###### Returns

`void`

##### error()

###### Call Signature

`error(err, fields?): void`

###### Parameters

###### err

`unknown`

###### fields?

`any`

###### Returns

`void`

###### Call Signature

```ts
error(
   err, 
   msg, 
   fields?): void;
```

###### Parameters

###### err

`unknown`

###### msg

`string`

###### fields?

`any`

###### Returns

`void`

###### Call Signature

`error(msg, fields?): void`

###### Parameters

###### msg

`string`

###### fields?

`any`

###### Returns

`void`

##### info()

`info(msg, fields?): void`

<!-- source: log/mod.ts:74 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L74)

Info logs a message at the info level.

###### Parameters

###### msg

`string`

###### fields?

`any`

###### Returns

`void`

##### trace()

`trace(msg, fields?): void`

<!-- source: log/mod.ts:60 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L60)

Trace logs a message at the trace level.

###### Parameters

###### msg

`string`

###### fields?

`any`

###### Returns

`void`

##### warn()

###### Call Signature

`warn(err, fields?): void`

Warn logs a message at the warn level.

###### Parameters

###### err

`unknown`

###### fields?

`any`

###### Returns

`void`

###### Call Signature

```ts
warn(
   err, 
   msg, 
   fields?): void;
```

Warn logs a message at the warn level.

###### Parameters

###### err

`unknown`

###### msg

`string`

###### fields?

`any`

###### Returns

`void`

###### Call Signature

`warn(msg, fields?): void`

Warn logs a message at the warn level.

###### Parameters

###### msg

`string`

###### fields?

`any`

###### Returns

`void`

##### with()

`with(fields): Logger`

<!-- source: log/mod.ts:53 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L53)

Returns a new logger with the given fields added to the context.

###### Parameters

###### fields

`any`

###### Returns

[`Logger`](#logger)

##### withLevel()

`withLevel(level): Logger`

<!-- source: log/mod.ts:46 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L46)

Returns a new logger with the specified level.

###### Parameters

###### level

[`LogLevel`](#loglevel)

###### Returns

[`Logger`](#logger)

<!-- symbol-end -->

## Type Aliases

<!-- symbol-start: FieldsObject -->
### FieldsObject

`type FieldsObject = Record<string, FieldValue>`

<!-- source: log/mod.ts:26 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L26)

A map of fields that can be logged

***

<!-- symbol-end -->

<!-- symbol-start: FieldValue -->
### FieldValue

```ts
type FieldValue = 
  | string
  | number
  | boolean
  | null
  | undefined
  | FieldsObject
  | FieldValue[];
```

<!-- source: log/mod.ts:12 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L12)

A field value we support logging

<!-- symbol-end -->

## Variables

<!-- symbol-start: default -->
### default

`const default: Logger`

<!-- source: log/mod.ts:149 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L149)

<!-- symbol-end -->

## Functions

<!-- symbol-start: debug() -->
### debug()

`function debug(msg, fields?): void`

<!-- source: log/mod.ts:166 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L166)

Debug logs a message at the debug level

#### Parameters

##### msg

`string`

##### fields?

`any`

#### Returns

`void`

***

<!-- symbol-end -->

<!-- symbol-start: error() -->
### error()

#### Call Signature

`function error(err, fields?): void`

Error logs a message at the error level

##### Parameters

###### err

`unknown`

###### fields?

`any`

##### Returns

`void`

#### Call Signature

```ts
function error(
   err, 
   msg, 
   fields?): void;
```

Error logs a message at the error level

##### Parameters

###### err

`unknown`

###### msg

`string`

###### fields?

`any`

##### Returns

`void`

#### Call Signature

`function error(msg, fields?): void`

Error logs a message at the error level

##### Parameters

###### msg

`string`

###### fields?

`any`

##### Returns

`void`

***

<!-- symbol-end -->

<!-- symbol-start: info() -->
### info()

`function info(msg, fields?): void`

<!-- source: log/mod.ts:173 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L173)

Info logs a message at the info level

#### Parameters

##### msg

`string`

##### fields?

`any`

#### Returns

`void`

***

<!-- symbol-end -->

<!-- symbol-start: trace() -->
### trace()

`function trace(msg, fields?): void`

<!-- source: log/mod.ts:159 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L159)

Trace logs a message at the trace level

#### Parameters

##### msg

`string`

##### fields?

`any`

#### Returns

`void`

***

<!-- symbol-end -->

<!-- symbol-start: warn() -->
### warn()

#### Call Signature

`function warn(err, fields?): void`

Warn logs a message at the warn level

##### Parameters

###### err

`unknown`

###### fields?

`any`

##### Returns

`void`

#### Call Signature

```ts
function warn(
   err, 
   msg, 
   fields?): void;
```

Warn logs a message at the warn level

##### Parameters

###### err

`unknown`

###### msg

`string`

###### fields?

`any`

##### Returns

`void`

#### Call Signature

`function warn(msg, fields?): void`

Warn logs a message at the warn level

##### Parameters

###### msg

`string`

###### fields?

`any`

##### Returns

`void`


<!-- symbol-end -->