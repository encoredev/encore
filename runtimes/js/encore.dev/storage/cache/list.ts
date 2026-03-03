import { getCurrentRequest } from "../../internal/reqtrack/mod";
import { CacheCluster } from "./cluster";
import { Keyspace, KeyspaceConfig, WriteOptions } from "./keyspace";

/**
 * Position in a list (left/head or right/tail).
 */
export type ListPosition = "left" | "right";

/**
 * Base class for list keyspaces with all list operations.
 * Subclasses provide typed serialization/deserialization.
 * @internal
 */
abstract class ListKeyspace<K, V> extends Keyspace<K> {
  constructor(cluster: CacheCluster, config: KeyspaceConfig<K>) {
    super(cluster, config);
  }

  protected abstract serializeItem(value: V): Buffer;
  protected abstract deserializeItem(data: Buffer): V;

  /**
   * Pushes one or more values at the head of the list stored at key.
   * If the key does not already exist, it is first created as an empty list.
   *
   * If multiple values are given, they are inserted one after another,
   * starting with the leftmost value. For instance,
   * `pushLeft(key, "a", "b", "c")` will result in a list containing
   * "c" as its first element, "b" as its second, and "a" as its third.
   *
   * @returns The length of the list after the operation.
   * @see https://redis.io/commands/lpush/
   */
  async pushLeft(key: K, ...values: V[]): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = values.map((v) => this.serializeItem(v));
    const ttlMs = this.resolveTtl();
    const result = await this.cluster.impl.lpush(
      mappedKey,
      serialized,
      ttlMs,
      source
    );
    return Number(result);
  }

  /**
   * Pushes one or more values at the tail of the list stored at key.
   * If the key does not already exist, it is first created as an empty list.
   *
   * If multiple values are given, they are inserted one after another,
   * starting with the leftmost value. For instance,
   * `pushRight(key, "a", "b", "c")` will result in a list containing
   * "a" as its first element, "b" as its second, and "c" as its third.
   *
   * @returns The length of the list after the operation.
   * @see https://redis.io/commands/rpush/
   */
  async pushRight(key: K, ...values: V[]): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = values.map((v) => this.serializeItem(v));
    const ttlMs = this.resolveTtl();
    const result = await this.cluster.impl.rpush(
      mappedKey,
      serialized,
      ttlMs,
      source
    );
    return Number(result);
  }

  /**
   * Pops a single element off the head of the list stored at key.
   *
   * @returns The popped value, or `undefined` if the key does not exist.
   * @see https://redis.io/commands/lpop/
   */
  async popLeft(key: K, options?: WriteOptions): Promise<V | undefined> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const ttlMs = this.resolveTtl(options);
    const result = await this.cluster.impl.lpop(mappedKey, ttlMs, source);
    if (result === null) {
      return undefined;
    }
    return this.deserializeItem(result);
  }

  /**
   * Pops a single element off the tail of the list stored at key.
   *
   * @returns The popped value, or `undefined` if the key does not exist.
   * @see https://redis.io/commands/rpop/
   */
  async popRight(key: K, options?: WriteOptions): Promise<V | undefined> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const ttlMs = this.resolveTtl(options);
    const result = await this.cluster.impl.rpop(mappedKey, ttlMs, source);
    if (result === null) {
      return undefined;
    }
    return this.deserializeItem(result);
  }

  /**
   * Returns the length of the list stored at key.
   *
   * Non-existing keys are considered as empty lists.
   *
   * @returns The list length.
   * @see https://redis.io/commands/llen/
   */
  async len(key: K): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const result = await this.cluster.impl.llen(mappedKey, source);
    return Number(result);
  }

  /**
   * Trims the list stored at key to only contain the elements between the indices
   * `start` and `stop` (inclusive). Both are zero-based indices.
   *
   * Negative indices can be used to indicate offsets from the end of the list,
   * where -1 is the last element of the list, -2 the penultimate element, and so on.
   *
   * Out of range indices are valid and are treated as if they specify the start or end of the list,
   * respectively. If `start` > `stop` the end result is an empty list.
   *
   * @param key - The cache key.
   * @param start - Start index (inclusive).
   * @param stop - Stop index (inclusive).
   * @see https://redis.io/commands/ltrim/
   */
  async trim(
    key: K,
    start: number,
    stop: number,
    options?: WriteOptions
  ): Promise<void> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const ttlMs = this.resolveTtl(options);
    await this.cluster.impl.ltrim(mappedKey, start, stop, ttlMs, source);
  }

  /**
   * Updates the list element at the given index.
   *
   * Negative indices can be used to indicate offsets from the end of the list,
   * where -1 is the last element of the list, -2 the penultimate element, and so on.
   *
   * @param key - The cache key.
   * @param index - Zero-based index of the element to update.
   * @param value - The new value.
   * @throws {Error} If the index is out of range.
   * @see https://redis.io/commands/lset/
   */
  async set(
    key: K,
    index: number,
    value: V,
    options?: WriteOptions
  ): Promise<void> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = this.serializeItem(value);
    const ttlMs = this.resolveTtl(options);
    await this.cluster.impl.lset(mappedKey, index, serialized, ttlMs, source);
  }

  /**
   * Returns the value of the list element at the given index.
   *
   * Negative indices can be used to indicate offsets from the end of the list,
   * where -1 is the last element of the list, -2 the penultimate element, and so on.
   *
   * @param key - The cache key.
   * @param index - Zero-based index of the element to retrieve.
   * @returns The value at the index, or `undefined` if out of range or the key does not exist.
   * @see https://redis.io/commands/lindex/
   */
  async get(key: K, index: number): Promise<V | undefined> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const result = await this.cluster.impl.lindex(mappedKey, index, source);

    if (result === null) {
      return undefined;
    }

    return this.deserializeItem(result);
  }

  /**
   * Returns all the elements in the list stored at key.
   *
   * If the key does not exist it returns an empty array.
   *
   * @returns All elements in the list.
   * @see https://redis.io/commands/lrange/
   */
  async items(key: K): Promise<V[]> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const results = await this.cluster.impl.lrangeAll(mappedKey, source);
    return results.map((r) => this.deserializeItem(r));
  }

  /**
   * Returns the elements in the list stored at key between `start` and `stop` (inclusive).
   * Both are zero-based indices.
   *
   * Negative indices can be used to indicate offsets from the end of the list,
   * where -1 is the last element of the list, -2 the penultimate element, and so on.
   *
   * If the key does not exist it returns an empty array.
   *
   * @param key - The cache key.
   * @param start - Start index (inclusive).
   * @param stop - Stop index (inclusive).
   * @returns The elements in the specified range.
   * @see https://redis.io/commands/lrange/
   */
  async getRange(key: K, start: number, stop: number): Promise<V[]> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const results = await this.cluster.impl.lrange(
      mappedKey,
      start,
      stop,
      source
    );
    return results.map((r) => this.deserializeItem(r));
  }

  /**
   * Inserts `value` into the list stored at key, at the position just before `pivot`.
   *
   * If the list does not contain `pivot`, the value is not inserted and -1 is returned.
   *
   * @param key - The cache key.
   * @param pivot - The existing element to insert before.
   * @param value - The value to insert.
   * @returns The new list length, or -1 if `pivot` was not found.
   * @see https://redis.io/commands/linsert/
   */
  async insertBefore(
    key: K,
    pivot: V,
    value: V,
    options?: WriteOptions
  ): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const pivotSerialized = this.serializeItem(pivot);
    const valueSerialized = this.serializeItem(value);
    const ttlMs = this.resolveTtl(options);
    const result = await this.cluster.impl.linsertBefore(
      mappedKey,
      pivotSerialized,
      valueSerialized,
      ttlMs,
      source
    );
    return Number(result);
  }

  /**
   * Inserts `value` into the list stored at key, at the position just after `pivot`.
   *
   * If the list does not contain `pivot`, the value is not inserted and -1 is returned.
   *
   * @param key - The cache key.
   * @param pivot - The existing element to insert after.
   * @param value - The value to insert.
   * @returns The new list length, or -1 if `pivot` was not found.
   * @see https://redis.io/commands/linsert/
   */
  async insertAfter(
    key: K,
    pivot: V,
    value: V,
    options?: WriteOptions
  ): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const pivotSerialized = this.serializeItem(pivot);
    const valueSerialized = this.serializeItem(value);
    const ttlMs = this.resolveTtl(options);
    const result = await this.cluster.impl.linsertAfter(
      mappedKey,
      pivotSerialized,
      valueSerialized,
      ttlMs,
      source
    );
    return Number(result);
  }

  /**
   * Removes all occurrences of `value` in the list stored at key.
   *
   * If the list does not contain `value`, or the list does not exist, returns 0.
   *
   * @param key - The cache key.
   * @param value - The value to remove.
   * @returns The number of elements removed.
   * @see https://redis.io/commands/lrem/
   */
  async removeAll(key: K, value: V, options?: WriteOptions): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const valueSerialized = this.serializeItem(value);
    const ttlMs = this.resolveTtl(options);
    const result = await this.cluster.impl.lremAll(
      mappedKey,
      valueSerialized,
      ttlMs,
      source
    );
    return Number(result);
  }

  /**
   * Removes the first `count` occurrences of `value` in the list stored at key,
   * scanning from head to tail.
   *
   * If the list does not contain `value`, or the list does not exist, returns 0.
   *
   * @param key - The cache key.
   * @param count - Maximum number of occurrences to remove.
   * @param value - The value to remove.
   * @returns The number of elements removed.
   * @see https://redis.io/commands/lrem/
   */
  async removeFirst(
    key: K,
    count: number,
    value: V,
    options?: WriteOptions
  ): Promise<number> {
    if (count < 0) {
      throw new Error("count must be non-negative");
    }
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const valueSerialized = this.serializeItem(value);
    const ttlMs = this.resolveTtl(options);
    const result = await this.cluster.impl.lremFirst(
      mappedKey,
      count,
      valueSerialized,
      ttlMs,
      source
    );
    return Number(result);
  }

  /**
   * Removes the last `count` occurrences of `value` in the list stored at key,
   * scanning from tail to head.
   *
   * If the list does not contain `value`, or the list does not exist, returns 0.
   *
   * @param key - The cache key.
   * @param count - Maximum number of occurrences to remove.
   * @param value - The value to remove.
   * @returns The number of elements removed.
   * @see https://redis.io/commands/lrem/
   */
  async removeLast(
    key: K,
    count: number,
    value: V,
    options?: WriteOptions
  ): Promise<number> {
    if (count < 0) {
      throw new Error("count must be non-negative");
    }
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const valueSerialized = this.serializeItem(value);
    const ttlMs = this.resolveTtl(options);
    // Negative count means remove from tail to head
    const result = await this.cluster.impl.lremLast(
      mappedKey,
      count,
      valueSerialized,
      ttlMs,
      source
    );
    return Number(result);
  }

  /**
   * Atomically moves an element from the list stored at `src` to the list stored at `dst`.
   *
   * The value moved can be either the head (`srcPos === "left"`) or tail (`srcPos === "right"`)
   * of the list at `src`. Similarly, the value can be placed either at the head (`dstPos === "left"`)
   * or tail (`dstPos === "right"`) of the list at `dst`.
   *
   * If `src` and `dst` are the same list, the value is atomically rotated from one end to the other
   * when `srcPos !== dstPos`, or if `srcPos === dstPos` nothing happens.
   *
   * @param src - Source list key.
   * @param dst - Destination list key.
   * @param srcPos - Position to pop from in the source list.
   * @param dstPos - Position to push to in the destination list.
   * @returns The moved element, or `undefined` if the source list does not exist.
   * @see https://redis.io/commands/lmove/
   */
  async move(
    src: K,
    dst: K,
    srcPos: ListPosition,
    dstPos: ListPosition,
    options?: WriteOptions
  ): Promise<V | undefined> {
    const source = getCurrentRequest();
    const srcKey = this.mapKey(src);
    const dstKey = this.mapKey(dst);
    const ttlMs = this.resolveTtl(options);
    const result = await this.cluster.impl.lmove(
      srcKey,
      dstKey,
      srcPos,
      dstPos,
      ttlMs,
      source
    );
    if (result === null || result === undefined) {
      return undefined;
    }
    return this.deserializeItem(result);
  }
}

/**
 * StringListKeyspace stores lists of string values.
 *
 * @example
 * ```ts
 * const recentViews = new StringListKeyspace<string>(cluster, {
 *   keyPattern: "recent-views/:userId",
 *   defaultExpiry: ExpireIn(86400000), // 24 hours
 * });
 *
 * await recentViews.pushLeft("user1", "product-123", "product-456");
 * const views = await recentViews.items("user1");
 * ```
 */
export class StringListKeyspace<K> extends ListKeyspace<K, string> {
  constructor(cluster: CacheCluster, config: KeyspaceConfig<K>) {
    super(cluster, config);
  }

  protected serializeItem(value: string): Buffer {
    return Buffer.from(value, "utf-8");
  }

  protected deserializeItem(data: Buffer): string {
    return data.toString("utf-8");
  }
}

/**
 * NumberListKeyspace stores lists of numeric values.
 *
 * @example
 * ```ts
 * const scores = new NumberListKeyspace<string>(cluster, {
 *   keyPattern: "scores/:gameId",
 * });
 *
 * await scores.pushRight("game1", 100, 200, 300);
 * const allScores = await scores.items("game1");
 * ```
 */
export class NumberListKeyspace<K> extends ListKeyspace<K, number> {
  constructor(cluster: CacheCluster, config: KeyspaceConfig<K>) {
    super(cluster, config);
  }

  protected serializeItem(value: number): Buffer {
    return Buffer.from(String(value), "utf-8");
  }

  protected deserializeItem(data: Buffer): number {
    return Number(data.toString("utf-8"));
  }
}
