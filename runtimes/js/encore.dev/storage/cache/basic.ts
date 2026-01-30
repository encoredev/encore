import { getCurrentRequest } from "../../internal/reqtrack/mod";
import { CacheCluster } from "./cluster";
import { CacheMiss, CacheKeyExists } from "./errors";
import { Keyspace, KeyspaceConfig, WriteOptions } from "./keyspace";

/**
 * Result of a get operation that may or may not find a value.
 */
export interface GetResult<V> {
  /** Whether the key was found */
  found: boolean;
  /** The value if found */
  value?: V;
}

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
   * Gets the value for the given key.
   * Returns undefined if the key is not found.
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
   * Sets the value for the given key.
   */
  async set(key: K, value: V, options?: WriteOptions): Promise<void> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = this.serialize(value);
    const ttlMs = this.resolveTtl(options);

    await this.cluster.impl.set(mappedKey, serialized, ttlMs, source);
  }

  /**
   * Sets the value only if the key does not already exist.
   * @throws {CacheKeyExists} If the key already exists.
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
   * Sets the value only if the key already exists.
   * @throws {CacheMiss} If the key does not exist.
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
   * Gets the current value and sets a new value atomically.
   * Returns undefined if the key did not exist before.
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
   * Gets the current value and deletes the key atomically.
   * Returns undefined if the key did not exist.
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

  /**
   * Gets multiple values in a single batch operation.
   * @returns An array of results, one for each key in the same order.
   */
  async getMulti(...keys: K[]): Promise<GetResult<V>[]> {
    const source = getCurrentRequest();
    const mappedKeys = keys.map((k) => this.mapKey(k));

    const results = await this.cluster.impl.mget(mappedKeys, source);

    return results.map((r) => {
      if (r === null || r === undefined) {
        return { found: false };
      }
      return { found: true, value: this.deserialize(r) };
    });
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
   * Appends a string to the existing value.
   * If the key doesn't exist, creates it with the given value.
   * @returns The length of the string after appending.
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
   * Gets a substring of the stored string value.
   * @param start - Start index (inclusive, 0-based)
   * @param end - End index (inclusive, 0-based, use -1 for end of string)
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
   * Overwrites part of the string starting at the specified offset.
   * @returns The length of the string after the operation.
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
   * Gets the length of the stored string value.
   */
  async strlen(key: K): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const result = await this.cluster.impl.strlen(mappedKey, source);
    return Number(result);
  }
}

/**
 * IntKeyspace stores 64-bit integer values.
 * Values are floored to integers using `Math.floor`.
 * For fractional values, use `FloatKeyspace` instead.
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
   * Increments the value by the given delta.
   * If the key doesn't exist, initializes it to delta.
   * @returns The new value after incrementing.
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
   * Decrements the value by the given delta.
   * If the key doesn't exist, initializes it to -delta.
   * @returns The new value after decrementing.
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
   * Increments the value by the given delta.
   * If the key doesn't exist, initializes it to delta.
   * @returns The new value after incrementing.
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
   * Decrements the value by the given delta.
   * If the key doesn't exist, initializes it to -delta.
   * @returns The new value after decrementing.
   */
  async decrement(
    key: K,
    delta: number = 1,
    options?: WriteOptions
  ): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const ttlMs = this.resolveTtl(options);
    return await this.cluster.impl.decrByFloat(mappedKey, delta, ttlMs, source);
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
