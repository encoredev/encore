---
title: encore.dev/storage/cache
lang: ts
toc: true
---

## Classes

<!-- symbol-start: CacheCluster -->
### CacheCluster <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/cluster.ts#L43" target="_blank" rel="noopener">source</a>

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

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/cluster.ts#L55" target="_blank" rel="noopener">source</a>

`new CacheCluster(name, cfg?): CacheCluster`

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

##### named() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/cluster.ts#L64" target="_blank" rel="noopener">source</a>

`static named<Name>(name): CacheCluster`

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
### CacheError <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/errors.ts#L4" target="_blank" rel="noopener">source</a>

CacheError is the base class for all cache-related errors.

#### Extends

- `Error`

#### Extended by

- [`CacheMiss`](#cachemiss)
- [`CacheKeyExists`](#cachekeyexists)

#### Constructors

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/errors.ts#L5" target="_blank" rel="noopener">source</a>

`new CacheError(msg): CacheError`

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
### CacheKeyExists <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/errors.ts#L49" target="_blank" rel="noopener">source</a>

CacheKeyExists is thrown when attempting to set a key that already exists
using setIfNotExists.

#### Extends

- [`CacheError`](#cacheerror)

#### Constructors

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/errors.ts#L50" target="_blank" rel="noopener">source</a>

`new CacheKeyExists(key): CacheKeyExists`

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
### CacheMiss <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/errors.ts#L26" target="_blank" rel="noopener">source</a>

CacheMiss is thrown when a cache key is not found.

#### Extends

- [`CacheError`](#cacheerror)

#### Constructors

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/errors.ts#L27" target="_blank" rel="noopener">source</a>

`new CacheMiss(key): CacheMiss`

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
### FloatKeyspace <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L410" target="_blank" rel="noopener">source</a>

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

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L411" target="_blank" rel="noopener">source</a>

`new FloatKeyspace<K>(cluster, config): FloatKeyspace<K>`

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

###### Inherited from

`BasicKeyspace.cluster`

##### config

`protected readonly config: KeyspaceConfig<K>`

###### Inherited from

`BasicKeyspace.config`

##### keyMapper

`protected readonly keyMapper: (key) => string`

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`BasicKeyspace.keyMapper`

#### Methods

##### decrement() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L462" target="_blank" rel="noopener">source</a>

```ts
decrement(
   key, 
   delta?, 
options?): Promise<number>;
```

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

##### delete() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L137" target="_blank" rel="noopener">source</a>

`delete(...keys): Promise<number>`

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

##### deserialize() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L419" target="_blank" rel="noopener">source</a>

`protected deserialize(data): number`

Deserializes a Buffer from storage to a value.

###### Parameters

###### data

`Buffer`

###### Returns

`number`

###### Overrides

`BasicKeyspace.deserialize`

##### get() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L33" target="_blank" rel="noopener">source</a>

`get(key): Promise<number | undefined>`

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

##### getAndDelete() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L165" target="_blank" rel="noopener">source</a>

`getAndDelete(key): Promise<number | undefined>`

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

##### getAndSet() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L134" target="_blank" rel="noopener">source</a>

```ts
getAndSet(
   key, 
   value, 
options?): Promise<number | undefined>;
```

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

##### increment() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L437" target="_blank" rel="noopener">source</a>

```ts
increment(
   key, 
   delta?, 
options?): Promise<number>;
```

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

##### mapKey() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L91" target="_blank" rel="noopener">source</a>

`protected mapKey(key): string`

Maps a key to its Redis key string.

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`BasicKeyspace.mapKey`

##### multiGet() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L52" target="_blank" rel="noopener">source</a>

`multiGet(...keys): Promise<(number | undefined)[]>`

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

##### replace() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L109" target="_blank" rel="noopener">source</a>

```ts
replace(
   key, 
   value, 
options?): Promise<void>;
```

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

##### resolveTtl() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L103" target="_blank" rel="noopener">source</a>

`protected resolveTtl(options?): number | undefined`

Resolves the TTL for a write operation.
Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms

###### Parameters

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`number` \| `undefined`

###### Inherited from

`BasicKeyspace.resolveTtl`

##### serialize() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L415" target="_blank" rel="noopener">source</a>

`protected serialize(value): Buffer`

Serializes a value to a Buffer for storage.

###### Parameters

###### value

`number`

###### Returns

`Buffer`

###### Overrides

`BasicKeyspace.serialize`

##### set() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L66" target="_blank" rel="noopener">source</a>

```ts
set(
   key, 
   value, 
options?): Promise<void>;
```

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

##### setIfNotExists() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L81" target="_blank" rel="noopener">source</a>

```ts
setIfNotExists(
   key, 
   value, 
options?): Promise<void>;
```

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

##### with() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L123" target="_blank" rel="noopener">source</a>

`with(options): this`

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
### IntKeyspace <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L323" target="_blank" rel="noopener">source</a>

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

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L324" target="_blank" rel="noopener">source</a>

`new IntKeyspace<K>(cluster, config): IntKeyspace<K>`

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

###### Inherited from

`BasicKeyspace.cluster`

##### config

`protected readonly config: KeyspaceConfig<K>`

###### Inherited from

`BasicKeyspace.config`

##### keyMapper

`protected readonly keyMapper: (key) => string`

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`BasicKeyspace.keyMapper`

#### Methods

##### decrement() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L380" target="_blank" rel="noopener">source</a>

```ts
decrement(
   key, 
   delta?, 
options?): Promise<number>;
```

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

##### delete() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L137" target="_blank" rel="noopener">source</a>

`delete(...keys): Promise<number>`

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

##### deserialize() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L332" target="_blank" rel="noopener">source</a>

`protected deserialize(data): number`

Deserializes a Buffer from storage to a value.

###### Parameters

###### data

`Buffer`

###### Returns

`number`

###### Overrides

`BasicKeyspace.deserialize`

##### get() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L33" target="_blank" rel="noopener">source</a>

`get(key): Promise<number | undefined>`

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

##### getAndDelete() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L165" target="_blank" rel="noopener">source</a>

`getAndDelete(key): Promise<number | undefined>`

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

##### getAndSet() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L134" target="_blank" rel="noopener">source</a>

```ts
getAndSet(
   key, 
   value, 
options?): Promise<number | undefined>;
```

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

##### increment() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L350" target="_blank" rel="noopener">source</a>

```ts
increment(
   key, 
   delta?, 
options?): Promise<number>;
```

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

##### mapKey() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L91" target="_blank" rel="noopener">source</a>

`protected mapKey(key): string`

Maps a key to its Redis key string.

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`BasicKeyspace.mapKey`

##### multiGet() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L52" target="_blank" rel="noopener">source</a>

`multiGet(...keys): Promise<(number | undefined)[]>`

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

##### replace() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L109" target="_blank" rel="noopener">source</a>

```ts
replace(
   key, 
   value, 
options?): Promise<void>;
```

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

##### resolveTtl() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L103" target="_blank" rel="noopener">source</a>

`protected resolveTtl(options?): number | undefined`

Resolves the TTL for a write operation.
Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms

###### Parameters

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`number` \| `undefined`

###### Inherited from

`BasicKeyspace.resolveTtl`

##### serialize() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L328" target="_blank" rel="noopener">source</a>

`protected serialize(value): Buffer`

Serializes a value to a Buffer for storage.

###### Parameters

###### value

`number`

###### Returns

`Buffer`

###### Overrides

`BasicKeyspace.serialize`

##### set() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L66" target="_blank" rel="noopener">source</a>

```ts
set(
   key, 
   value, 
options?): Promise<void>;
```

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

##### setIfNotExists() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L81" target="_blank" rel="noopener">source</a>

```ts
setIfNotExists(
   key, 
   value, 
options?): Promise<void>;
```

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

##### with() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L123" target="_blank" rel="noopener">source</a>

`with(options): this`

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
### NumberListKeyspace <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L484" target="_blank" rel="noopener">source</a>

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

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L485" target="_blank" rel="noopener">source</a>

`new NumberListKeyspace<K>(cluster, config): NumberListKeyspace<K>`

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

###### Inherited from

`ListKeyspace.cluster`

##### config

`protected readonly config: KeyspaceConfig<K>`

###### Inherited from

`ListKeyspace.config`

##### keyMapper

`protected readonly keyMapper: (key) => string`

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`ListKeyspace.keyMapper`

#### Methods

##### delete() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L137" target="_blank" rel="noopener">source</a>

`delete(...keys): Promise<number>`

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

##### deserializeItem() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L493" target="_blank" rel="noopener">source</a>

`protected deserializeItem(data): number`

###### Parameters

###### data

`Buffer`

###### Returns

`number`

###### Overrides

`ListKeyspace.deserializeItem`

##### get() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L187" target="_blank" rel="noopener">source</a>

`get(key, index): Promise<number | undefined>`

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

##### getRange() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L229" target="_blank" rel="noopener">source</a>

```ts
getRange(
   key, 
   start, 
stop): Promise<number[]>;
```

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

##### insertAfter() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L284" target="_blank" rel="noopener">source</a>

```ts
insertAfter(
   key, 
   pivot, 
   value, 
options?): Promise<number>;
```

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

##### insertBefore() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L252" target="_blank" rel="noopener">source</a>

```ts
insertBefore(
   key, 
   pivot, 
   value, 
options?): Promise<number>;
```

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

##### items() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L207" target="_blank" rel="noopener">source</a>

`items(key): Promise<number[]>`

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

##### len() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L117" target="_blank" rel="noopener">source</a>

`len(key): Promise<number>`

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

##### mapKey() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L91" target="_blank" rel="noopener">source</a>

`protected mapKey(key): string`

Maps a key to its Redis key string.

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`ListKeyspace.mapKey`

##### move() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L417" target="_blank" rel="noopener">source</a>

```ts
move(
   src, 
   dst, 
   srcPos, 
   dstPos, 
options?): Promise<number | undefined>;
```

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

##### popLeft() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L81" target="_blank" rel="noopener">source</a>

`popLeft(key, options?): Promise<number | undefined>`

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

##### popRight() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L98" target="_blank" rel="noopener">source</a>

`popRight(key, options?): Promise<number | undefined>`

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

##### pushLeft() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L35" target="_blank" rel="noopener">source</a>

`pushLeft(key, ...values): Promise<number>`

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

##### pushRight() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L61" target="_blank" rel="noopener">source</a>

`pushRight(key, ...values): Promise<number>`

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

##### removeAll() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L315" target="_blank" rel="noopener">source</a>

```ts
removeAll(
   key, 
   value, 
options?): Promise<number>;
```

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

##### removeFirst() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L341" target="_blank" rel="noopener">source</a>

```ts
removeFirst(
   key, 
   count, 
   value, 
options?): Promise<number>;
```

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

##### removeLast() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L376" target="_blank" rel="noopener">source</a>

```ts
removeLast(
   key, 
   count, 
   value, 
options?): Promise<number>;
```

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

##### resolveTtl() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L103" target="_blank" rel="noopener">source</a>

`protected resolveTtl(options?): number | undefined`

Resolves the TTL for a write operation.
Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms

###### Parameters

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`number` \| `undefined`

###### Inherited from

`ListKeyspace.resolveTtl`

##### serializeItem() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L489" target="_blank" rel="noopener">source</a>

`protected serializeItem(value): Buffer`

###### Parameters

###### value

`number`

###### Returns

`Buffer`

###### Overrides

`ListKeyspace.serializeItem`

##### set() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L163" target="_blank" rel="noopener">source</a>

```ts
set(
   key, 
   index, 
   value, 
options?): Promise<void>;
```

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

##### trim() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L139" target="_blank" rel="noopener">source</a>

```ts
trim(
   key, 
   start, 
   stop, 
options?): Promise<void>;
```

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

##### with() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L123" target="_blank" rel="noopener">source</a>

`with(options): this`

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
### NumberSetKeyspace <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L479" target="_blank" rel="noopener">source</a>

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

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L480" target="_blank" rel="noopener">source</a>

`new NumberSetKeyspace<K>(cluster, config): NumberSetKeyspace<K>`

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

###### Inherited from

`SetKeyspace.cluster`

##### config

`protected readonly config: KeyspaceConfig<K>`

###### Inherited from

`SetKeyspace.config`

##### keyMapper

`protected readonly keyMapper: (key) => string`

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`SetKeyspace.keyMapper`

#### Methods

##### add() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L26" target="_blank" rel="noopener">source</a>

`add(key, ...members): Promise<number>`

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

##### contains() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L111" target="_blank" rel="noopener">source</a>

`contains(key, member): Promise<boolean>`

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

##### delete() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L137" target="_blank" rel="noopener">source</a>

`delete(...keys): Promise<number>`

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

##### deserializeItem() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L488" target="_blank" rel="noopener">source</a>

`protected deserializeItem(data): number`

###### Parameters

###### data

`Buffer`

###### Returns

`number`

###### Overrides

`SetKeyspace.deserializeItem`

##### diff() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L174" target="_blank" rel="noopener">source</a>

`diff(...keys): Promise<number[]>`

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

##### diffSet() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L189" target="_blank" rel="noopener">source</a>

`diffSet(...keys): Promise<Set<number>>`

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

##### diffStore() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L203" target="_blank" rel="noopener">source</a>

`diffStore(destination, ...keys): Promise<number>`

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

##### intersect() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L233" target="_blank" rel="noopener">source</a>

`intersect(...keys): Promise<number[]>`

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

##### intersectSet() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L248" target="_blank" rel="noopener">source</a>

`intersectSet(...keys): Promise<Set<number>>`

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

##### intersectStore() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L262" target="_blank" rel="noopener">source</a>

`intersectStore(destination, ...keys): Promise<number>`

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

##### items() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L141" target="_blank" rel="noopener">source</a>

`items(key): Promise<number[]>`

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

##### itemsSet() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L156" target="_blank" rel="noopener">source</a>

`itemsSet(key): Promise<Set<number>>`

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

##### len() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L126" target="_blank" rel="noopener">source</a>

`len(key): Promise<number>`

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

##### mapKey() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L91" target="_blank" rel="noopener">source</a>

`protected mapKey(key): string`

Maps a key to its Redis key string.

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`SetKeyspace.mapKey`

##### move() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L416" target="_blank" rel="noopener">source</a>

```ts
move(
   src, 
   dst, 
   member, 
options?): Promise<boolean>;
```

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

##### pop() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L90" target="_blank" rel="noopener">source</a>

```ts
pop(
   key, 
   count, 
options?): Promise<number[]>;
```

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

##### popOne() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L68" target="_blank" rel="noopener">source</a>

`popOne(key, options?): Promise<number | undefined>`

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

##### remove() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L48" target="_blank" rel="noopener">source</a>

`remove(key, ...members): Promise<number>`

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

##### resolveTtl() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L103" target="_blank" rel="noopener">source</a>

`protected resolveTtl(options?): number | undefined`

Resolves the TTL for a write operation.
Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms

###### Parameters

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`number` \| `undefined`

###### Inherited from

`SetKeyspace.resolveTtl`

##### sample() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L364" target="_blank" rel="noopener">source</a>

`sample(key, count): Promise<number[]>`

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

##### sampleOne() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L343" target="_blank" rel="noopener">source</a>

`sampleOne(key): Promise<number | undefined>`

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

##### sampleWithReplacement() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L389" target="_blank" rel="noopener">source</a>

`sampleWithReplacement(key, count): Promise<number[]>`

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

##### serializeItem() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L484" target="_blank" rel="noopener">source</a>

`protected serializeItem(value): Buffer`

###### Parameters

###### value

`number`

###### Returns

`Buffer`

###### Overrides

`SetKeyspace.serializeItem`

##### union() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L291" target="_blank" rel="noopener">source</a>

`union(...keys): Promise<number[]>`

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

##### unionSet() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L306" target="_blank" rel="noopener">source</a>

`unionSet(...keys): Promise<Set<number>>`

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

##### unionStore() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L320" target="_blank" rel="noopener">source</a>

`unionStore(destination, ...keys): Promise<number>`

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

##### with() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L123" target="_blank" rel="noopener">source</a>

`with(options): this`

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
### StringKeyspace <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L193" target="_blank" rel="noopener">source</a>

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

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L194" target="_blank" rel="noopener">source</a>

`new StringKeyspace<K>(cluster, config): StringKeyspace<K>`

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

###### Inherited from

[`IntKeyspace`](#intkeyspace).[`cluster`](#cluster-1)

##### config

`protected readonly config: KeyspaceConfig<K>`

###### Inherited from

`BasicKeyspace.config`

##### keyMapper

`protected readonly keyMapper: (key) => string`

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`BasicKeyspace.keyMapper`

#### Methods

##### append() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L215" target="_blank" rel="noopener">source</a>

```ts
append(
   key, 
   value, 
options?): Promise<number>;
```

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

##### delete() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L137" target="_blank" rel="noopener">source</a>

`delete(...keys): Promise<number>`

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

##### deserialize() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L202" target="_blank" rel="noopener">source</a>

`protected deserialize(data): string`

Deserializes a Buffer from storage to a value.

###### Parameters

###### data

`Buffer`

###### Returns

`string`

###### Overrides

`BasicKeyspace.deserialize`

##### get() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L33" target="_blank" rel="noopener">source</a>

`get(key): Promise<string | undefined>`

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

##### getAndDelete() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L165" target="_blank" rel="noopener">source</a>

`getAndDelete(key): Promise<string | undefined>`

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

##### getAndSet() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L134" target="_blank" rel="noopener">source</a>

```ts
getAndSet(
   key, 
   value, 
options?): Promise<string | undefined>;
```

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

##### getRange() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L245" target="_blank" rel="noopener">source</a>

```ts
getRange(
   key, 
   start, 
end): Promise<string>;
```

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

##### len() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L300" target="_blank" rel="noopener">source</a>

`len(key): Promise<number>`

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

##### mapKey() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L91" target="_blank" rel="noopener">source</a>

`protected mapKey(key): string`

Maps a key to its Redis key string.

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`BasicKeyspace.mapKey`

##### multiGet() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L52" target="_blank" rel="noopener">source</a>

`multiGet(...keys): Promise<(string | undefined)[]>`

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

##### replace() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L109" target="_blank" rel="noopener">source</a>

```ts
replace(
   key, 
   value, 
options?): Promise<void>;
```

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

##### resolveTtl() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L103" target="_blank" rel="noopener">source</a>

`protected resolveTtl(options?): number | undefined`

Resolves the TTL for a write operation.
Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms

###### Parameters

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`number` \| `undefined`

###### Inherited from

`BasicKeyspace.resolveTtl`

##### serialize() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L198" target="_blank" rel="noopener">source</a>

`protected serialize(value): Buffer`

Serializes a value to a Buffer for storage.

###### Parameters

###### value

`string`

###### Returns

`Buffer`

###### Overrides

`BasicKeyspace.serialize`

##### set() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L66" target="_blank" rel="noopener">source</a>

```ts
set(
   key, 
   value, 
options?): Promise<void>;
```

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

##### setIfNotExists() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L81" target="_blank" rel="noopener">source</a>

```ts
setIfNotExists(
   key, 
   value, 
options?): Promise<void>;
```

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

##### setRange() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L273" target="_blank" rel="noopener">source</a>

```ts
setRange(
   key, 
   offset, 
   value, 
options?): Promise<number>;
```

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

##### with() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L123" target="_blank" rel="noopener">source</a>

`with(options): this`

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
### StringListKeyspace <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L457" target="_blank" rel="noopener">source</a>

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

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L458" target="_blank" rel="noopener">source</a>

`new StringListKeyspace<K>(cluster, config): StringListKeyspace<K>`

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

###### Inherited from

`ListKeyspace.cluster`

##### config

`protected readonly config: KeyspaceConfig<K>`

###### Inherited from

`ListKeyspace.config`

##### keyMapper

`protected readonly keyMapper: (key) => string`

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`ListKeyspace.keyMapper`

#### Methods

##### delete() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L137" target="_blank" rel="noopener">source</a>

`delete(...keys): Promise<number>`

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

##### deserializeItem() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L466" target="_blank" rel="noopener">source</a>

`protected deserializeItem(data): string`

###### Parameters

###### data

`Buffer`

###### Returns

`string`

###### Overrides

`ListKeyspace.deserializeItem`

##### get() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L187" target="_blank" rel="noopener">source</a>

`get(key, index): Promise<string | undefined>`

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

##### getRange() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L229" target="_blank" rel="noopener">source</a>

```ts
getRange(
   key, 
   start, 
stop): Promise<string[]>;
```

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

##### insertAfter() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L284" target="_blank" rel="noopener">source</a>

```ts
insertAfter(
   key, 
   pivot, 
   value, 
options?): Promise<number>;
```

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

##### insertBefore() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L252" target="_blank" rel="noopener">source</a>

```ts
insertBefore(
   key, 
   pivot, 
   value, 
options?): Promise<number>;
```

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

##### items() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L207" target="_blank" rel="noopener">source</a>

`items(key): Promise<string[]>`

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

##### len() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L117" target="_blank" rel="noopener">source</a>

`len(key): Promise<number>`

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

##### mapKey() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L91" target="_blank" rel="noopener">source</a>

`protected mapKey(key): string`

Maps a key to its Redis key string.

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`ListKeyspace.mapKey`

##### move() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L417" target="_blank" rel="noopener">source</a>

```ts
move(
   src, 
   dst, 
   srcPos, 
   dstPos, 
options?): Promise<string | undefined>;
```

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

##### popLeft() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L81" target="_blank" rel="noopener">source</a>

`popLeft(key, options?): Promise<string | undefined>`

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

##### popRight() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L98" target="_blank" rel="noopener">source</a>

`popRight(key, options?): Promise<string | undefined>`

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

##### pushLeft() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L35" target="_blank" rel="noopener">source</a>

`pushLeft(key, ...values): Promise<number>`

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

##### pushRight() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L61" target="_blank" rel="noopener">source</a>

`pushRight(key, ...values): Promise<number>`

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

##### removeAll() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L315" target="_blank" rel="noopener">source</a>

```ts
removeAll(
   key, 
   value, 
options?): Promise<number>;
```

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

##### removeFirst() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L341" target="_blank" rel="noopener">source</a>

```ts
removeFirst(
   key, 
   count, 
   value, 
options?): Promise<number>;
```

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

##### removeLast() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L376" target="_blank" rel="noopener">source</a>

```ts
removeLast(
   key, 
   count, 
   value, 
options?): Promise<number>;
```

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

##### resolveTtl() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L103" target="_blank" rel="noopener">source</a>

`protected resolveTtl(options?): number | undefined`

Resolves the TTL for a write operation.
Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms

###### Parameters

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`number` \| `undefined`

###### Inherited from

`ListKeyspace.resolveTtl`

##### serializeItem() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L462" target="_blank" rel="noopener">source</a>

`protected serializeItem(value): Buffer`

###### Parameters

###### value

`string`

###### Returns

`Buffer`

###### Overrides

`ListKeyspace.serializeItem`

##### set() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L163" target="_blank" rel="noopener">source</a>

```ts
set(
   key, 
   index, 
   value, 
options?): Promise<void>;
```

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

##### trim() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L139" target="_blank" rel="noopener">source</a>

```ts
trim(
   key, 
   start, 
   stop, 
options?): Promise<void>;
```

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

##### with() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L123" target="_blank" rel="noopener">source</a>

`with(options): this`

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
### StringSetKeyspace <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L452" target="_blank" rel="noopener">source</a>

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

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L453" target="_blank" rel="noopener">source</a>

`new StringSetKeyspace<K>(cluster, config): StringSetKeyspace<K>`

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

###### Inherited from

`SetKeyspace.cluster`

##### config

`protected readonly config: KeyspaceConfig<K>`

###### Inherited from

`SetKeyspace.config`

##### keyMapper

`protected readonly keyMapper: (key) => string`

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`SetKeyspace.keyMapper`

#### Methods

##### add() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L26" target="_blank" rel="noopener">source</a>

`add(key, ...members): Promise<number>`

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

##### contains() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L111" target="_blank" rel="noopener">source</a>

`contains(key, member): Promise<boolean>`

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

##### delete() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L137" target="_blank" rel="noopener">source</a>

`delete(...keys): Promise<number>`

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

##### deserializeItem() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L461" target="_blank" rel="noopener">source</a>

`protected deserializeItem(data): string`

###### Parameters

###### data

`Buffer`

###### Returns

`string`

###### Overrides

`SetKeyspace.deserializeItem`

##### diff() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L174" target="_blank" rel="noopener">source</a>

`diff(...keys): Promise<string[]>`

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

##### diffSet() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L189" target="_blank" rel="noopener">source</a>

`diffSet(...keys): Promise<Set<string>>`

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

##### diffStore() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L203" target="_blank" rel="noopener">source</a>

`diffStore(destination, ...keys): Promise<number>`

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

##### intersect() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L233" target="_blank" rel="noopener">source</a>

`intersect(...keys): Promise<string[]>`

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

##### intersectSet() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L248" target="_blank" rel="noopener">source</a>

`intersectSet(...keys): Promise<Set<string>>`

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

##### intersectStore() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L262" target="_blank" rel="noopener">source</a>

`intersectStore(destination, ...keys): Promise<number>`

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

##### items() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L141" target="_blank" rel="noopener">source</a>

`items(key): Promise<string[]>`

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

##### itemsSet() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L156" target="_blank" rel="noopener">source</a>

`itemsSet(key): Promise<Set<string>>`

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

##### len() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L126" target="_blank" rel="noopener">source</a>

`len(key): Promise<number>`

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

##### mapKey() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L91" target="_blank" rel="noopener">source</a>

`protected mapKey(key): string`

Maps a key to its Redis key string.

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`SetKeyspace.mapKey`

##### move() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L416" target="_blank" rel="noopener">source</a>

```ts
move(
   src, 
   dst, 
   member, 
options?): Promise<boolean>;
```

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

##### pop() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L90" target="_blank" rel="noopener">source</a>

```ts
pop(
   key, 
   count, 
options?): Promise<string[]>;
```

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

##### popOne() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L68" target="_blank" rel="noopener">source</a>

`popOne(key, options?): Promise<string | undefined>`

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

##### remove() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L48" target="_blank" rel="noopener">source</a>

`remove(key, ...members): Promise<number>`

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

##### resolveTtl() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L103" target="_blank" rel="noopener">source</a>

`protected resolveTtl(options?): number | undefined`

Resolves the TTL for a write operation.
Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms

###### Parameters

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`number` \| `undefined`

###### Inherited from

`SetKeyspace.resolveTtl`

##### sample() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L364" target="_blank" rel="noopener">source</a>

`sample(key, count): Promise<string[]>`

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

##### sampleOne() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L343" target="_blank" rel="noopener">source</a>

`sampleOne(key): Promise<string | undefined>`

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

##### sampleWithReplacement() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L389" target="_blank" rel="noopener">source</a>

`sampleWithReplacement(key, count): Promise<string[]>`

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

##### serializeItem() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L457" target="_blank" rel="noopener">source</a>

`protected serializeItem(value): Buffer`

###### Parameters

###### value

`string`

###### Returns

`Buffer`

###### Overrides

`SetKeyspace.serializeItem`

##### union() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L291" target="_blank" rel="noopener">source</a>

`union(...keys): Promise<string[]>`

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

##### unionSet() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L306" target="_blank" rel="noopener">source</a>

`unionSet(...keys): Promise<Set<string>>`

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

##### unionStore() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/set.ts#L320" target="_blank" rel="noopener">source</a>

`unionStore(destination, ...keys): Promise<number>`

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

##### with() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L123" target="_blank" rel="noopener">source</a>

`with(options): this`

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
### StructKeyspace <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L499" target="_blank" rel="noopener">source</a>

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

##### Constructor <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L500" target="_blank" rel="noopener">source</a>

`new StructKeyspace<K, V>(cluster, config): StructKeyspace<K, V>`

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

###### Inherited from

`BasicKeyspace.cluster`

##### config

`protected readonly config: KeyspaceConfig<K>`

###### Inherited from

`BasicKeyspace.config`

##### keyMapper

`protected readonly keyMapper: (key) => string`

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`BasicKeyspace.keyMapper`

#### Methods

##### delete() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L137" target="_blank" rel="noopener">source</a>

`delete(...keys): Promise<number>`

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

##### deserialize() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L508" target="_blank" rel="noopener">source</a>

`protected deserialize(data): V`

Deserializes a Buffer from storage to a value.

###### Parameters

###### data

`Buffer`

###### Returns

`V`

###### Overrides

`BasicKeyspace.deserialize`

##### get() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L33" target="_blank" rel="noopener">source</a>

`get(key): Promise<V | undefined>`

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

##### getAndDelete() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L165" target="_blank" rel="noopener">source</a>

`getAndDelete(key): Promise<V | undefined>`

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

##### getAndSet() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L134" target="_blank" rel="noopener">source</a>

```ts
getAndSet(
   key, 
   value, 
options?): Promise<V | undefined>;
```

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

##### mapKey() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L91" target="_blank" rel="noopener">source</a>

`protected mapKey(key): string`

Maps a key to its Redis key string.

###### Parameters

###### key

`K`

###### Returns

`string`

###### Inherited from

`BasicKeyspace.mapKey`

##### multiGet() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L52" target="_blank" rel="noopener">source</a>

`multiGet(...keys): Promise<(V | undefined)[]>`

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

##### replace() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L109" target="_blank" rel="noopener">source</a>

```ts
replace(
   key, 
   value, 
options?): Promise<void>;
```

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

##### resolveTtl() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L103" target="_blank" rel="noopener">source</a>

`protected resolveTtl(options?): number | undefined`

Resolves the TTL for a write operation.
Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms

###### Parameters

###### options?

[`WriteOptions`](#writeoptions)

###### Returns

`number` \| `undefined`

###### Inherited from

`BasicKeyspace.resolveTtl`

##### serialize() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L504" target="_blank" rel="noopener">source</a>

`protected serialize(value): Buffer`

Serializes a value to a Buffer for storage.

###### Parameters

###### value

`V`

###### Returns

`Buffer`

###### Overrides

`BasicKeyspace.serialize`

##### set() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L66" target="_blank" rel="noopener">source</a>

```ts
set(
   key, 
   value, 
options?): Promise<void>;
```

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

##### setIfNotExists() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/basic.ts#L81" target="_blank" rel="noopener">source</a>

```ts
setIfNotExists(
   key, 
   value, 
options?): Promise<void>;
```

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

##### with() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L123" target="_blank" rel="noopener">source</a>

`with(options): this`

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
### CacheClusterConfig <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/cluster.ts#L20" target="_blank" rel="noopener">source</a>

Configuration options for a cache cluster.

#### Properties

##### evictionPolicy?

`optional evictionPolicy?: EvictionPolicy`

The eviction policy to use when the cache is full.
Defaults to "allkeys-lru".

***

<!-- symbol-end -->

<!-- symbol-start: KeyspaceConfig -->
### KeyspaceConfig <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L8" target="_blank" rel="noopener">source</a>

Configuration for a cache keyspace.

#### Type Parameters

##### K

`K`

#### Properties

##### defaultExpiry?

`optional defaultExpiry?: Expiry`

Default expiry for cache entries in this keyspace.
If not set, entries do not expire.

##### keyPattern

`keyPattern: string`

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
### WriteOptions <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/keyspace.ts#L32" target="_blank" rel="noopener">source</a>

Options for write operations.

#### Properties

##### expiry?

`optional expiry?: Expiry`

Expiry for this specific write operation.
Overrides the keyspace's defaultExpiry.

<!-- symbol-end -->

## Type Aliases

<!-- symbol-start: EvictionPolicy -->
### EvictionPolicy <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/cluster.ts#L7" target="_blank" rel="noopener">source</a>

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

Redis eviction policy that determines how keys are evicted when memory is full.

***

<!-- symbol-end -->

<!-- symbol-start: Expiry -->
### Expiry <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/expiry.ts#L5" target="_blank" rel="noopener">source</a>

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

Expiry represents a cache key expiration configuration.
Use the helper functions to create expiry configurations.

***

<!-- symbol-end -->

<!-- symbol-start: ListPosition -->
### ListPosition <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/list.ts#L8" target="_blank" rel="noopener">source</a>

`type ListPosition = "left" | "right"`

Position in a list (left/head or right/tail).

<!-- symbol-end -->

## Variables

<!-- symbol-start: keepTTL -->
### keepTTL <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/expiry.ts#L67" target="_blank" rel="noopener">source</a>

`const keepTTL: Expiry = "keep-ttl"`

keepTTL preserves the existing TTL when updating a cache entry.
If the key doesn't exist, no TTL is set.

***

<!-- symbol-end -->

<!-- symbol-start: neverExpire -->
### neverExpire <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/expiry.ts#L61" target="_blank" rel="noopener">source</a>

`const neverExpire: Expiry = "never"`

neverExpire sets the cache entry to never expire.
Note: Redis may still evict the key based on the eviction policy.

<!-- symbol-end -->

## Functions

<!-- symbol-start: expireDailyAt() -->
### expireDailyAt() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/expiry.ts#L49" target="_blank" rel="noopener">source</a>

```ts
function expireDailyAt(
   hours, 
   minutes, 
   seconds): Expiry;
```

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
### expireIn() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/expiry.ts#L15" target="_blank" rel="noopener">source</a>

`function expireIn(ms): Expiry`

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
### expireInHours() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/expiry.ts#L39" target="_blank" rel="noopener">source</a>

`function expireInHours(hours): Expiry`

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
### expireInMinutes() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/expiry.ts#L31" target="_blank" rel="noopener">source</a>

`function expireInMinutes(minutes): Expiry`

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
### expireInSeconds() <a class="symbol-source" href="https://github.com/encoredev/encore/blob/main/runtimes/js/encore.dev/storage/cache/expiry.ts#L23" target="_blank" rel="noopener">source</a>

`function expireInSeconds(seconds): Expiry`

expireInSeconds sets the cache entry to expire after the specified seconds.

#### Parameters

##### seconds

`number`

Duration in seconds

#### Returns

[`Expiry`](#expiry-1)


<!-- symbol-end -->