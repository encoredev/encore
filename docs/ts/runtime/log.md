---
title: encore.dev/log
lang: ts
toc: true
---

# encore.dev/log

## Enumerations

<!-- symbol-start: LogLevel -->
### LogLevel

<!-- source: log/mod.ts:28 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L28)

#### Enumeration Members

##### Debug

`Debug: number`

<!-- source: log/mod.ts:30 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L30)

##### Error

`Error: number`

<!-- source: log/mod.ts:33 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L33)

##### Info

`Info: number`

<!-- source: log/mod.ts:31 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L31)

##### Trace

`Trace: number`

<!-- source: log/mod.ts:29 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L29)

##### Warn

`Warn: number`

<!-- source: log/mod.ts:32 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L32)

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

<!-- source: log/mod.ts:88 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L88)

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

<!-- source: log/mod.ts:89 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L89)

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

<!-- source: log/mod.ts:90 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L90)

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

<!-- source: log/mod.ts:81 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L81)

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

<!-- source: log/mod.ts:82 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L82)

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

<!-- source: log/mod.ts:83 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L83)

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

<!-- source: log/mod.ts:204 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L204)

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

<!-- source: log/mod.ts:205 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L205)

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

<!-- source: log/mod.ts:210 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L210)

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

<!-- source: log/mod.ts:180 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L180)

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

<!-- source: log/mod.ts:181 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L181)

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

<!-- source: log/mod.ts:186 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L186)

Warn logs a message at the warn level

##### Parameters

###### msg

`string`

###### fields?

`any`

##### Returns

`void`


<!-- symbol-end -->