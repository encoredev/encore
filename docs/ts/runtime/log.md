---
title: encore.dev/log
lang: ts
toc: true
---

# encore.dev/log

## Enumerations

### LogLevel

<!-- source: log/mod.ts:28 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L28 -->

#### Enumeration Members

##### Debug

`Debug: number;`

<!-- source: log/mod.ts:30 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L30 -->

##### Error

`Error: number;`

<!-- source: log/mod.ts:33 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L33 -->

##### Info

`Info: number;`

<!-- source: log/mod.ts:31 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L31 -->

##### Trace

`Trace: number;`

<!-- source: log/mod.ts:29 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L29 -->

##### Warn

`Warn: number;`

<!-- source: log/mod.ts:32 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L32 -->

## Interfaces

### Logger

<!-- source: log/mod.ts:36 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L36 -->

#### Methods

##### debug()

`debug(msg, fields?): void;`

<!-- source: log/mod.ts:67 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L67 -->

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

`error(err, fields?): void;`

<!-- source: log/mod.ts:88 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L88 -->

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

<!-- source: log/mod.ts:89 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L89 -->

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

`error(msg, fields?): void;`

<!-- source: log/mod.ts:90 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L90 -->

###### Parameters

###### msg

`string`

###### fields?

`any`

###### Returns

`void`

##### info()

`info(msg, fields?): void;`

<!-- source: log/mod.ts:74 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L74 -->

Info logs a message at the info level.

###### Parameters

###### msg

`string`

###### fields?

`any`

###### Returns

`void`

##### trace()

`trace(msg, fields?): void;`

<!-- source: log/mod.ts:60 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L60 -->

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

`warn(err, fields?): void;`

<!-- source: log/mod.ts:81 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L81 -->

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

<!-- source: log/mod.ts:82 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L82 -->

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

`warn(msg, fields?): void;`

<!-- source: log/mod.ts:83 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L83 -->

Warn logs a message at the warn level.

###### Parameters

###### msg

`string`

###### fields?

`any`

###### Returns

`void`

##### with()

`with(fields): Logger;`

<!-- source: log/mod.ts:53 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L53 -->

Returns a new logger with the given fields added to the context.

###### Parameters

###### fields

`any`

###### Returns

[`Logger`](#logger)

##### withLevel()

`withLevel(level): Logger;`

<!-- source: log/mod.ts:46 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L46 -->

Returns a new logger with the specified level.

###### Parameters

###### level

[`LogLevel`](#loglevel)

###### Returns

[`Logger`](#logger)

## Type Aliases

### FieldsObject

`type FieldsObject = Record<string, FieldValue>;`

<!-- source: log/mod.ts:26 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L26 -->

A map of fields that can be logged

***

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

<!-- source: log/mod.ts:12 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L12 -->

A field value we support logging

## Variables

### default

`const default: Logger;`

<!-- source: log/mod.ts:149 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L149 -->

## Functions

### debug()

`function debug(msg, fields?): void;`

<!-- source: log/mod.ts:166 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L166 -->

Debug logs a message at the debug level

#### Parameters

##### msg

`string`

##### fields?

`any`

#### Returns

`void`

***

### error()

#### Call Signature

`function error(err, fields?): void;`

<!-- source: log/mod.ts:204 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L204 -->

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

<!-- source: log/mod.ts:205 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L205 -->

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

`function error(msg, fields?): void;`

<!-- source: log/mod.ts:210 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L210 -->

Error logs a message at the error level

##### Parameters

###### msg

`string`

###### fields?

`any`

##### Returns

`void`

***

### info()

`function info(msg, fields?): void;`

<!-- source: log/mod.ts:173 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L173 -->

Info logs a message at the info level

#### Parameters

##### msg

`string`

##### fields?

`any`

#### Returns

`void`

***

### trace()

`function trace(msg, fields?): void;`

<!-- source: log/mod.ts:159 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L159 -->

Trace logs a message at the trace level

#### Parameters

##### msg

`string`

##### fields?

`any`

#### Returns

`void`

***

### warn()

#### Call Signature

`function warn(err, fields?): void;`

<!-- source: log/mod.ts:180 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L180 -->

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

<!-- source: log/mod.ts:181 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L181 -->

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

`function warn(msg, fields?): void;`

<!-- source: log/mod.ts:186 url=https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L186 -->

Warn logs a message at the warn level

##### Parameters

###### msg

`string`

###### fields?

`any`

##### Returns

`void`
