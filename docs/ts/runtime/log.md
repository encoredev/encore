---
title: encore.dev/log
lang: ts
toc: true
---

## Enumerations

<!-- symbol-start: LogLevel -->
### LogLevel <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L28" target="_blank" rel="noopener">source</a>

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
### Logger <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L36" target="_blank" rel="noopener">source</a>

#### Methods

##### debug() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L67" target="_blank" rel="noopener">source</a>

`debug(msg, fields?): void`

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

##### info() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L74" target="_blank" rel="noopener">source</a>

`info(msg, fields?): void`

Info logs a message at the info level.

###### Parameters

###### msg

`string`

###### fields?

`any`

###### Returns

`void`

##### trace() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L60" target="_blank" rel="noopener">source</a>

`trace(msg, fields?): void`

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

##### with() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L53" target="_blank" rel="noopener">source</a>

`with(fields): Logger`

Returns a new logger with the given fields added to the context.

###### Parameters

###### fields

`any`

###### Returns

[`Logger`](#logger)

##### withLevel() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L46" target="_blank" rel="noopener">source</a>

`withLevel(level): Logger`

Returns a new logger with the specified level.

###### Parameters

###### level

[`LogLevel`](#loglevel)

###### Returns

[`Logger`](#logger)

<!-- symbol-end -->

## Type Aliases

<!-- symbol-start: FieldsObject -->
### FieldsObject <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L26" target="_blank" rel="noopener">source</a>

`type FieldsObject = Record<string, FieldValue>`

A map of fields that can be logged

***

<!-- symbol-end -->

<!-- symbol-start: FieldValue -->
### FieldValue <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L12" target="_blank" rel="noopener">source</a>

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

A field value we support logging

<!-- symbol-end -->

## Variables

<!-- symbol-start: default -->
### default <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L149" target="_blank" rel="noopener">source</a>

`const default: Logger`

<!-- symbol-end -->

## Functions

<!-- symbol-start: debug() -->
### debug() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L166" target="_blank" rel="noopener">source</a>

`function debug(msg, fields?): void`

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
### info() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L173" target="_blank" rel="noopener">source</a>

`function info(msg, fields?): void`

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
### trace() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/log/mod.ts#L159" target="_blank" rel="noopener">source</a>

`function trace(msg, fields?): void`

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