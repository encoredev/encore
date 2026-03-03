import { getCurrentRequest } from "../../internal/reqtrack/mod";
import { CacheCluster } from "./cluster";
import { CacheMiss, CacheKeyExists } from "./errors";
import { Keyspace, KeyspaceConfig, WriteOptions } from "./keyspace";

/**
 * Base class for basic (scalar value) keyspaces.
 * Provides get/set/replace/etc operations.
 * @internal
 */
abstract class BasicKeyspace<K, V> extends Keyspace<K> {
  constructor(cluster: CacheCluster, config: KeyspaceConfig<K>) {
    super(cluster, config);
  }

  /**
   * Serializes a value to a Buffer for storage.
   */
  protected abstract serialize(value: V): Buffer;

  /**
   * Deserializes a Buffer from storage to a value.
   */
  protected abstract deserialize(data: Buffer): V;

  /**
   * Gets the value stored at key.
   * If the key does not exist, it returns `undefined`.
   *
   * @returns The value, or `undefined` if the key does not exist.
   * @see https://redis.io/commands/get/
   */
  async get(key: K): Promise<V | undefined> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const result = await this.cluster.impl.get(mappedKey, source);

    if (result === null) {
      return undefined;
    }

    return this.deserialize(result);
  }

  /**
   * Gets the values stored at multiple keys.
   *
   * @returns An array of values in the same order as the provided keys.
   * Each element is the value or `undefined` if the key was not found.
   * @see https://redis.io/commands/mget/
   */
  async multiGet(...keys: K[]): Promise<(V | undefined)[]> {
    const source = getCurrentRequest();
    const mappedKeys = keys.map((k) => this.mapKey(k));
    const results = await this.cluster.impl.mget(mappedKeys, source);
    return results.map((r) =>
      r === null || r === undefined ? undefined : this.deserialize(r)
    );
  }

  /**
   * Updates the value stored at key to val.
   *
   * @see https://redis.io/commands/set/
   */
  async set(key: K, value: V, options?: WriteOptions): Promise<void> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = this.serialize(value);
    const ttlMs = this.resolveTtl(options);

    await this.cluster.impl.set(mappedKey, serialized, ttlMs, source);
  }

  /**
   * Sets the value stored at key to val, but only if the key does not exist beforehand.
   *
   * @throws {CacheKeyExists} If the key already exists.
   * @see https://redis.io/commands/setnx/
   */
  async setIfNotExists(
    key: K,
    value: V,
    options?: WriteOptions
  ): Promise<void> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = this.serialize(value);
    const ttlMs = this.resolveTtl(options);

    const set = await this.cluster.impl.setIfNotExists(
      mappedKey,
      serialized,
      ttlMs,
      source
    );

    if (!set) {
      throw new CacheKeyExists(mappedKey);
    }
  }

  /**
   * Replaces the existing value stored at key to val.
   *
   * @throws {CacheMiss} If the key does not already exist.
   * @see https://redis.io/commands/set/
   */
  async replace(key: K, value: V, options?: WriteOptions): Promise<void> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = this.serialize(value);
    const ttlMs = this.resolveTtl(options);

    const replaced = await this.cluster.impl.replace(
      mappedKey,
      serialized,
      ttlMs,
      source
    );

    if (!replaced) {
      throw new CacheMiss(mappedKey);
    }
  }

  /**
   * Updates the value of key to val and returns the previously stored value.
   * If the key does not already exist, it sets it and returns `undefined`.
   *
   * @returns The previous value, or `undefined` if the key did not exist.
   * @see https://redis.io/commands/getset/
   */
  async getAndSet(
    key: K,
    value: V,
    options?: WriteOptions
  ): Promise<V | undefined> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = this.serialize(value);
    const ttlMs = this.resolveTtl(options);

    const oldValue = await this.cluster.impl.getAndSet(
      mappedKey,
      serialized,
      ttlMs,
      source
    );

    if (oldValue === null) {
      return undefined;
    }

    return this.deserialize(oldValue);
  }

  /**
   * Deletes the key and returns the previously stored value.
   * If the key does not already exist, it returns `undefined`.
   *
   * @returns The previous value, or `undefined` if the key did not exist.
   * @see https://redis.io/commands/getdel/
   */
  async getAndDelete(key: K): Promise<V | undefined> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);

    const value = await this.cluster.impl.getAndDelete(mappedKey, source);

    if (value === null) {
      return undefined;
    }

    return this.deserialize(value);
  }
}

/**
 * StringKeyspace stores string values.
 *
 * @example
 * ```ts
 * const tokens = new StringKeyspace<string>(cluster, {
 *   keyPattern: "token/:id",
 *   defaultExpiry: ExpireIn(3600000), // 1 hour
 * });
 *
 * await tokens.set("abc123", "user-token-value");
 * const token = await tokens.get("abc123");
 * ```
 */
export class StringKeyspace<K> extends BasicKeyspace<K, string> {
  constructor(cluster: CacheCluster, config: KeyspaceConfig<K>) {
    super(cluster, config);
  }

  protected serialize(value: string): Buffer {
    return Buffer.from(value, "utf-8");
  }

  protected deserialize(data: Buffer): string {
    return data.toString("utf-8");
  }

  /**
   * Appends a string to the value stored at key.
   *
   * If the key does not exist it is first created and set as the empty string,
   * causing append to behave like set.
   *
   * @returns The new string length.
   * @see https://redis.io/commands/append/
   */
  async append(key: K, value: string, options?: WriteOptions): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const ttlMs = this.resolveTtl(options);
    const result = await this.cluster.impl.append(
      mappedKey,
      Buffer.from(value, "utf-8"),
      ttlMs,
      source
    );
    return Number(result);
  }

  /**
   * Returns a substring of the string value stored at key.
   *
   * The `start` and `end` values are zero-based indices, but unlike typical slicing
   * the `end` value is inclusive.
   *
   * Negative values can be used in order to provide an offset starting
   * from the end of the string, so -1 means the last character.
   *
   * If the string does not exist it returns the empty string.
   *
   * @param key - The cache key.
   * @param start - Start index (inclusive, 0-based).
   * @param end - End index (inclusive, 0-based). Use -1 for end of string.
   * @returns The substring.
   * @see https://redis.io/commands/getrange/
   */
  async getRange(key: K, start: number, end: number): Promise<string> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const result = await this.cluster.impl.getRange(
      mappedKey,
      start,
      end,
      source
    );
    return result.toString("utf-8");
  }

  /**
   * Overwrites part of the string stored at key, starting at
   * the zero-based `offset` and for the entire length of `value`, extending
   * the string if necessary.
   *
   * If the offset is larger than the current string length stored at key,
   * the string is first padded with zero-bytes to make offset fit.
   *
   * Non-existing keys are considered as empty strings.
   *
   * @param key - The cache key.
   * @param offset - Zero-based byte offset to start writing at.
   * @param value - The string to write.
   * @returns The length of the string after the operation.
   * @see https://redis.io/commands/setrange/
   */
  async setRange(
    key: K,
    offset: number,
    value: string,
    options?: WriteOptions
  ): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const ttlMs = this.resolveTtl(options);
    const result = await this.cluster.impl.setRange(
      mappedKey,
      offset,
      Buffer.from(value, "utf-8"),
      ttlMs,
      source
    );
    return Number(result);
  }

  /**
   * Returns the length of the string value stored at key.
   *
   * Non-existing keys are considered as empty strings.
   *
   * @returns The string length.
   * @see https://redis.io/commands/strlen/
   */
  async len(key: K): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const result = await this.cluster.impl.strlen(mappedKey, source);
    return Number(result);
  }
}

/**
 * IntKeyspace stores 64-bit integer values.
 * Values are floored to integers using `Math.floor`.
 * For fractional values, use {@link FloatKeyspace} instead.
 *
 * @example
 * ```ts
 * const counters = new IntKeyspace<string>(cluster, {
 *   keyPattern: "counter/:name",
 * });
 *
 * await counters.set("page-views", 0);
 * const newCount = await counters.increment("page-views", 1);
 * ```
 */
export class IntKeyspace<K> extends BasicKeyspace<K, number> {
  constructor(cluster: CacheCluster, config: KeyspaceConfig<K>) {
    super(cluster, config);
  }

  protected serialize(value: number): Buffer {
    return Buffer.from(String(Math.floor(value)), "utf-8");
  }

  protected deserialize(data: Buffer): number {
    return parseInt(data.toString("utf-8"), 10);
  }

  /**
   * Increments the number stored at key by `delta`.
   *
   * If the key does not exist it is first created with a value of 0
   * before incrementing.
   *
   * Negative values can be used to decrease the value,
   * but typically you want to use {@link decrement} for that.
   *
   * @param key - The cache key.
   * @param delta - The amount to increment by (default 1).
   * @returns The new value after incrementing.
   * @see https://redis.io/commands/incrby/
   */
  async increment(
    key: K,
    delta: number = 1,
    options?: WriteOptions
  ): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const ttlMs = this.resolveTtl(options);
    return await this.cluster.impl.incrBy(
      mappedKey,
      Math.floor(delta),
      ttlMs,
      source
    );
  }

  /**
   * Decrements the number stored at key by `delta`.
   *
   * If the key does not exist it is first created with a value of 0
   * before decrementing.
   *
   * Negative values can be used to increase the value,
   * but typically you want to use {@link increment} for that.
   *
   * @param key - The cache key.
   * @param delta - The amount to decrement by (default 1).
   * @returns The new value after decrementing.
   * @see https://redis.io/commands/decrby/
   */
  async decrement(
    key: K,
    delta: number = 1,
    options?: WriteOptions
  ): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const ttlMs = this.resolveTtl(options);
    return await this.cluster.impl.decrBy(
      mappedKey,
      Math.floor(delta),
      ttlMs,
      source
    );
  }
}

/**
 * FloatKeyspace stores 64-bit floating point values.
 *
 * @example
 * ```ts
 * const scores = new FloatKeyspace<string>(cluster, {
 *   keyPattern: "score/:playerId",
 * });
 *
 * await scores.set("player1", 100.5);
 * const newScore = await scores.increment("player1", 10.25);
 * ```
 */
export class FloatKeyspace<K> extends BasicKeyspace<K, number> {
  constructor(cluster: CacheCluster, config: KeyspaceConfig<K>) {
    super(cluster, config);
  }

  protected serialize(value: number): Buffer {
    return Buffer.from(String(value), "utf-8");
  }

  protected deserialize(data: Buffer): number {
    return parseFloat(data.toString("utf-8"));
  }

  /**
   * Increments the number stored at key by `delta`.
   *
   * If the key does not exist it is first created with a value of 0
   * before incrementing.
   *
   * Negative values can be used to decrease the value,
   * but typically you want to use {@link decrement} for that.
   *
   * @param key - The cache key.
   * @param delta - The amount to increment by (default 1).
   * @returns The new value after incrementing.
   * @see https://redis.io/commands/incrbyfloat/
   */
  async increment(
    key: K,
    delta: number = 1,
    options?: WriteOptions
  ): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const ttlMs = this.resolveTtl(options);
    return await this.cluster.impl.incrByFloat(mappedKey, delta, ttlMs, source);
  }

  /**
   * Decrements the number stored at key by `delta`.
   *
   * If the key does not exist it is first created with a value of 0
   * before decrementing.
   *
   * Negative values can be used to increase the value,
   * but typically you want to use {@link increment} for that.
   *
   * @param key - The cache key.
   * @param delta - The amount to decrement by (default 1).
   * @returns The new value after decrementing.
   * @see https://redis.io/commands/incrbyfloat/
   */
  async decrement(
    key: K,
    delta: number = 1,
    options?: WriteOptions
  ): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const ttlMs = this.resolveTtl(options);
    return await this.cluster.impl.incrByFloat(
      mappedKey,
      -delta,
      ttlMs,
      source
    );
  }
}

/**
 * StructKeyspace stores arbitrary objects serialized as JSON.
 *
 * @example
 * ```ts
 * interface User {
 *   id: string;
 *   name: string;
 *   email: string;
 * }
 *
 * const users = new StructKeyspace<string, User>(cluster, {
 *   keyPattern: "user/:id",
 *   defaultExpiry: ExpireIn(3600000),
 * });
 *
 * await users.set("user1", { id: "user1", name: "Alice", email: "alice@example.com" });
 * const user = await users.get("user1");
 * ```
 */
export class StructKeyspace<K, V> extends BasicKeyspace<K, V> {
  constructor(cluster: CacheCluster, config: KeyspaceConfig<K>) {
    super(cluster, config);
  }

  protected serialize(value: V): Buffer {
    return Buffer.from(JSON.stringify(value), "utf-8");
  }

  protected deserialize(data: Buffer): V {
    return JSON.parse(data.toString("utf-8")) as V;
  }
}
