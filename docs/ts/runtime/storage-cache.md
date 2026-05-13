---
title: encore.dev/storage/cache
lang: ts
toc: true
---

# encore.dev/storage/cache

## Classes

<!-- symbol-start: CacheCluster -->
### CacheCluster

<!-- source: storage/cache/cluster.ts:43 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/cluster.ts#L43)

CacheCluster represents a Redis cache cluster.

Create a new cluster using `new CacheCluster(name)`.
Reference an existing cluster using `CacheCluster.named(name)`.

#### Example

```ts
import { CacheCluster } from "encore.dev/storage/cache";

const myCache = new CacheCluster("my-cache", {
  evictionPolicy: "allkeys-lru",
});
```

#### Constructors

##### Constructor

`new CacheCluster(name, cfg?): CacheCluster`

<!-- source: storage/cache/cluster.ts:55 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/cluster.ts#L55)

Creates a new cache cluster with the given name and configuration.

###### Parameters

###### name

`string`

The unique name for this cache cluster

###### cfg?

[`CacheClusterConfig`](#cacheclusterconfig)

Optional configuration for the cluster

###### Returns

[`CacheCluster`](#cachecluster)

#### Methods

##### named()

`static named<Name>(name): CacheCluster`

<!-- source: storage/cache/cluster.ts:64 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/cluster.ts#L64)

Reference an existing cache cluster by name.
To create a new cache cluster, use `new CacheCluster(...)` instead.

###### Type Parameters

###### Name

`Name` *extends* `string`

###### Parameters

###### name

`StringLiteral`\<`Name`\>

###### Returns

[`CacheCluster`](#cachecluster)

***

<!-- symbol-end -->

<!-- symbol-start: CacheError -->
### CacheError

<!-- source: storage/cache/errors.ts:4 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/errors.ts#L4)

CacheError is the base class for all cache-related errors.

#### Extends

- `Error`

#### Extended by

- [`CacheMiss`](#cachemiss)
- [`CacheKeyExists`](#cachekeyexists)

#### Constructors

##### Constructor

`new CacheError(msg): CacheError`

<!-- source: storage/cache/errors.ts:5 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/errors.ts#L5)

###### Parameters

###### msg

`string`

###### Returns

[`CacheError`](#cacheerror)

###### Overrides

`Error.constructor`

***

<!-- symbol-end -->

<!-- symbol-start: CacheKeyExists -->
### CacheKeyExists

<!-- source: storage/cache/errors.ts:49 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/errors.ts#L49)

CacheKeyExists is thrown when attempting to set a key that already exists
using setIfNotExists.

#### Extends

- [`CacheError`](#cacheerror)

#### Constructors

##### Constructor

`new CacheKeyExists(key): CacheKeyExists`

<!-- source: storage/cache/errors.ts:50 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/errors.ts#L50)

###### Parameters

###### key

`string`

###### Returns

[`CacheKeyExists`](#cachekeyexists)

###### Overrides

[`CacheError`](#cacheerror).[`constructor`](#constructor-1)

***

<!-- symbol-end -->

<!-- symbol-start: CacheMiss -->
### CacheMiss

<!-- source: storage/cache/errors.ts:26 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/errors.ts#L26)

CacheMiss is thrown when a cache key is not found.

#### Extends

- [`CacheError`](#cacheerror)

#### Constructors

##### Constructor

`new CacheMiss(key): CacheMiss`

<!-- source: storage/cache/errors.ts:27 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/errors.ts#L27)

###### Parameters

###### key

`string`

###### Returns

[`CacheMiss`](#cachemiss)

###### Overrides

[`CacheError`](#cacheerror).[`constructor`](#constructor-1)

***

<!-- symbol-end -->

<!-- symbol-start: FloatKeyspace -->
### FloatKeyspace

<!-- source: storage/cache/basic.ts:410 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L410)

FloatKeyspace stores 64-bit floating point values.

#### Example

```ts
const scores = new FloatKeyspace<string>(cluster, {
  keyPattern: "score/:playerId",
});

await scores.set("player1", 100.5);
const newScore = await scores.increment("player1", 10.25);
```

#### Extends

- `BasicKeyspace`\<`K`, `number`\>

#### Type Parameters

##### K

`K`

#### Constructors

##### Constructor

`new FloatKeyspace<K>(cluster, config): FloatKeyspace<K>`

<!-- source: storage/cache/basic.ts:411 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L411)

###### Parameters

###### cluster

[`CacheCluster`](#cachecluster)

###### config

[`KeyspaceConfig`](#keyspaceconfig)\<`K`\>

###### Returns

[`FloatKeyspace`](#floatkeyspace)\<`K`\>

###### Overrides

`BasicKeyspace<K, number>.constructor`

#### Properties

##### cluster

`protected readonly cluster: CacheCluster`

<!-- source: storage/cache/keyspace.ts:46 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L46)

###### Inherited from

`BasicKeyspace.cluster`

##### config

`protected readonly config: KeyspaceConfig<K>`

<!-- source: storage/cache/keyspace.ts:47 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L47)

###### Inherited from

`BasicKeyspace.config`

##### keyMapper

`protected readonly keyMapper: (key) => string`

<!-- source: storage/cache/keyspace.ts:48 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L48)

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`BasicKeyspace.keyMapper`

#### Methods

##### decrement()

```ts
decrement(
   key, 
   delta?, 
options?): Promise<number>;
```

<!-- source: storage/cache/basic.ts:462 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L462)

Decrements the number stored at key by `delta`.

If the key does not exist it is first created with a value of 0
before decrementing.

Negative values can be used to increase the value,
but typically you want to use [increment](#increment) for that.

###### Parameters

###### key

`K`

The cache key.

###### delta?

`number` = `1`

The amount to decrement by (default 1).

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number`\>

The new value after decrementing.

###### See

https://redis.io/commands/incrbyfloat/

##### delete()

`delete(...keys): Promise<number>`

<!-- source: storage/cache/keyspace.ts:137 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L137)

Deletes the specified keys.
If a key does not exist it is ignored.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<`number`\>

The number of keys that were deleted.

###### See

https://redis.io/commands/del/

###### Inherited from

`BasicKeyspace.delete`

##### deserialize()

`protected deserialize(data): number`

<!-- source: storage/cache/basic.ts:419 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L419)

Deserializes a Buffer from storage to a value.

###### Parameters

###### data

`Buffer`

###### Returns

`number`

###### Overrides

`BasicKeyspace.deserialize`

##### get()

`get(key): Promise<number | undefined>`

<!-- source: storage/cache/basic.ts:33 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L33)

Gets the value stored at key.
If the key does not exist, it returns `undefined`.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`number` \| `undefined`\>

The value, or `undefined` if the key does not exist.

###### See

https://redis.io/commands/get/

###### Inherited from

`BasicKeyspace.get`

##### getAndDelete()

`getAndDelete(key): Promise<number | undefined>`

<!-- source: storage/cache/basic.ts:165 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L165)

Deletes the key and returns the previously stored value.
If the key does not already exist, it returns `undefined`.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`number` \| `undefined`\>

The previous value, or `undefined` if the key did not exist.

###### See

https://redis.io/commands/getdel/

###### Inherited from

`BasicKeyspace.getAndDelete`

##### getAndSet()

```ts
getAndSet(
   key, 
   value, 
options?): Promise<number | undefined>;
```

<!-- source: storage/cache/basic.ts:134 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L134)

Updates the value of key to val and returns the previously stored value.
If the key does not already exist, it sets it and returns `undefined`.

###### Parameters

###### key

`K`

###### value

`number`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number` \| `undefined`\>

The previous value, or `undefined` if the key did not exist.

###### See

https://redis.io/commands/getset/

###### Inherited from

`BasicKeyspace.getAndSet`

##### increment()

```ts
increment(
   key, 
   delta?, 
options?): Promise<number>;
```

<!-- source: storage/cache/basic.ts:437 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L437)

Increments the number stored at key by `delta`.

If the key does not exist it is first created with a value of 0
before incrementing.

Negative values can be used to decrease the value,
but typically you want to use [decrement](#decrement) for that.

###### Parameters

###### key

`K`

The cache key.

###### delta?

`number` = `1`

The amount to increment by (default 1).

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number`\>

The new value after incrementing.

###### See

https://redis.io/commands/incrbyfloat/

##### mapKey()

`protected mapKey(key): string`

<!-- source: storage/cache/keyspace.ts:91 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L91)

Maps a key to its Redis key string.

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`BasicKeyspace.mapKey`

##### multiGet()

`multiGet(...keys): Promise<(number | undefined)[]>`

<!-- source: storage/cache/basic.ts:52 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L52)

Gets the values stored at multiple keys.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<(`number` \| `undefined`)[]\>

An array of values in the same order as the provided keys.
Each element is the value or `undefined` if the key was not found.

###### See

https://redis.io/commands/mget/

###### Inherited from

`BasicKeyspace.multiGet`

##### replace()

```ts
replace(
   key, 
   value, 
options?): Promise<void>;
```

<!-- source: storage/cache/basic.ts:109 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L109)

Replaces the existing value stored at key to val.

###### Parameters

###### key

`K`

###### value

`number`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`void`\>

###### Throws

If the key does not already exist.

###### See

https://redis.io/commands/set/

###### Inherited from

`BasicKeyspace.replace`

##### resolveTtl()

`protected resolveTtl(options?): number | undefined`

<!-- source: storage/cache/keyspace.ts:103 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L103)

Resolves the TTL for a write operation.
Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms

###### Parameters

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`number` \| `undefined`

###### Inherited from

`BasicKeyspace.resolveTtl`

##### serialize()

`protected serialize(value): Buffer`

<!-- source: storage/cache/basic.ts:415 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L415)

Serializes a value to a Buffer for storage.

###### Parameters

###### value

`number`

###### Returns

`Buffer`

###### Overrides

`BasicKeyspace.serialize`

##### set()

```ts
set(
   key, 
   value, 
options?): Promise<void>;
```

<!-- source: storage/cache/basic.ts:66 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L66)

Updates the value stored at key to val.

###### Parameters

###### key

`K`

###### value

`number`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`void`\>

###### See

https://redis.io/commands/set/

###### Inherited from

`BasicKeyspace.set`

##### setIfNotExists()

```ts
setIfNotExists(
   key, 
   value, 
options?): Promise<void>;
```

<!-- source: storage/cache/basic.ts:81 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L81)

Sets the value stored at key to val, but only if the key does not exist beforehand.

###### Parameters

###### key

`K`

###### value

`number`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`void`\>

###### Throws

If the key already exists.

###### See

https://redis.io/commands/setnx/

###### Inherited from

`BasicKeyspace.setIfNotExists`

##### with()

`with(options): this`

<!-- source: storage/cache/keyspace.ts:123 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L123)

Returns a shallow clone of this keyspace with the specified write options applied.
This allows setting expiry for a chain of operations.

###### Parameters

###### options

[`WriteOptions`](#writeoptions)

###### Returns

`this`

###### Example

`await myKeyspace.with({ expiry: expireIn(5000) }).set(key, value)`

###### Inherited from

`BasicKeyspace.with`

***

<!-- symbol-end -->

<!-- symbol-start: IntKeyspace -->
### IntKeyspace

<!-- source: storage/cache/basic.ts:323 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L323)

IntKeyspace stores 64-bit integer values.
Values are floored to integers using `Math.floor`.
For fractional values, use [FloatKeyspace](#floatkeyspace) instead.

#### Example

```ts
const counters = new IntKeyspace<string>(cluster, {
  keyPattern: "counter/:name",
});

await counters.set("page-views", 0);
const newCount = await counters.increment("page-views", 1);
```

#### Extends

- `BasicKeyspace`\<`K`, `number`\>

#### Type Parameters

##### K

`K`

#### Constructors

##### Constructor

`new IntKeyspace<K>(cluster, config): IntKeyspace<K>`

<!-- source: storage/cache/basic.ts:324 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L324)

###### Parameters

###### cluster

[`CacheCluster`](#cachecluster)

###### config

[`KeyspaceConfig`](#keyspaceconfig)\<`K`\>

###### Returns

[`IntKeyspace`](#intkeyspace)\<`K`\>

###### Overrides

`BasicKeyspace<K, number>.constructor`

#### Properties

##### cluster

`protected readonly cluster: CacheCluster`

<!-- source: storage/cache/keyspace.ts:46 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L46)

###### Inherited from

`BasicKeyspace.cluster`

##### config

`protected readonly config: KeyspaceConfig<K>`

<!-- source: storage/cache/keyspace.ts:47 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L47)

###### Inherited from

`BasicKeyspace.config`

##### keyMapper

`protected readonly keyMapper: (key) => string`

<!-- source: storage/cache/keyspace.ts:48 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L48)

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`BasicKeyspace.keyMapper`

#### Methods

##### decrement()

```ts
decrement(
   key, 
   delta?, 
options?): Promise<number>;
```

<!-- source: storage/cache/basic.ts:380 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L380)

Decrements the number stored at key by `delta`.

If the key does not exist it is first created with a value of 0
before decrementing.

Negative values can be used to increase the value,
but typically you want to use [increment](#increment-1) for that.

###### Parameters

###### key

`K`

The cache key.

###### delta?

`number` = `1`

The amount to decrement by (default 1).

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number`\>

The new value after decrementing.

###### See

https://redis.io/commands/decrby/

##### delete()

`delete(...keys): Promise<number>`

<!-- source: storage/cache/keyspace.ts:137 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L137)

Deletes the specified keys.
If a key does not exist it is ignored.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<`number`\>

The number of keys that were deleted.

###### See

https://redis.io/commands/del/

###### Inherited from

`BasicKeyspace.delete`

##### deserialize()

`protected deserialize(data): number`

<!-- source: storage/cache/basic.ts:332 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L332)

Deserializes a Buffer from storage to a value.

###### Parameters

###### data

`Buffer`

###### Returns

`number`

###### Overrides

`BasicKeyspace.deserialize`

##### get()

`get(key): Promise<number | undefined>`

<!-- source: storage/cache/basic.ts:33 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L33)

Gets the value stored at key.
If the key does not exist, it returns `undefined`.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`number` \| `undefined`\>

The value, or `undefined` if the key does not exist.

###### See

https://redis.io/commands/get/

###### Inherited from

`BasicKeyspace.get`

##### getAndDelete()

`getAndDelete(key): Promise<number | undefined>`

<!-- source: storage/cache/basic.ts:165 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L165)

Deletes the key and returns the previously stored value.
If the key does not already exist, it returns `undefined`.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`number` \| `undefined`\>

The previous value, or `undefined` if the key did not exist.

###### See

https://redis.io/commands/getdel/

###### Inherited from

`BasicKeyspace.getAndDelete`

##### getAndSet()

```ts
getAndSet(
   key, 
   value, 
options?): Promise<number | undefined>;
```

<!-- source: storage/cache/basic.ts:134 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L134)

Updates the value of key to val and returns the previously stored value.
If the key does not already exist, it sets it and returns `undefined`.

###### Parameters

###### key

`K`

###### value

`number`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number` \| `undefined`\>

The previous value, or `undefined` if the key did not exist.

###### See

https://redis.io/commands/getset/

###### Inherited from

`BasicKeyspace.getAndSet`

##### increment()

```ts
increment(
   key, 
   delta?, 
options?): Promise<number>;
```

<!-- source: storage/cache/basic.ts:350 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L350)

Increments the number stored at key by `delta`.

If the key does not exist it is first created with a value of 0
before incrementing.

Negative values can be used to decrease the value,
but typically you want to use [decrement](#decrement-1) for that.

###### Parameters

###### key

`K`

The cache key.

###### delta?

`number` = `1`

The amount to increment by (default 1).

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number`\>

The new value after incrementing.

###### See

https://redis.io/commands/incrby/

##### mapKey()

`protected mapKey(key): string`

<!-- source: storage/cache/keyspace.ts:91 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L91)

Maps a key to its Redis key string.

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`BasicKeyspace.mapKey`

##### multiGet()

`multiGet(...keys): Promise<(number | undefined)[]>`

<!-- source: storage/cache/basic.ts:52 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L52)

Gets the values stored at multiple keys.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<(`number` \| `undefined`)[]\>

An array of values in the same order as the provided keys.
Each element is the value or `undefined` if the key was not found.

###### See

https://redis.io/commands/mget/

###### Inherited from

`BasicKeyspace.multiGet`

##### replace()

```ts
replace(
   key, 
   value, 
options?): Promise<void>;
```

<!-- source: storage/cache/basic.ts:109 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L109)

Replaces the existing value stored at key to val.

###### Parameters

###### key

`K`

###### value

`number`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`void`\>

###### Throws

If the key does not already exist.

###### See

https://redis.io/commands/set/

###### Inherited from

`BasicKeyspace.replace`

##### resolveTtl()

`protected resolveTtl(options?): number | undefined`

<!-- source: storage/cache/keyspace.ts:103 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L103)

Resolves the TTL for a write operation.
Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms

###### Parameters

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`number` \| `undefined`

###### Inherited from

`BasicKeyspace.resolveTtl`

##### serialize()

`protected serialize(value): Buffer`

<!-- source: storage/cache/basic.ts:328 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L328)

Serializes a value to a Buffer for storage.

###### Parameters

###### value

`number`

###### Returns

`Buffer`

###### Overrides

`BasicKeyspace.serialize`

##### set()

```ts
set(
   key, 
   value, 
options?): Promise<void>;
```

<!-- source: storage/cache/basic.ts:66 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L66)

Updates the value stored at key to val.

###### Parameters

###### key

`K`

###### value

`number`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`void`\>

###### See

https://redis.io/commands/set/

###### Inherited from

`BasicKeyspace.set`

##### setIfNotExists()

```ts
setIfNotExists(
   key, 
   value, 
options?): Promise<void>;
```

<!-- source: storage/cache/basic.ts:81 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L81)

Sets the value stored at key to val, but only if the key does not exist beforehand.

###### Parameters

###### key

`K`

###### value

`number`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`void`\>

###### Throws

If the key already exists.

###### See

https://redis.io/commands/setnx/

###### Inherited from

`BasicKeyspace.setIfNotExists`

##### with()

`with(options): this`

<!-- source: storage/cache/keyspace.ts:123 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L123)

Returns a shallow clone of this keyspace with the specified write options applied.
This allows setting expiry for a chain of operations.

###### Parameters

###### options

[`WriteOptions`](#writeoptions)

###### Returns

`this`

###### Example

`await myKeyspace.with({ expiry: expireIn(5000) }).set(key, value)`

###### Inherited from

`BasicKeyspace.with`

***

<!-- symbol-end -->

<!-- symbol-start: NumberListKeyspace -->
### NumberListKeyspace

<!-- source: storage/cache/list.ts:484 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L484)

NumberListKeyspace stores lists of numeric values.

#### Example

```ts
const scores = new NumberListKeyspace<string>(cluster, {
  keyPattern: "scores/:gameId",
});

await scores.pushRight("game1", 100, 200, 300);
const allScores = await scores.items("game1");
```

#### Extends

- `ListKeyspace`\<`K`, `number`\>

#### Type Parameters

##### K

`K`

#### Constructors

##### Constructor

`new NumberListKeyspace<K>(cluster, config): NumberListKeyspace<K>`

<!-- source: storage/cache/list.ts:485 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L485)

###### Parameters

###### cluster

[`CacheCluster`](#cachecluster)

###### config

[`KeyspaceConfig`](#keyspaceconfig)\<`K`\>

###### Returns

[`NumberListKeyspace`](#numberlistkeyspace)\<`K`\>

###### Overrides

`ListKeyspace<K, number>.constructor`

#### Properties

##### cluster

`protected readonly cluster: CacheCluster`

<!-- source: storage/cache/keyspace.ts:46 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L46)

###### Inherited from

`ListKeyspace.cluster`

##### config

`protected readonly config: KeyspaceConfig<K>`

<!-- source: storage/cache/keyspace.ts:47 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L47)

###### Inherited from

`ListKeyspace.config`

##### keyMapper

`protected readonly keyMapper: (key) => string`

<!-- source: storage/cache/keyspace.ts:48 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L48)

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`ListKeyspace.keyMapper`

#### Methods

##### delete()

`delete(...keys): Promise<number>`

<!-- source: storage/cache/keyspace.ts:137 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L137)

Deletes the specified keys.
If a key does not exist it is ignored.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<`number`\>

The number of keys that were deleted.

###### See

https://redis.io/commands/del/

###### Inherited from

`ListKeyspace.delete`

##### deserializeItem()

`protected deserializeItem(data): number`

<!-- source: storage/cache/list.ts:493 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L493)

###### Parameters

###### data

`Buffer`

###### Returns

`number`

###### Overrides

`ListKeyspace.deserializeItem`

##### get()

`get(key, index): Promise<number | undefined>`

<!-- source: storage/cache/list.ts:187 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L187)

Returns the value of the list element at the given index.

Negative indices can be used to indicate offsets from the end of the list,
where -1 is the last element of the list, -2 the penultimate element, and so on.

###### Parameters

###### key

`K`

The cache key.

###### index

`number`

Zero-based index of the element to retrieve.

###### Returns

`Promise`\<`number` \| `undefined`\>

The value at the index, or `undefined` if out of range or the key does not exist.

###### See

https://redis.io/commands/lindex/

###### Inherited from

`ListKeyspace.get`

##### getRange()

```ts
getRange(
   key, 
   start, 
stop): Promise<number[]>;
```

<!-- source: storage/cache/list.ts:229 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L229)

Returns the elements in the list stored at key between `start` and `stop` (inclusive).
Both are zero-based indices.

Negative indices can be used to indicate offsets from the end of the list,
where -1 is the last element of the list, -2 the penultimate element, and so on.

If the key does not exist it returns an empty array.

###### Parameters

###### key

`K`

The cache key.

###### start

`number`

Start index (inclusive).

###### stop

`number`

Stop index (inclusive).

###### Returns

`Promise`\<`number`[]\>

The elements in the specified range.

###### See

https://redis.io/commands/lrange/

###### Inherited from

`ListKeyspace.getRange`

##### insertAfter()

```ts
insertAfter(
   key, 
   pivot, 
   value, 
options?): Promise<number>;
```

<!-- source: storage/cache/list.ts:284 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L284)

Inserts `value` into the list stored at key, at the position just after `pivot`.

If the list does not contain `pivot`, the value is not inserted and -1 is returned.

###### Parameters

###### key

`K`

The cache key.

###### pivot

`number`

The existing element to insert after.

###### value

`number`

The value to insert.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number`\>

The new list length, or -1 if `pivot` was not found.

###### See

https://redis.io/commands/linsert/

###### Inherited from

`ListKeyspace.insertAfter`

##### insertBefore()

```ts
insertBefore(
   key, 
   pivot, 
   value, 
options?): Promise<number>;
```

<!-- source: storage/cache/list.ts:252 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L252)

Inserts `value` into the list stored at key, at the position just before `pivot`.

If the list does not contain `pivot`, the value is not inserted and -1 is returned.

###### Parameters

###### key

`K`

The cache key.

###### pivot

`number`

The existing element to insert before.

###### value

`number`

The value to insert.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number`\>

The new list length, or -1 if `pivot` was not found.

###### See

https://redis.io/commands/linsert/

###### Inherited from

`ListKeyspace.insertBefore`

##### items()

`items(key): Promise<number[]>`

<!-- source: storage/cache/list.ts:207 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L207)

Returns all the elements in the list stored at key.

If the key does not exist it returns an empty array.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`number`[]\>

All elements in the list.

###### See

https://redis.io/commands/lrange/

###### Inherited from

`ListKeyspace.items`

##### len()

`len(key): Promise<number>`

<!-- source: storage/cache/list.ts:117 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L117)

Returns the length of the list stored at key.

Non-existing keys are considered as empty lists.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`number`\>

The list length.

###### See

https://redis.io/commands/llen/

###### Inherited from

`ListKeyspace.len`

##### mapKey()

`protected mapKey(key): string`

<!-- source: storage/cache/keyspace.ts:91 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L91)

Maps a key to its Redis key string.

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`ListKeyspace.mapKey`

##### move()

```ts
move(
   src, 
   dst, 
   srcPos, 
   dstPos, 
options?): Promise<number | undefined>;
```

<!-- source: storage/cache/list.ts:417 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L417)

Atomically moves an element from the list stored at `src` to the list stored at `dst`.

The value moved can be either the head (`srcPos === "left"`) or tail (`srcPos === "right"`)
of the list at `src`. Similarly, the value can be placed either at the head (`dstPos === "left"`)
or tail (`dstPos === "right"`) of the list at `dst`.

If `src` and `dst` are the same list, the value is atomically rotated from one end to the other
when `srcPos !== dstPos`, or if `srcPos === dstPos` nothing happens.

###### Parameters

###### src

`K`

Source list key.

###### dst

`K`

Destination list key.

###### srcPos

[`ListPosition`](#listposition)

Position to pop from in the source list.

###### dstPos

[`ListPosition`](#listposition)

Position to push to in the destination list.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number` \| `undefined`\>

The moved element, or `undefined` if the source list does not exist.

###### See

https://redis.io/commands/lmove/

###### Inherited from

`ListKeyspace.move`

##### popLeft()

`popLeft(key, options?): Promise<number | undefined>`

<!-- source: storage/cache/list.ts:81 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L81)

Pops a single element off the head of the list stored at key.

###### Parameters

###### key

`K`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number` \| `undefined`\>

The popped value, or `undefined` if the key does not exist.

###### See

https://redis.io/commands/lpop/

###### Inherited from

`ListKeyspace.popLeft`

##### popRight()

`popRight(key, options?): Promise<number | undefined>`

<!-- source: storage/cache/list.ts:98 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L98)

Pops a single element off the tail of the list stored at key.

###### Parameters

###### key

`K`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number` \| `undefined`\>

The popped value, or `undefined` if the key does not exist.

###### See

https://redis.io/commands/rpop/

###### Inherited from

`ListKeyspace.popRight`

##### pushLeft()

`pushLeft(key, ...values): Promise<number>`

<!-- source: storage/cache/list.ts:35 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L35)

Pushes one or more values at the head of the list stored at key.
If the key does not already exist, it is first created as an empty list.

If multiple values are given, they are inserted one after another,
starting with the leftmost value. For instance,
`pushLeft(key, "a", "b", "c")` will result in a list containing
"c" as its first element, "b" as its second, and "a" as its third.

###### Parameters

###### key

`K`

###### values

...`number`[]

###### Returns

`Promise`\<`number`\>

The length of the list after the operation.

###### See

https://redis.io/commands/lpush/

###### Inherited from

`ListKeyspace.pushLeft`

##### pushRight()

`pushRight(key, ...values): Promise<number>`

<!-- source: storage/cache/list.ts:61 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L61)

Pushes one or more values at the tail of the list stored at key.
If the key does not already exist, it is first created as an empty list.

If multiple values are given, they are inserted one after another,
starting with the leftmost value. For instance,
`pushRight(key, "a", "b", "c")` will result in a list containing
"a" as its first element, "b" as its second, and "c" as its third.

###### Parameters

###### key

`K`

###### values

...`number`[]

###### Returns

`Promise`\<`number`\>

The length of the list after the operation.

###### See

https://redis.io/commands/rpush/

###### Inherited from

`ListKeyspace.pushRight`

##### removeAll()

```ts
removeAll(
   key, 
   value, 
options?): Promise<number>;
```

<!-- source: storage/cache/list.ts:315 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L315)

Removes all occurrences of `value` in the list stored at key.

If the list does not contain `value`, or the list does not exist, returns 0.

###### Parameters

###### key

`K`

The cache key.

###### value

`number`

The value to remove.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number`\>

The number of elements removed.

###### See

https://redis.io/commands/lrem/

###### Inherited from

`ListKeyspace.removeAll`

##### removeFirst()

```ts
removeFirst(
   key, 
   count, 
   value, 
options?): Promise<number>;
```

<!-- source: storage/cache/list.ts:341 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L341)

Removes the first `count` occurrences of `value` in the list stored at key,
scanning from head to tail.

If the list does not contain `value`, or the list does not exist, returns 0.

###### Parameters

###### key

`K`

The cache key.

###### count

`number`

Maximum number of occurrences to remove.

###### value

`number`

The value to remove.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number`\>

The number of elements removed.

###### See

https://redis.io/commands/lrem/

###### Inherited from

`ListKeyspace.removeFirst`

##### removeLast()

```ts
removeLast(
   key, 
   count, 
   value, 
options?): Promise<number>;
```

<!-- source: storage/cache/list.ts:376 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L376)

Removes the last `count` occurrences of `value` in the list stored at key,
scanning from tail to head.

If the list does not contain `value`, or the list does not exist, returns 0.

###### Parameters

###### key

`K`

The cache key.

###### count

`number`

Maximum number of occurrences to remove.

###### value

`number`

The value to remove.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number`\>

The number of elements removed.

###### See

https://redis.io/commands/lrem/

###### Inherited from

`ListKeyspace.removeLast`

##### resolveTtl()

`protected resolveTtl(options?): number | undefined`

<!-- source: storage/cache/keyspace.ts:103 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L103)

Resolves the TTL for a write operation.
Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms

###### Parameters

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`number` \| `undefined`

###### Inherited from

`ListKeyspace.resolveTtl`

##### serializeItem()

`protected serializeItem(value): Buffer`

<!-- source: storage/cache/list.ts:489 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L489)

###### Parameters

###### value

`number`

###### Returns

`Buffer`

###### Overrides

`ListKeyspace.serializeItem`

##### set()

```ts
set(
   key, 
   index, 
   value, 
options?): Promise<void>;
```

<!-- source: storage/cache/list.ts:163 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L163)

Updates the list element at the given index.

Negative indices can be used to indicate offsets from the end of the list,
where -1 is the last element of the list, -2 the penultimate element, and so on.

###### Parameters

###### key

`K`

The cache key.

###### index

`number`

Zero-based index of the element to update.

###### value

`number`

The new value.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`void`\>

###### Throws

If the index is out of range.

###### See

https://redis.io/commands/lset/

###### Inherited from

`ListKeyspace.set`

##### trim()

```ts
trim(
   key, 
   start, 
   stop, 
options?): Promise<void>;
```

<!-- source: storage/cache/list.ts:139 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L139)

Trims the list stored at key to only contain the elements between the indices
`start` and `stop` (inclusive). Both are zero-based indices.

Negative indices can be used to indicate offsets from the end of the list,
where -1 is the last element of the list, -2 the penultimate element, and so on.

Out of range indices are valid and are treated as if they specify the start or end of the list,
respectively. If `start` > `stop` the end result is an empty list.

###### Parameters

###### key

`K`

The cache key.

###### start

`number`

Start index (inclusive).

###### stop

`number`

Stop index (inclusive).

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`void`\>

###### See

https://redis.io/commands/ltrim/

###### Inherited from

`ListKeyspace.trim`

##### with()

`with(options): this`

<!-- source: storage/cache/keyspace.ts:123 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L123)

Returns a shallow clone of this keyspace with the specified write options applied.
This allows setting expiry for a chain of operations.

###### Parameters

###### options

[`WriteOptions`](#writeoptions)

###### Returns

`this`

###### Example

`await myKeyspace.with({ expiry: expireIn(5000) }).set(key, value)`

###### Inherited from

`ListKeyspace.with`

***

<!-- symbol-end -->

<!-- symbol-start: NumberSetKeyspace -->
### NumberSetKeyspace

<!-- source: storage/cache/set.ts:479 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L479)

NumberSetKeyspace stores sets of unique numeric values.

#### Example

```ts
const scores = new NumberSetKeyspace<string>(cluster, {
  keyPattern: "unique-scores/:gameId",
});

await scores.add("game1", 100, 200, 300);
const hasScore = await scores.contains("game1", 100);
```

#### Extends

- `SetKeyspace`\<`K`, `number`\>

#### Type Parameters

##### K

`K`

#### Constructors

##### Constructor

`new NumberSetKeyspace<K>(cluster, config): NumberSetKeyspace<K>`

<!-- source: storage/cache/set.ts:480 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L480)

###### Parameters

###### cluster

[`CacheCluster`](#cachecluster)

###### config

[`KeyspaceConfig`](#keyspaceconfig)\<`K`\>

###### Returns

[`NumberSetKeyspace`](#numbersetkeyspace)\<`K`\>

###### Overrides

`SetKeyspace<K, number>.constructor`

#### Properties

##### cluster

`protected readonly cluster: CacheCluster`

<!-- source: storage/cache/keyspace.ts:46 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L46)

###### Inherited from

`SetKeyspace.cluster`

##### config

`protected readonly config: KeyspaceConfig<K>`

<!-- source: storage/cache/keyspace.ts:47 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L47)

###### Inherited from

`SetKeyspace.config`

##### keyMapper

`protected readonly keyMapper: (key) => string`

<!-- source: storage/cache/keyspace.ts:48 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L48)

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`SetKeyspace.keyMapper`

#### Methods

##### add()

`add(key, ...members): Promise<number>`

<!-- source: storage/cache/set.ts:26 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L26)

Adds one or more values to the set stored at key.
If the key does not already exist, it is first created as an empty set.

###### Parameters

###### key

`K`

###### members

...`number`[]

###### Returns

`Promise`\<`number`\>

The number of values that were added to the set,
not including values already present beforehand.

###### See

https://redis.io/commands/sadd/

###### Inherited from

`SetKeyspace.add`

##### contains()

`contains(key, member): Promise<boolean>`

<!-- source: storage/cache/set.ts:111 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L111)

Reports whether the set stored at key contains the given value.

If the key does not exist it returns `false`.

###### Parameters

###### key

`K`

###### member

`number`

###### Returns

`Promise`\<`boolean`\>

`true` if the member exists in the set, `false` otherwise.

###### See

https://redis.io/commands/sismember/

###### Inherited from

`SetKeyspace.contains`

##### delete()

`delete(...keys): Promise<number>`

<!-- source: storage/cache/keyspace.ts:137 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L137)

Deletes the specified keys.
If a key does not exist it is ignored.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<`number`\>

The number of keys that were deleted.

###### See

https://redis.io/commands/del/

###### Inherited from

`SetKeyspace.delete`

##### deserializeItem()

`protected deserializeItem(data): number`

<!-- source: storage/cache/set.ts:488 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L488)

###### Parameters

###### data

`Buffer`

###### Returns

`number`

###### Overrides

`SetKeyspace.deserializeItem`

##### diff()

`diff(...keys): Promise<number[]>`

<!-- source: storage/cache/set.ts:174 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L174)

Computes the set difference between the first set and all the consecutive sets.

Set difference means the values present in the first set that are not present
in any of the other sets.

Keys that do not exist are considered as empty sets.

###### Parameters

###### keys

...`K`[]

Keys of sets to compute difference for. At least one must be provided.

###### Returns

`Promise`\<`number`[]\>

Members in the first set but not in any of the other sets.

###### Throws

If no keys are provided.

###### See

https://redis.io/commands/sdiff/

###### Inherited from

`SetKeyspace.diff`

##### diffSet()

`diffSet(...keys): Promise<Set<number>>`

<!-- source: storage/cache/set.ts:189 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L189)

Identical to [diff](#diff) except it returns the values as a `Set`.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<`Set`\<`number`\>\>

###### See

https://redis.io/commands/sdiff/

###### Inherited from

`SetKeyspace.diffSet`

##### diffStore()

`diffStore(destination, ...keys): Promise<number>`

<!-- source: storage/cache/set.ts:203 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L203)

Computes the set difference between keys (like [diff](#diff)) and stores the result
in `destination`.

###### Parameters

###### destination

`K`

Key to store the result.

###### keys

...`K`[]

Keys of sets to compute difference for.

###### Returns

`Promise`\<`number`\>

The size of the resulting set.

###### See

https://redis.io/commands/sdiffstore/

###### Inherited from

`SetKeyspace.diffStore`

##### intersect()

`intersect(...keys): Promise<number[]>`

<!-- source: storage/cache/set.ts:233 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L233)

Computes the set intersection between the sets stored at the given keys.

Set intersection means the values common to all the provided sets.

Keys that do not exist are considered to be empty sets.
As a result, if any key is missing the final result is the empty set.

###### Parameters

###### keys

...`K`[]

Keys of sets to compute intersection for. At least one must be provided.

###### Returns

`Promise`\<`number`[]\>

Members common to all sets.

###### Throws

If no keys are provided.

###### See

https://redis.io/commands/sinter/

###### Inherited from

`SetKeyspace.intersect`

##### intersectSet()

`intersectSet(...keys): Promise<Set<number>>`

<!-- source: storage/cache/set.ts:248 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L248)

Identical to [intersect](#intersect) except it returns the values as a `Set`.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<`Set`\<`number`\>\>

###### See

https://redis.io/commands/sinter/

###### Inherited from

`SetKeyspace.intersectSet`

##### intersectStore()

`intersectStore(destination, ...keys): Promise<number>`

<!-- source: storage/cache/set.ts:262 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L262)

Computes the set intersection between keys (like [intersect](#intersect)) and stores the result
in `destination`.

###### Parameters

###### destination

`K`

Key to store the result.

###### keys

...`K`[]

Keys of sets to compute intersection for.

###### Returns

`Promise`\<`number`\>

The size of the resulting set.

###### See

https://redis.io/commands/sinterstore/

###### Inherited from

`SetKeyspace.intersectStore`

##### items()

`items(key): Promise<number[]>`

<!-- source: storage/cache/set.ts:141 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L141)

Returns the elements in the set stored at key.

If the key does not exist it returns an empty array.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`number`[]\>

All members of the set.

###### See

https://redis.io/commands/smembers/

###### Inherited from

`SetKeyspace.items`

##### itemsSet()

`itemsSet(key): Promise<Set<number>>`

<!-- source: storage/cache/set.ts:156 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L156)

Identical to [items](#items-1) except it returns the values as a `Set`.

If the key does not exist it returns an empty `Set`.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`Set`\<`number`\>\>

All members of the set as a `Set`.

###### See

https://redis.io/commands/smembers/

###### Inherited from

`SetKeyspace.itemsSet`

##### len()

`len(key): Promise<number>`

<!-- source: storage/cache/set.ts:126 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L126)

Returns the number of elements in the set stored at key.

If the key does not exist it returns 0.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`number`\>

The set cardinality.

###### See

https://redis.io/commands/scard/

###### Inherited from

`SetKeyspace.len`

##### mapKey()

`protected mapKey(key): string`

<!-- source: storage/cache/keyspace.ts:91 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L91)

Maps a key to its Redis key string.

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`SetKeyspace.mapKey`

##### move()

```ts
move(
   src, 
   dst, 
   member, 
options?): Promise<boolean>;
```

<!-- source: storage/cache/set.ts:416 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L416)

Atomically moves the given member from the set stored at `src`
to the set stored at `dst`.

If the element already exists in `dst` it is still removed from `src`.

###### Parameters

###### src

`K`

Source set key.

###### dst

`K`

Destination set key.

###### member

`number`

The member to move.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`boolean`\>

`true` if the member was moved, `false` if not found in `src`.

###### See

https://redis.io/commands/smove/

###### Inherited from

`SetKeyspace.move`

##### pop()

```ts
pop(
   key, 
   count, 
options?): Promise<number[]>;
```

<!-- source: storage/cache/set.ts:90 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L90)

Removes up to `count` random elements (bounded by the set's size)
from the set stored at key and returns them.

If the set is empty it returns an empty array.

###### Parameters

###### key

`K`

The cache key.

###### count

`number`

Number of members to pop.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number`[]\>

The removed members (may be fewer than `count` if the set is small).

###### See

https://redis.io/commands/spop/

###### Inherited from

`SetKeyspace.pop`

##### popOne()

`popOne(key, options?): Promise<number | undefined>`

<!-- source: storage/cache/set.ts:68 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L68)

Removes a random element from the set stored at key and returns it.

###### Parameters

###### key

`K`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number` \| `undefined`\>

The removed member, or `undefined` if the set is empty.

###### See

https://redis.io/commands/spop/

###### Inherited from

`SetKeyspace.popOne`

##### remove()

`remove(key, ...members): Promise<number>`

<!-- source: storage/cache/set.ts:48 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L48)

Removes one or more values from the set stored at key.
Values not present in the set are ignored.
If the key does not already exist, it is a no-op.

###### Parameters

###### key

`K`

###### members

...`number`[]

###### Returns

`Promise`\<`number`\>

The number of values that were removed from the set.

###### See

https://redis.io/commands/srem/

###### Inherited from

`SetKeyspace.remove`

##### resolveTtl()

`protected resolveTtl(options?): number | undefined`

<!-- source: storage/cache/keyspace.ts:103 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L103)

Resolves the TTL for a write operation.
Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms

###### Parameters

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`number` \| `undefined`

###### Inherited from

`SetKeyspace.resolveTtl`

##### sample()

`sample(key, count): Promise<number[]>`

<!-- source: storage/cache/set.ts:364 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L364)

Returns up to `count` distinct random elements from the set stored at key.
The same element is never returned multiple times.

If the key does not exist it returns an empty array.

###### Parameters

###### key

`K`

The cache key.

###### count

`number`

Number of distinct members to return.

###### Returns

`Promise`\<`number`[]\>

Random members (may be fewer than `count` if the set is small).

###### See

https://redis.io/commands/srandmember/

###### Inherited from

`SetKeyspace.sample`

##### sampleOne()

`sampleOne(key): Promise<number | undefined>`

<!-- source: storage/cache/set.ts:343 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L343)

Returns a random member from the set stored at key without removing it.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`number` \| `undefined`\>

A random member, or `undefined` if the key does not exist.

###### See

https://redis.io/commands/srandmember/

###### Inherited from

`SetKeyspace.sampleOne`

##### sampleWithReplacement()

`sampleWithReplacement(key, count): Promise<number[]>`

<!-- source: storage/cache/set.ts:389 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L389)

Returns `count` random elements from the set stored at key.
The same element may be returned multiple times.

If the key does not exist it returns an empty array.

###### Parameters

###### key

`K`

The cache key.

###### count

`number`

Number of members to return (may include duplicates).

###### Returns

`Promise`\<`number`[]\>

Random members, possibly with duplicates.

###### See

https://redis.io/commands/srandmember/

###### Inherited from

`SetKeyspace.sampleWithReplacement`

##### serializeItem()

`protected serializeItem(value): Buffer`

<!-- source: storage/cache/set.ts:484 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L484)

###### Parameters

###### value

`number`

###### Returns

`Buffer`

###### Overrides

`SetKeyspace.serializeItem`

##### union()

`union(...keys): Promise<number[]>`

<!-- source: storage/cache/set.ts:291 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L291)

Computes the set union between the sets stored at the given keys.

Set union means the values present in at least one of the provided sets.

Keys that do not exist are considered to be empty sets.

###### Parameters

###### keys

...`K`[]

Keys of sets to compute union for. At least one must be provided.

###### Returns

`Promise`\<`number`[]\>

Members in any of the provided sets.

###### Throws

If no keys are provided.

###### See

https://redis.io/commands/sunion/

###### Inherited from

`SetKeyspace.union`

##### unionSet()

`unionSet(...keys): Promise<Set<number>>`

<!-- source: storage/cache/set.ts:306 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L306)

Identical to [union](#union) except it returns the values as a `Set`.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<`Set`\<`number`\>\>

###### See

https://redis.io/commands/sunion/

###### Inherited from

`SetKeyspace.unionSet`

##### unionStore()

`unionStore(destination, ...keys): Promise<number>`

<!-- source: storage/cache/set.ts:320 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L320)

Computes the set union between sets (like [union](#union)) and stores the result
in `destination`.

###### Parameters

###### destination

`K`

Key to store the result.

###### keys

...`K`[]

Keys of sets to compute union for.

###### Returns

`Promise`\<`number`\>

The size of the resulting set.

###### See

https://redis.io/commands/sunionstore/

###### Inherited from

`SetKeyspace.unionStore`

##### with()

`with(options): this`

<!-- source: storage/cache/keyspace.ts:123 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L123)

Returns a shallow clone of this keyspace with the specified write options applied.
This allows setting expiry for a chain of operations.

###### Parameters

###### options

[`WriteOptions`](#writeoptions)

###### Returns

`this`

###### Example

`await myKeyspace.with({ expiry: expireIn(5000) }).set(key, value)`

###### Inherited from

`SetKeyspace.with`

***

<!-- symbol-end -->

<!-- symbol-start: StringKeyspace -->
### StringKeyspace

<!-- source: storage/cache/basic.ts:193 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L193)

StringKeyspace stores string values.

#### Example

```ts
const tokens = new StringKeyspace<string>(cluster, {
  keyPattern: "token/:id",
  defaultExpiry: ExpireIn(3600000), // 1 hour
});

await tokens.set("abc123", "user-token-value");
const token = await tokens.get("abc123");
```

#### Extends

- `BasicKeyspace`\<`K`, `string`\>

#### Type Parameters

##### K

`K`

#### Constructors

##### Constructor

`new StringKeyspace<K>(cluster, config): StringKeyspace<K>`

<!-- source: storage/cache/basic.ts:194 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L194)

###### Parameters

###### cluster

[`CacheCluster`](#cachecluster)

###### config

[`KeyspaceConfig`](#keyspaceconfig)\<`K`\>

###### Returns

[`StringKeyspace`](#stringkeyspace)\<`K`\>

###### Overrides

`BasicKeyspace<K, string>.constructor`

#### Properties

##### cluster

`protected readonly cluster: CacheCluster`

<!-- source: storage/cache/keyspace.ts:46 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L46)

###### Inherited from

[`IntKeyspace`](#intkeyspace).[`cluster`](#cluster-1)

##### config

`protected readonly config: KeyspaceConfig<K>`

<!-- source: storage/cache/keyspace.ts:47 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L47)

###### Inherited from

`BasicKeyspace.config`

##### keyMapper

`protected readonly keyMapper: (key) => string`

<!-- source: storage/cache/keyspace.ts:48 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L48)

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`BasicKeyspace.keyMapper`

#### Methods

##### append()

```ts
append(
   key, 
   value, 
options?): Promise<number>;
```

<!-- source: storage/cache/basic.ts:215 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L215)

Appends a string to the value stored at key.

If the key does not exist it is first created and set as the empty string,
causing append to behave like set.

###### Parameters

###### key

`K`

###### value

`string`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number`\>

The new string length.

###### See

https://redis.io/commands/append/

##### delete()

`delete(...keys): Promise<number>`

<!-- source: storage/cache/keyspace.ts:137 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L137)

Deletes the specified keys.
If a key does not exist it is ignored.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<`number`\>

The number of keys that were deleted.

###### See

https://redis.io/commands/del/

###### Inherited from

`BasicKeyspace.delete`

##### deserialize()

`protected deserialize(data): string`

<!-- source: storage/cache/basic.ts:202 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L202)

Deserializes a Buffer from storage to a value.

###### Parameters

###### data

`Buffer`

###### Returns

`string`

###### Overrides

`BasicKeyspace.deserialize`

##### get()

`get(key): Promise<string | undefined>`

<!-- source: storage/cache/basic.ts:33 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L33)

Gets the value stored at key.
If the key does not exist, it returns `undefined`.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`string` \| `undefined`\>

The value, or `undefined` if the key does not exist.

###### See

https://redis.io/commands/get/

###### Inherited from

`BasicKeyspace.get`

##### getAndDelete()

`getAndDelete(key): Promise<string | undefined>`

<!-- source: storage/cache/basic.ts:165 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L165)

Deletes the key and returns the previously stored value.
If the key does not already exist, it returns `undefined`.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`string` \| `undefined`\>

The previous value, or `undefined` if the key did not exist.

###### See

https://redis.io/commands/getdel/

###### Inherited from

`BasicKeyspace.getAndDelete`

##### getAndSet()

```ts
getAndSet(
   key, 
   value, 
options?): Promise<string | undefined>;
```

<!-- source: storage/cache/basic.ts:134 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L134)

Updates the value of key to val and returns the previously stored value.
If the key does not already exist, it sets it and returns `undefined`.

###### Parameters

###### key

`K`

###### value

`string`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`string` \| `undefined`\>

The previous value, or `undefined` if the key did not exist.

###### See

https://redis.io/commands/getset/

###### Inherited from

`BasicKeyspace.getAndSet`

##### getRange()

```ts
getRange(
   key, 
   start, 
end): Promise<string>;
```

<!-- source: storage/cache/basic.ts:245 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L245)

Returns a substring of the string value stored at key.

The `start` and `end` values are zero-based indices, but unlike typical slicing
the `end` value is inclusive.

Negative values can be used in order to provide an offset starting
from the end of the string, so -1 means the last character.

If the string does not exist it returns the empty string.

###### Parameters

###### key

`K`

The cache key.

###### start

`number`

Start index (inclusive, 0-based).

###### end

`number`

End index (inclusive, 0-based). Use -1 for end of string.

###### Returns

`Promise`\<`string`\>

The substring.

###### See

https://redis.io/commands/getrange/

##### len()

`len(key): Promise<number>`

<!-- source: storage/cache/basic.ts:300 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L300)

Returns the length of the string value stored at key.

Non-existing keys are considered as empty strings.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`number`\>

The string length.

###### See

https://redis.io/commands/strlen/

##### mapKey()

`protected mapKey(key): string`

<!-- source: storage/cache/keyspace.ts:91 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L91)

Maps a key to its Redis key string.

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`BasicKeyspace.mapKey`

##### multiGet()

`multiGet(...keys): Promise<(string | undefined)[]>`

<!-- source: storage/cache/basic.ts:52 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L52)

Gets the values stored at multiple keys.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<(`string` \| `undefined`)[]\>

An array of values in the same order as the provided keys.
Each element is the value or `undefined` if the key was not found.

###### See

https://redis.io/commands/mget/

###### Inherited from

`BasicKeyspace.multiGet`

##### replace()

```ts
replace(
   key, 
   value, 
options?): Promise<void>;
```

<!-- source: storage/cache/basic.ts:109 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L109)

Replaces the existing value stored at key to val.

###### Parameters

###### key

`K`

###### value

`string`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`void`\>

###### Throws

If the key does not already exist.

###### See

https://redis.io/commands/set/

###### Inherited from

`BasicKeyspace.replace`

##### resolveTtl()

`protected resolveTtl(options?): number | undefined`

<!-- source: storage/cache/keyspace.ts:103 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L103)

Resolves the TTL for a write operation.
Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms

###### Parameters

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`number` \| `undefined`

###### Inherited from

`BasicKeyspace.resolveTtl`

##### serialize()

`protected serialize(value): Buffer`

<!-- source: storage/cache/basic.ts:198 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L198)

Serializes a value to a Buffer for storage.

###### Parameters

###### value

`string`

###### Returns

`Buffer`

###### Overrides

`BasicKeyspace.serialize`

##### set()

```ts
set(
   key, 
   value, 
options?): Promise<void>;
```

<!-- source: storage/cache/basic.ts:66 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L66)

Updates the value stored at key to val.

###### Parameters

###### key

`K`

###### value

`string`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`void`\>

###### See

https://redis.io/commands/set/

###### Inherited from

`BasicKeyspace.set`

##### setIfNotExists()

```ts
setIfNotExists(
   key, 
   value, 
options?): Promise<void>;
```

<!-- source: storage/cache/basic.ts:81 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L81)

Sets the value stored at key to val, but only if the key does not exist beforehand.

###### Parameters

###### key

`K`

###### value

`string`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`void`\>

###### Throws

If the key already exists.

###### See

https://redis.io/commands/setnx/

###### Inherited from

`BasicKeyspace.setIfNotExists`

##### setRange()

```ts
setRange(
   key, 
   offset, 
   value, 
options?): Promise<number>;
```

<!-- source: storage/cache/basic.ts:273 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L273)

Overwrites part of the string stored at key, starting at
the zero-based `offset` and for the entire length of `value`, extending
the string if necessary.

If the offset is larger than the current string length stored at key,
the string is first padded with zero-bytes to make offset fit.

Non-existing keys are considered as empty strings.

###### Parameters

###### key

`K`

The cache key.

###### offset

`number`

Zero-based byte offset to start writing at.

###### value

`string`

The string to write.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number`\>

The length of the string after the operation.

###### See

https://redis.io/commands/setrange/

##### with()

`with(options): this`

<!-- source: storage/cache/keyspace.ts:123 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L123)

Returns a shallow clone of this keyspace with the specified write options applied.
This allows setting expiry for a chain of operations.

###### Parameters

###### options

[`WriteOptions`](#writeoptions)

###### Returns

`this`

###### Example

`await myKeyspace.with({ expiry: expireIn(5000) }).set(key, value)`

###### Inherited from

`BasicKeyspace.with`

***

<!-- symbol-end -->

<!-- symbol-start: StringListKeyspace -->
### StringListKeyspace

<!-- source: storage/cache/list.ts:457 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L457)

StringListKeyspace stores lists of string values.

#### Example

```ts
const recentViews = new StringListKeyspace<string>(cluster, {
  keyPattern: "recent-views/:userId",
  defaultExpiry: ExpireIn(86400000), // 24 hours
});

await recentViews.pushLeft("user1", "product-123", "product-456");
const views = await recentViews.items("user1");
```

#### Extends

- `ListKeyspace`\<`K`, `string`\>

#### Type Parameters

##### K

`K`

#### Constructors

##### Constructor

`new StringListKeyspace<K>(cluster, config): StringListKeyspace<K>`

<!-- source: storage/cache/list.ts:458 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L458)

###### Parameters

###### cluster

[`CacheCluster`](#cachecluster)

###### config

[`KeyspaceConfig`](#keyspaceconfig)\<`K`\>

###### Returns

[`StringListKeyspace`](#stringlistkeyspace)\<`K`\>

###### Overrides

`ListKeyspace<K, string>.constructor`

#### Properties

##### cluster

`protected readonly cluster: CacheCluster`

<!-- source: storage/cache/keyspace.ts:46 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L46)

###### Inherited from

`ListKeyspace.cluster`

##### config

`protected readonly config: KeyspaceConfig<K>`

<!-- source: storage/cache/keyspace.ts:47 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L47)

###### Inherited from

`ListKeyspace.config`

##### keyMapper

`protected readonly keyMapper: (key) => string`

<!-- source: storage/cache/keyspace.ts:48 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L48)

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`ListKeyspace.keyMapper`

#### Methods

##### delete()

`delete(...keys): Promise<number>`

<!-- source: storage/cache/keyspace.ts:137 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L137)

Deletes the specified keys.
If a key does not exist it is ignored.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<`number`\>

The number of keys that were deleted.

###### See

https://redis.io/commands/del/

###### Inherited from

`ListKeyspace.delete`

##### deserializeItem()

`protected deserializeItem(data): string`

<!-- source: storage/cache/list.ts:466 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L466)

###### Parameters

###### data

`Buffer`

###### Returns

`string`

###### Overrides

`ListKeyspace.deserializeItem`

##### get()

`get(key, index): Promise<string | undefined>`

<!-- source: storage/cache/list.ts:187 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L187)

Returns the value of the list element at the given index.

Negative indices can be used to indicate offsets from the end of the list,
where -1 is the last element of the list, -2 the penultimate element, and so on.

###### Parameters

###### key

`K`

The cache key.

###### index

`number`

Zero-based index of the element to retrieve.

###### Returns

`Promise`\<`string` \| `undefined`\>

The value at the index, or `undefined` if out of range or the key does not exist.

###### See

https://redis.io/commands/lindex/

###### Inherited from

`ListKeyspace.get`

##### getRange()

```ts
getRange(
   key, 
   start, 
stop): Promise<string[]>;
```

<!-- source: storage/cache/list.ts:229 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L229)

Returns the elements in the list stored at key between `start` and `stop` (inclusive).
Both are zero-based indices.

Negative indices can be used to indicate offsets from the end of the list,
where -1 is the last element of the list, -2 the penultimate element, and so on.

If the key does not exist it returns an empty array.

###### Parameters

###### key

`K`

The cache key.

###### start

`number`

Start index (inclusive).

###### stop

`number`

Stop index (inclusive).

###### Returns

`Promise`\<`string`[]\>

The elements in the specified range.

###### See

https://redis.io/commands/lrange/

###### Inherited from

`ListKeyspace.getRange`

##### insertAfter()

```ts
insertAfter(
   key, 
   pivot, 
   value, 
options?): Promise<number>;
```

<!-- source: storage/cache/list.ts:284 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L284)

Inserts `value` into the list stored at key, at the position just after `pivot`.

If the list does not contain `pivot`, the value is not inserted and -1 is returned.

###### Parameters

###### key

`K`

The cache key.

###### pivot

`string`

The existing element to insert after.

###### value

`string`

The value to insert.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number`\>

The new list length, or -1 if `pivot` was not found.

###### See

https://redis.io/commands/linsert/

###### Inherited from

`ListKeyspace.insertAfter`

##### insertBefore()

```ts
insertBefore(
   key, 
   pivot, 
   value, 
options?): Promise<number>;
```

<!-- source: storage/cache/list.ts:252 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L252)

Inserts `value` into the list stored at key, at the position just before `pivot`.

If the list does not contain `pivot`, the value is not inserted and -1 is returned.

###### Parameters

###### key

`K`

The cache key.

###### pivot

`string`

The existing element to insert before.

###### value

`string`

The value to insert.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number`\>

The new list length, or -1 if `pivot` was not found.

###### See

https://redis.io/commands/linsert/

###### Inherited from

`ListKeyspace.insertBefore`

##### items()

`items(key): Promise<string[]>`

<!-- source: storage/cache/list.ts:207 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L207)

Returns all the elements in the list stored at key.

If the key does not exist it returns an empty array.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`string`[]\>

All elements in the list.

###### See

https://redis.io/commands/lrange/

###### Inherited from

`ListKeyspace.items`

##### len()

`len(key): Promise<number>`

<!-- source: storage/cache/list.ts:117 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L117)

Returns the length of the list stored at key.

Non-existing keys are considered as empty lists.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`number`\>

The list length.

###### See

https://redis.io/commands/llen/

###### Inherited from

`ListKeyspace.len`

##### mapKey()

`protected mapKey(key): string`

<!-- source: storage/cache/keyspace.ts:91 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L91)

Maps a key to its Redis key string.

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`ListKeyspace.mapKey`

##### move()

```ts
move(
   src, 
   dst, 
   srcPos, 
   dstPos, 
options?): Promise<string | undefined>;
```

<!-- source: storage/cache/list.ts:417 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L417)

Atomically moves an element from the list stored at `src` to the list stored at `dst`.

The value moved can be either the head (`srcPos === "left"`) or tail (`srcPos === "right"`)
of the list at `src`. Similarly, the value can be placed either at the head (`dstPos === "left"`)
or tail (`dstPos === "right"`) of the list at `dst`.

If `src` and `dst` are the same list, the value is atomically rotated from one end to the other
when `srcPos !== dstPos`, or if `srcPos === dstPos` nothing happens.

###### Parameters

###### src

`K`

Source list key.

###### dst

`K`

Destination list key.

###### srcPos

[`ListPosition`](#listposition)

Position to pop from in the source list.

###### dstPos

[`ListPosition`](#listposition)

Position to push to in the destination list.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`string` \| `undefined`\>

The moved element, or `undefined` if the source list does not exist.

###### See

https://redis.io/commands/lmove/

###### Inherited from

`ListKeyspace.move`

##### popLeft()

`popLeft(key, options?): Promise<string | undefined>`

<!-- source: storage/cache/list.ts:81 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L81)

Pops a single element off the head of the list stored at key.

###### Parameters

###### key

`K`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`string` \| `undefined`\>

The popped value, or `undefined` if the key does not exist.

###### See

https://redis.io/commands/lpop/

###### Inherited from

`ListKeyspace.popLeft`

##### popRight()

`popRight(key, options?): Promise<string | undefined>`

<!-- source: storage/cache/list.ts:98 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L98)

Pops a single element off the tail of the list stored at key.

###### Parameters

###### key

`K`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`string` \| `undefined`\>

The popped value, or `undefined` if the key does not exist.

###### See

https://redis.io/commands/rpop/

###### Inherited from

`ListKeyspace.popRight`

##### pushLeft()

`pushLeft(key, ...values): Promise<number>`

<!-- source: storage/cache/list.ts:35 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L35)

Pushes one or more values at the head of the list stored at key.
If the key does not already exist, it is first created as an empty list.

If multiple values are given, they are inserted one after another,
starting with the leftmost value. For instance,
`pushLeft(key, "a", "b", "c")` will result in a list containing
"c" as its first element, "b" as its second, and "a" as its third.

###### Parameters

###### key

`K`

###### values

...`string`[]

###### Returns

`Promise`\<`number`\>

The length of the list after the operation.

###### See

https://redis.io/commands/lpush/

###### Inherited from

`ListKeyspace.pushLeft`

##### pushRight()

`pushRight(key, ...values): Promise<number>`

<!-- source: storage/cache/list.ts:61 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L61)

Pushes one or more values at the tail of the list stored at key.
If the key does not already exist, it is first created as an empty list.

If multiple values are given, they are inserted one after another,
starting with the leftmost value. For instance,
`pushRight(key, "a", "b", "c")` will result in a list containing
"a" as its first element, "b" as its second, and "c" as its third.

###### Parameters

###### key

`K`

###### values

...`string`[]

###### Returns

`Promise`\<`number`\>

The length of the list after the operation.

###### See

https://redis.io/commands/rpush/

###### Inherited from

`ListKeyspace.pushRight`

##### removeAll()

```ts
removeAll(
   key, 
   value, 
options?): Promise<number>;
```

<!-- source: storage/cache/list.ts:315 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L315)

Removes all occurrences of `value` in the list stored at key.

If the list does not contain `value`, or the list does not exist, returns 0.

###### Parameters

###### key

`K`

The cache key.

###### value

`string`

The value to remove.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number`\>

The number of elements removed.

###### See

https://redis.io/commands/lrem/

###### Inherited from

`ListKeyspace.removeAll`

##### removeFirst()

```ts
removeFirst(
   key, 
   count, 
   value, 
options?): Promise<number>;
```

<!-- source: storage/cache/list.ts:341 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L341)

Removes the first `count` occurrences of `value` in the list stored at key,
scanning from head to tail.

If the list does not contain `value`, or the list does not exist, returns 0.

###### Parameters

###### key

`K`

The cache key.

###### count

`number`

Maximum number of occurrences to remove.

###### value

`string`

The value to remove.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number`\>

The number of elements removed.

###### See

https://redis.io/commands/lrem/

###### Inherited from

`ListKeyspace.removeFirst`

##### removeLast()

```ts
removeLast(
   key, 
   count, 
   value, 
options?): Promise<number>;
```

<!-- source: storage/cache/list.ts:376 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L376)

Removes the last `count` occurrences of `value` in the list stored at key,
scanning from tail to head.

If the list does not contain `value`, or the list does not exist, returns 0.

###### Parameters

###### key

`K`

The cache key.

###### count

`number`

Maximum number of occurrences to remove.

###### value

`string`

The value to remove.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`number`\>

The number of elements removed.

###### See

https://redis.io/commands/lrem/

###### Inherited from

`ListKeyspace.removeLast`

##### resolveTtl()

`protected resolveTtl(options?): number | undefined`

<!-- source: storage/cache/keyspace.ts:103 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L103)

Resolves the TTL for a write operation.
Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms

###### Parameters

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`number` \| `undefined`

###### Inherited from

`ListKeyspace.resolveTtl`

##### serializeItem()

`protected serializeItem(value): Buffer`

<!-- source: storage/cache/list.ts:462 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L462)

###### Parameters

###### value

`string`

###### Returns

`Buffer`

###### Overrides

`ListKeyspace.serializeItem`

##### set()

```ts
set(
   key, 
   index, 
   value, 
options?): Promise<void>;
```

<!-- source: storage/cache/list.ts:163 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L163)

Updates the list element at the given index.

Negative indices can be used to indicate offsets from the end of the list,
where -1 is the last element of the list, -2 the penultimate element, and so on.

###### Parameters

###### key

`K`

The cache key.

###### index

`number`

Zero-based index of the element to update.

###### value

`string`

The new value.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`void`\>

###### Throws

If the index is out of range.

###### See

https://redis.io/commands/lset/

###### Inherited from

`ListKeyspace.set`

##### trim()

```ts
trim(
   key, 
   start, 
   stop, 
options?): Promise<void>;
```

<!-- source: storage/cache/list.ts:139 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L139)

Trims the list stored at key to only contain the elements between the indices
`start` and `stop` (inclusive). Both are zero-based indices.

Negative indices can be used to indicate offsets from the end of the list,
where -1 is the last element of the list, -2 the penultimate element, and so on.

Out of range indices are valid and are treated as if they specify the start or end of the list,
respectively. If `start` > `stop` the end result is an empty list.

###### Parameters

###### key

`K`

The cache key.

###### start

`number`

Start index (inclusive).

###### stop

`number`

Stop index (inclusive).

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`void`\>

###### See

https://redis.io/commands/ltrim/

###### Inherited from

`ListKeyspace.trim`

##### with()

`with(options): this`

<!-- source: storage/cache/keyspace.ts:123 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L123)

Returns a shallow clone of this keyspace with the specified write options applied.
This allows setting expiry for a chain of operations.

###### Parameters

###### options

[`WriteOptions`](#writeoptions)

###### Returns

`this`

###### Example

`await myKeyspace.with({ expiry: expireIn(5000) }).set(key, value)`

###### Inherited from

`ListKeyspace.with`

***

<!-- symbol-end -->

<!-- symbol-start: StringSetKeyspace -->
### StringSetKeyspace

<!-- source: storage/cache/set.ts:452 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L452)

StringSetKeyspace stores sets of unique string values.

#### Example

```ts
const tags = new StringSetKeyspace<string>(cluster, {
  keyPattern: "tags/:articleId",
});

await tags.add("article1", "typescript", "programming", "web");
const hasTech = await tags.contains("article1", "typescript");
const allTags = await tags.items("article1");
const tagSet = await tags.itemsSet("article1");
```

#### Extends

- `SetKeyspace`\<`K`, `string`\>

#### Type Parameters

##### K

`K`

#### Constructors

##### Constructor

`new StringSetKeyspace<K>(cluster, config): StringSetKeyspace<K>`

<!-- source: storage/cache/set.ts:453 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L453)

###### Parameters

###### cluster

[`CacheCluster`](#cachecluster)

###### config

[`KeyspaceConfig`](#keyspaceconfig)\<`K`\>

###### Returns

[`StringSetKeyspace`](#stringsetkeyspace)\<`K`\>

###### Overrides

`SetKeyspace<K, string>.constructor`

#### Properties

##### cluster

`protected readonly cluster: CacheCluster`

<!-- source: storage/cache/keyspace.ts:46 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L46)

###### Inherited from

`SetKeyspace.cluster`

##### config

`protected readonly config: KeyspaceConfig<K>`

<!-- source: storage/cache/keyspace.ts:47 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L47)

###### Inherited from

`SetKeyspace.config`

##### keyMapper

`protected readonly keyMapper: (key) => string`

<!-- source: storage/cache/keyspace.ts:48 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L48)

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`SetKeyspace.keyMapper`

#### Methods

##### add()

`add(key, ...members): Promise<number>`

<!-- source: storage/cache/set.ts:26 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L26)

Adds one or more values to the set stored at key.
If the key does not already exist, it is first created as an empty set.

###### Parameters

###### key

`K`

###### members

...`string`[]

###### Returns

`Promise`\<`number`\>

The number of values that were added to the set,
not including values already present beforehand.

###### See

https://redis.io/commands/sadd/

###### Inherited from

`SetKeyspace.add`

##### contains()

`contains(key, member): Promise<boolean>`

<!-- source: storage/cache/set.ts:111 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L111)

Reports whether the set stored at key contains the given value.

If the key does not exist it returns `false`.

###### Parameters

###### key

`K`

###### member

`string`

###### Returns

`Promise`\<`boolean`\>

`true` if the member exists in the set, `false` otherwise.

###### See

https://redis.io/commands/sismember/

###### Inherited from

`SetKeyspace.contains`

##### delete()

`delete(...keys): Promise<number>`

<!-- source: storage/cache/keyspace.ts:137 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L137)

Deletes the specified keys.
If a key does not exist it is ignored.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<`number`\>

The number of keys that were deleted.

###### See

https://redis.io/commands/del/

###### Inherited from

`SetKeyspace.delete`

##### deserializeItem()

`protected deserializeItem(data): string`

<!-- source: storage/cache/set.ts:461 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L461)

###### Parameters

###### data

`Buffer`

###### Returns

`string`

###### Overrides

`SetKeyspace.deserializeItem`

##### diff()

`diff(...keys): Promise<string[]>`

<!-- source: storage/cache/set.ts:174 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L174)

Computes the set difference between the first set and all the consecutive sets.

Set difference means the values present in the first set that are not present
in any of the other sets.

Keys that do not exist are considered as empty sets.

###### Parameters

###### keys

...`K`[]

Keys of sets to compute difference for. At least one must be provided.

###### Returns

`Promise`\<`string`[]\>

Members in the first set but not in any of the other sets.

###### Throws

If no keys are provided.

###### See

https://redis.io/commands/sdiff/

###### Inherited from

`SetKeyspace.diff`

##### diffSet()

`diffSet(...keys): Promise<Set<string>>`

<!-- source: storage/cache/set.ts:189 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L189)

Identical to [diff](#diff-1) except it returns the values as a `Set`.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<`Set`\<`string`\>\>

###### See

https://redis.io/commands/sdiff/

###### Inherited from

`SetKeyspace.diffSet`

##### diffStore()

`diffStore(destination, ...keys): Promise<number>`

<!-- source: storage/cache/set.ts:203 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L203)

Computes the set difference between keys (like [diff](#diff-1)) and stores the result
in `destination`.

###### Parameters

###### destination

`K`

Key to store the result.

###### keys

...`K`[]

Keys of sets to compute difference for.

###### Returns

`Promise`\<`number`\>

The size of the resulting set.

###### See

https://redis.io/commands/sdiffstore/

###### Inherited from

`SetKeyspace.diffStore`

##### intersect()

`intersect(...keys): Promise<string[]>`

<!-- source: storage/cache/set.ts:233 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L233)

Computes the set intersection between the sets stored at the given keys.

Set intersection means the values common to all the provided sets.

Keys that do not exist are considered to be empty sets.
As a result, if any key is missing the final result is the empty set.

###### Parameters

###### keys

...`K`[]

Keys of sets to compute intersection for. At least one must be provided.

###### Returns

`Promise`\<`string`[]\>

Members common to all sets.

###### Throws

If no keys are provided.

###### See

https://redis.io/commands/sinter/

###### Inherited from

`SetKeyspace.intersect`

##### intersectSet()

`intersectSet(...keys): Promise<Set<string>>`

<!-- source: storage/cache/set.ts:248 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L248)

Identical to [intersect](#intersect-1) except it returns the values as a `Set`.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<`Set`\<`string`\>\>

###### See

https://redis.io/commands/sinter/

###### Inherited from

`SetKeyspace.intersectSet`

##### intersectStore()

`intersectStore(destination, ...keys): Promise<number>`

<!-- source: storage/cache/set.ts:262 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L262)

Computes the set intersection between keys (like [intersect](#intersect-1)) and stores the result
in `destination`.

###### Parameters

###### destination

`K`

Key to store the result.

###### keys

...`K`[]

Keys of sets to compute intersection for.

###### Returns

`Promise`\<`number`\>

The size of the resulting set.

###### See

https://redis.io/commands/sinterstore/

###### Inherited from

`SetKeyspace.intersectStore`

##### items()

`items(key): Promise<string[]>`

<!-- source: storage/cache/set.ts:141 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L141)

Returns the elements in the set stored at key.

If the key does not exist it returns an empty array.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`string`[]\>

All members of the set.

###### See

https://redis.io/commands/smembers/

###### Inherited from

`SetKeyspace.items`

##### itemsSet()

`itemsSet(key): Promise<Set<string>>`

<!-- source: storage/cache/set.ts:156 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L156)

Identical to [items](#items-3) except it returns the values as a `Set`.

If the key does not exist it returns an empty `Set`.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`Set`\<`string`\>\>

All members of the set as a `Set`.

###### See

https://redis.io/commands/smembers/

###### Inherited from

`SetKeyspace.itemsSet`

##### len()

`len(key): Promise<number>`

<!-- source: storage/cache/set.ts:126 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L126)

Returns the number of elements in the set stored at key.

If the key does not exist it returns 0.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`number`\>

The set cardinality.

###### See

https://redis.io/commands/scard/

###### Inherited from

`SetKeyspace.len`

##### mapKey()

`protected mapKey(key): string`

<!-- source: storage/cache/keyspace.ts:91 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L91)

Maps a key to its Redis key string.

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`SetKeyspace.mapKey`

##### move()

```ts
move(
   src, 
   dst, 
   member, 
options?): Promise<boolean>;
```

<!-- source: storage/cache/set.ts:416 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L416)

Atomically moves the given member from the set stored at `src`
to the set stored at `dst`.

If the element already exists in `dst` it is still removed from `src`.

###### Parameters

###### src

`K`

Source set key.

###### dst

`K`

Destination set key.

###### member

`string`

The member to move.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`boolean`\>

`true` if the member was moved, `false` if not found in `src`.

###### See

https://redis.io/commands/smove/

###### Inherited from

`SetKeyspace.move`

##### pop()

```ts
pop(
   key, 
   count, 
options?): Promise<string[]>;
```

<!-- source: storage/cache/set.ts:90 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L90)

Removes up to `count` random elements (bounded by the set's size)
from the set stored at key and returns them.

If the set is empty it returns an empty array.

###### Parameters

###### key

`K`

The cache key.

###### count

`number`

Number of members to pop.

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`string`[]\>

The removed members (may be fewer than `count` if the set is small).

###### See

https://redis.io/commands/spop/

###### Inherited from

`SetKeyspace.pop`

##### popOne()

`popOne(key, options?): Promise<string | undefined>`

<!-- source: storage/cache/set.ts:68 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L68)

Removes a random element from the set stored at key and returns it.

###### Parameters

###### key

`K`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`string` \| `undefined`\>

The removed member, or `undefined` if the set is empty.

###### See

https://redis.io/commands/spop/

###### Inherited from

`SetKeyspace.popOne`

##### remove()

`remove(key, ...members): Promise<number>`

<!-- source: storage/cache/set.ts:48 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L48)

Removes one or more values from the set stored at key.
Values not present in the set are ignored.
If the key does not already exist, it is a no-op.

###### Parameters

###### key

`K`

###### members

...`string`[]

###### Returns

`Promise`\<`number`\>

The number of values that were removed from the set.

###### See

https://redis.io/commands/srem/

###### Inherited from

`SetKeyspace.remove`

##### resolveTtl()

`protected resolveTtl(options?): number | undefined`

<!-- source: storage/cache/keyspace.ts:103 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L103)

Resolves the TTL for a write operation.
Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms

###### Parameters

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`number` \| `undefined`

###### Inherited from

`SetKeyspace.resolveTtl`

##### sample()

`sample(key, count): Promise<string[]>`

<!-- source: storage/cache/set.ts:364 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L364)

Returns up to `count` distinct random elements from the set stored at key.
The same element is never returned multiple times.

If the key does not exist it returns an empty array.

###### Parameters

###### key

`K`

The cache key.

###### count

`number`

Number of distinct members to return.

###### Returns

`Promise`\<`string`[]\>

Random members (may be fewer than `count` if the set is small).

###### See

https://redis.io/commands/srandmember/

###### Inherited from

`SetKeyspace.sample`

##### sampleOne()

`sampleOne(key): Promise<string | undefined>`

<!-- source: storage/cache/set.ts:343 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L343)

Returns a random member from the set stored at key without removing it.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`string` \| `undefined`\>

A random member, or `undefined` if the key does not exist.

###### See

https://redis.io/commands/srandmember/

###### Inherited from

`SetKeyspace.sampleOne`

##### sampleWithReplacement()

`sampleWithReplacement(key, count): Promise<string[]>`

<!-- source: storage/cache/set.ts:389 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L389)

Returns `count` random elements from the set stored at key.
The same element may be returned multiple times.

If the key does not exist it returns an empty array.

###### Parameters

###### key

`K`

The cache key.

###### count

`number`

Number of members to return (may include duplicates).

###### Returns

`Promise`\<`string`[]\>

Random members, possibly with duplicates.

###### See

https://redis.io/commands/srandmember/

###### Inherited from

`SetKeyspace.sampleWithReplacement`

##### serializeItem()

`protected serializeItem(value): Buffer`

<!-- source: storage/cache/set.ts:457 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L457)

###### Parameters

###### value

`string`

###### Returns

`Buffer`

###### Overrides

`SetKeyspace.serializeItem`

##### union()

`union(...keys): Promise<string[]>`

<!-- source: storage/cache/set.ts:291 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L291)

Computes the set union between the sets stored at the given keys.

Set union means the values present in at least one of the provided sets.

Keys that do not exist are considered to be empty sets.

###### Parameters

###### keys

...`K`[]

Keys of sets to compute union for. At least one must be provided.

###### Returns

`Promise`\<`string`[]\>

Members in any of the provided sets.

###### Throws

If no keys are provided.

###### See

https://redis.io/commands/sunion/

###### Inherited from

`SetKeyspace.union`

##### unionSet()

`unionSet(...keys): Promise<Set<string>>`

<!-- source: storage/cache/set.ts:306 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L306)

Identical to [union](#union-1) except it returns the values as a `Set`.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<`Set`\<`string`\>\>

###### See

https://redis.io/commands/sunion/

###### Inherited from

`SetKeyspace.unionSet`

##### unionStore()

`unionStore(destination, ...keys): Promise<number>`

<!-- source: storage/cache/set.ts:320 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L320)

Computes the set union between sets (like [union](#union-1)) and stores the result
in `destination`.

###### Parameters

###### destination

`K`

Key to store the result.

###### keys

...`K`[]

Keys of sets to compute union for.

###### Returns

`Promise`\<`number`\>

The size of the resulting set.

###### See

https://redis.io/commands/sunionstore/

###### Inherited from

`SetKeyspace.unionStore`

##### with()

`with(options): this`

<!-- source: storage/cache/keyspace.ts:123 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L123)

Returns a shallow clone of this keyspace with the specified write options applied.
This allows setting expiry for a chain of operations.

###### Parameters

###### options

[`WriteOptions`](#writeoptions)

###### Returns

`this`

###### Example

`await myKeyspace.with({ expiry: expireIn(5000) }).set(key, value)`

###### Inherited from

`SetKeyspace.with`

***

<!-- symbol-end -->

<!-- symbol-start: StructKeyspace -->
### StructKeyspace

<!-- source: storage/cache/basic.ts:499 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L499)

StructKeyspace stores arbitrary objects serialized as JSON.

#### Example

```ts
interface User {
  id: string;
  name: string;
  email: string;
}

const users = new StructKeyspace<string, User>(cluster, {
  keyPattern: "user/:id",
  defaultExpiry: ExpireIn(3600000),
});

await users.set("user1", { id: "user1", name: "Alice", email: "alice@example.com" });
const user = await users.get("user1");
```

#### Extends

- `BasicKeyspace`\<`K`, `V`\>

#### Type Parameters

##### K

`K`

##### V

`V`

#### Constructors

##### Constructor

`new StructKeyspace<K, V>(cluster, config): StructKeyspace<K, V>`

<!-- source: storage/cache/basic.ts:500 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L500)

###### Parameters

###### cluster

[`CacheCluster`](#cachecluster)

###### config

[`KeyspaceConfig`](#keyspaceconfig)\<`K`\>

###### Returns

[`StructKeyspace`](#structkeyspace)\<`K`, `V`\>

###### Overrides

`BasicKeyspace<K, V>.constructor`

#### Properties

##### cluster

`protected readonly cluster: CacheCluster`

<!-- source: storage/cache/keyspace.ts:46 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L46)

###### Inherited from

`BasicKeyspace.cluster`

##### config

`protected readonly config: KeyspaceConfig<K>`

<!-- source: storage/cache/keyspace.ts:47 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L47)

###### Inherited from

`BasicKeyspace.config`

##### keyMapper

`protected readonly keyMapper: (key) => string`

<!-- source: storage/cache/keyspace.ts:48 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L48)

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`BasicKeyspace.keyMapper`

#### Methods

##### delete()

`delete(...keys): Promise<number>`

<!-- source: storage/cache/keyspace.ts:137 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L137)

Deletes the specified keys.
If a key does not exist it is ignored.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<`number`\>

The number of keys that were deleted.

###### See

https://redis.io/commands/del/

###### Inherited from

`BasicKeyspace.delete`

##### deserialize()

`protected deserialize(data): V`

<!-- source: storage/cache/basic.ts:508 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L508)

Deserializes a Buffer from storage to a value.

###### Parameters

###### data

`Buffer`

###### Returns

`V`

###### Overrides

`BasicKeyspace.deserialize`

##### get()

`get(key): Promise<V | undefined>`

<!-- source: storage/cache/basic.ts:33 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L33)

Gets the value stored at key.
If the key does not exist, it returns `undefined`.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`V` \| `undefined`\>

The value, or `undefined` if the key does not exist.

###### See

https://redis.io/commands/get/

###### Inherited from

`BasicKeyspace.get`

##### getAndDelete()

`getAndDelete(key): Promise<V | undefined>`

<!-- source: storage/cache/basic.ts:165 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L165)

Deletes the key and returns the previously stored value.
If the key does not already exist, it returns `undefined`.

###### Parameters

###### key

`K`

###### Returns

`Promise`\<`V` \| `undefined`\>

The previous value, or `undefined` if the key did not exist.

###### See

https://redis.io/commands/getdel/

###### Inherited from

`BasicKeyspace.getAndDelete`

##### getAndSet()

```ts
getAndSet(
   key, 
   value, 
options?): Promise<V | undefined>;
```

<!-- source: storage/cache/basic.ts:134 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L134)

Updates the value of key to val and returns the previously stored value.
If the key does not already exist, it sets it and returns `undefined`.

###### Parameters

###### key

`K`

###### value

`V`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`V` \| `undefined`\>

The previous value, or `undefined` if the key did not exist.

###### See

https://redis.io/commands/getset/

###### Inherited from

`BasicKeyspace.getAndSet`

##### mapKey()

`protected mapKey(key): string`

<!-- source: storage/cache/keyspace.ts:91 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L91)

Maps a key to its Redis key string.

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`BasicKeyspace.mapKey`

##### multiGet()

`multiGet(...keys): Promise<(V | undefined)[]>`

<!-- source: storage/cache/basic.ts:52 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L52)

Gets the values stored at multiple keys.

###### Parameters

###### keys

...`K`[]

###### Returns

`Promise`\<(`V` \| `undefined`)[]\>

An array of values in the same order as the provided keys.
Each element is the value or `undefined` if the key was not found.

###### See

https://redis.io/commands/mget/

###### Inherited from

`BasicKeyspace.multiGet`

##### replace()

```ts
replace(
   key, 
   value, 
options?): Promise<void>;
```

<!-- source: storage/cache/basic.ts:109 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L109)

Replaces the existing value stored at key to val.

###### Parameters

###### key

`K`

###### value

`V`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`void`\>

###### Throws

If the key does not already exist.

###### See

https://redis.io/commands/set/

###### Inherited from

`BasicKeyspace.replace`

##### resolveTtl()

`protected resolveTtl(options?): number | undefined`

<!-- source: storage/cache/keyspace.ts:103 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L103)

Resolves the TTL for a write operation.
Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms

###### Parameters

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`number` \| `undefined`

###### Inherited from

`BasicKeyspace.resolveTtl`

##### serialize()

`protected serialize(value): Buffer`

<!-- source: storage/cache/basic.ts:504 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L504)

Serializes a value to a Buffer for storage.

###### Parameters

###### value

`V`

###### Returns

`Buffer`

###### Overrides

`BasicKeyspace.serialize`

##### set()

```ts
set(
   key, 
   value, 
options?): Promise<void>;
```

<!-- source: storage/cache/basic.ts:66 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L66)

Updates the value stored at key to val.

###### Parameters

###### key

`K`

###### value

`V`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`void`\>

###### See

https://redis.io/commands/set/

###### Inherited from

`BasicKeyspace.set`

##### setIfNotExists()

```ts
setIfNotExists(
   key, 
   value, 
options?): Promise<void>;
```

<!-- source: storage/cache/basic.ts:81 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L81)

Sets the value stored at key to val, but only if the key does not exist beforehand.

###### Parameters

###### key

`K`

###### value

`V`

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`Promise`\<`void`\>

###### Throws

If the key already exists.

###### See

https://redis.io/commands/setnx/

###### Inherited from

`BasicKeyspace.setIfNotExists`

##### with()

`with(options): this`

<!-- source: storage/cache/keyspace.ts:123 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L123)

Returns a shallow clone of this keyspace with the specified write options applied.
This allows setting expiry for a chain of operations.

###### Parameters

###### options

[`WriteOptions`](#writeoptions)

###### Returns

`this`

###### Example

`await myKeyspace.with({ expiry: expireIn(5000) }).set(key, value)`

###### Inherited from

`BasicKeyspace.with`

<!-- symbol-end -->

## Interfaces

<!-- symbol-start: CacheClusterConfig -->
### CacheClusterConfig

<!-- source: storage/cache/cluster.ts:20 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/cluster.ts#L20)

Configuration options for a cache cluster.

#### Properties

##### evictionPolicy?

`optional evictionPolicy?: EvictionPolicy`

<!-- source: storage/cache/cluster.ts:25 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/cluster.ts#L25)

The eviction policy to use when the cache is full.
Defaults to "allkeys-lru".

***

<!-- symbol-end -->

<!-- symbol-start: KeyspaceConfig -->
### KeyspaceConfig

<!-- source: storage/cache/keyspace.ts:8 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L8)

Configuration for a cache keyspace.

#### Type Parameters

##### K

`K`

#### Properties

##### defaultExpiry?

`optional defaultExpiry?: Expiry`

<!-- source: storage/cache/keyspace.ts:26 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L26)

Default expiry for cache entries in this keyspace.
If not set, entries do not expire.

##### keyPattern

`keyPattern: string`

<!-- source: storage/cache/keyspace.ts:20 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L20)

The pattern for generating cache keys.
Use `:fieldName` to include a field from the key type.

###### Example

```ts
// For a simple key type (string, number)
keyPattern: "user/:id"

// For a struct key type
keyPattern: "user/:userId/region/:region"
```

***

<!-- symbol-end -->

<!-- symbol-start: WriteOptions -->
### WriteOptions

<!-- source: storage/cache/keyspace.ts:32 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L32)

Options for write operations.

#### Properties

##### expiry?

`optional expiry?: Expiry`

<!-- source: storage/cache/keyspace.ts:37 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L37)

Expiry for this specific write operation.
Overrides the keyspace's defaultExpiry.

<!-- symbol-end -->

## Type Aliases

<!-- symbol-start: EvictionPolicy -->
### EvictionPolicy

```ts
type EvictionPolicy = 
  | "noeviction"
  | "allkeys-lru"
  | "allkeys-lfu"
  | "allkeys-random"
  | "volatile-lru"
  | "volatile-lfu"
  | "volatile-ttl"
  | "volatile-random";
```

<!-- source: storage/cache/cluster.ts:7 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/cluster.ts#L7)

Redis eviction policy that determines how keys are evicted when memory is full.

***

<!-- symbol-end -->

<!-- symbol-start: Expiry -->
### Expiry

```ts
type Expiry = 
  | {
  durationMs: number;
  type: "duration";
}
  | {
  hours: number;
  minutes: number;
  seconds: number;
  type: "time";
}
  | "never"
  | "keep-ttl";
```

<!-- source: storage/cache/expiry.ts:5 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/expiry.ts#L5)

Expiry represents a cache key expiration configuration.
Use the helper functions to create expiry configurations.

***

<!-- symbol-end -->

<!-- symbol-start: ListPosition -->
### ListPosition

`type ListPosition = "left" | "right"`

<!-- source: storage/cache/list.ts:8 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L8)

Position in a list (left/head or right/tail).

<!-- symbol-end -->

## Variables

<!-- symbol-start: keepTTL -->
### keepTTL

`const keepTTL: Expiry = "keep-ttl"`

<!-- source: storage/cache/expiry.ts:67 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/expiry.ts#L67)

keepTTL preserves the existing TTL when updating a cache entry.
If the key doesn't exist, no TTL is set.

***

<!-- symbol-end -->

<!-- symbol-start: neverExpire -->
### neverExpire

`const neverExpire: Expiry = "never"`

<!-- source: storage/cache/expiry.ts:61 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/expiry.ts#L61)

neverExpire sets the cache entry to never expire.
Note: Redis may still evict the key based on the eviction policy.

<!-- symbol-end -->

## Functions

<!-- symbol-start: expireDailyAt() -->
### expireDailyAt()

```ts
function expireDailyAt(
   hours, 
   minutes, 
   seconds): Expiry;
```

<!-- source: storage/cache/expiry.ts:49 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/expiry.ts#L49)

expireDailyAt sets the cache entry to expire at a specific time each day (UTC).

#### Parameters

##### hours

`number`

Hour (0-23)

##### minutes

`number`

Minutes (0-59)

##### seconds

`number`

Seconds (0-59)

#### Returns

[`Expiry`](#expiry-1)

***

<!-- symbol-end -->

<!-- symbol-start: expireIn() -->
### expireIn()

`function expireIn(ms): Expiry`

<!-- source: storage/cache/expiry.ts:15 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/expiry.ts#L15)

expireIn sets the cache entry to expire after the specified duration.

#### Parameters

##### ms

`number`

Duration in milliseconds

#### Returns

[`Expiry`](#expiry-1)

***

<!-- symbol-end -->

<!-- symbol-start: expireInHours() -->
### expireInHours()

`function expireInHours(hours): Expiry`

<!-- source: storage/cache/expiry.ts:39 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/expiry.ts#L39)

expireInHours sets the cache entry to expire after the specified hours.

#### Parameters

##### hours

`number`

Duration in hours

#### Returns

[`Expiry`](#expiry-1)

***

<!-- symbol-end -->

<!-- symbol-start: expireInMinutes() -->
### expireInMinutes()

`function expireInMinutes(minutes): Expiry`

<!-- source: storage/cache/expiry.ts:31 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/expiry.ts#L31)

expireInMinutes sets the cache entry to expire after the specified minutes.

#### Parameters

##### minutes

`number`

Duration in minutes

#### Returns

[`Expiry`](#expiry-1)

***

<!-- symbol-end -->

<!-- symbol-start: expireInSeconds() -->
### expireInSeconds()

`function expireInSeconds(seconds): Expiry`

<!-- source: storage/cache/expiry.ts:23 -->
[source](https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/expiry.ts#L23)

expireInSeconds sets the cache entry to expire after the specified seconds.

#### Parameters

##### seconds

`number`

Duration in seconds

#### Returns

[`Expiry`](#expiry-1)


<!-- symbol-end -->