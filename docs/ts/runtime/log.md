---
title: encore.dev/log
lang: ts
toc: true
---

# encore.dev/log

## Enumerations

### LogLevel

Defined in: [log/mod.ts:28](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L28)

#### Enumeration Members

##### Debug

```ts
Debug: number;
```

Defined in: [log/mod.ts:30](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L30)

##### Error

```ts
Error: number;
```

Defined in: [log/mod.ts:33](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L33)

##### Info

```ts
Info: number;
```

Defined in: [log/mod.ts:31](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L31)

##### Trace

```ts
Trace: number;
```

Defined in: [log/mod.ts:29](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L29)

##### Warn

```ts
Warn: number;
```

Defined in: [log/mod.ts:32](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L32)

## Interfaces

### Logger

Defined in: [log/mod.ts:36](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L36)

#### Methods

##### debug()

```ts
debug(msg, fields?): void;
```

Defined in: [log/mod.ts:67](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L67)

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

```ts
error(err, fields?): void;
```

Defined in: [log/mod.ts:88](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L88)

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

Defined in: [log/mod.ts:89](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L89)

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

```ts
error(msg, fields?): void;
```

Defined in: [log/mod.ts:90](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L90)

###### Parameters

###### msg

`string`

###### fields?

`any`

###### Returns

`void`

##### info()

```ts
info(msg, fields?): void;
```

Defined in: [log/mod.ts:74](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L74)

Info logs a message at the info level.

###### Parameters

###### msg

`string`

###### fields?

`any`

###### Returns

`void`

##### trace()

```ts
trace(msg, fields?): void;
```

Defined in: [log/mod.ts:60](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L60)

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

```ts
warn(err, fields?): void;
```

Defined in: [log/mod.ts:81](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L81)

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

Defined in: [log/mod.ts:82](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L82)

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

```ts
warn(msg, fields?): void;
```

Defined in: [log/mod.ts:83](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L83)

Warn logs a message at the warn level.

###### Parameters

###### msg

`string`

###### fields?

`any`

###### Returns

`void`

##### with()

```ts
with(fields): Logger;
```

Defined in: [log/mod.ts:53](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L53)

Returns a new logger with the given fields added to the context.

###### Parameters

###### fields

`any`

###### Returns

[`Logger`](#logger)

##### withLevel()

```ts
withLevel(level): Logger;
```

Defined in: [log/mod.ts:46](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L46)

Returns a new logger with the specified level.

###### Parameters

###### level

[`LogLevel`](#loglevel)

###### Returns

[`Logger`](#logger)

## Type Aliases

### FieldsObject

```ts
type FieldsObject = Record<string, FieldValue>;
```

Defined in: [log/mod.ts:26](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L26)

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

Defined in: [log/mod.ts:12](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L12)

A field value we support logging

## Variables

### default

```ts
const default: Logger;
```

Defined in: [log/mod.ts:149](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L149)

## Functions

### debug()

```ts
function debug(msg, fields?): void;
```

Defined in: [log/mod.ts:166](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L166)

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

```ts
function error(err, fields?): void;
```

Defined in: [log/mod.ts:204](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L204)

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

Defined in: [log/mod.ts:205](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L205)

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

```ts
function error(msg, fields?): void;
```

Defined in: [log/mod.ts:210](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L210)

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

```ts
function info(msg, fields?): void;
```

Defined in: [log/mod.ts:173](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L173)

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

```ts
function trace(msg, fields?): void;
```

Defined in: [log/mod.ts:159](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L159)

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

```ts
function warn(err, fields?): void;
```

Defined in: [log/mod.ts:180](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L180)

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

Defined in: [log/mod.ts:181](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L181)

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

```ts
function warn(msg, fields?): void;
```

Defined in: [log/mod.ts:186](https://github.com/encoredev/encore/blob/4043f36cb4a881aeecf61aa6afc4f3c4ec76deca/runtimes/js/encore.dev/log/mod.ts#L186)

Warn logs a message at the warn level

##### Parameters

###### msg

`string`

###### fields?

`any`

##### Returns

`void`
