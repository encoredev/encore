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
   * Pushes one or more values to the left (head) of the list.
   * @returns The length of the list after the operation.
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
   * Pushes one or more values to the right (tail) of the list.
   * @returns The length of the list after the operation.
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
   * Pops one value from the left (head) of the list.
   * @returns The popped value, or undefined if the list is empty.
   */
  async popLeft(key: K, options?: WriteOptions): Promise<V | undefined> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const ttlMs = this.resolveTtl(options);
    const results = await this.cluster.impl.lpop(mappedKey, 1, ttlMs, source);
    if (results.length === 0) {
      return undefined;
    }
    return this.deserializeItem(results[0]);
  }

  /**
   * Pops one value from the right (tail) of the list.
   * @returns The popped value, or undefined if the list is empty.
   */
  async popRight(key: K, options?: WriteOptions): Promise<V | undefined> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const ttlMs = this.resolveTtl(options);
    const results = await this.cluster.impl.rpop(mappedKey, 1, ttlMs, source);
    if (results.length === 0) {
      return undefined;
    }
    return this.deserializeItem(results[0]);
  }

  /**
   * Gets the element at the specified index.
   * Negative indices count from the end (-1 is the last element).
   * @returns The value at the index, or undefined if out of range or key doesn't exist.
   */
  async getIndex(key: K, index: number): Promise<V | undefined> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const result = await this.cluster.impl.lindex(mappedKey, index, source);

    if (result === null) {
      return undefined;
    }

    return this.deserializeItem(result);
  }

  /**
   * Sets the element at the specified index.
   * Negative indices count from the end (-1 is the last element).
   * @throws Error if index is out of range.
   */
  async setIndex(
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
   * Gets a range of elements from the list.
   * Negative indices count from the end (-1 is the last element).
   * @param start - Start index (inclusive)
   * @param stop - Stop index (inclusive)
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
   * Gets all elements in the list.
   */
  async items(key: K): Promise<V[]> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const results = await this.cluster.impl.litems(mappedKey, source);
    return results.map((r) => this.deserializeItem(r));
  }

  /**
   * Gets the length of the list.
   */
  async len(key: K): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const result = await this.cluster.impl.llen(mappedKey, source);
    return Number(result);
  }

  /**
   * Trims the list to the specified range.
   * Elements outside the range are removed.
   * Negative indices count from the end (-1 is the last element).
   * @param start - Start index (inclusive)
   * @param stop - Stop index (inclusive)
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
   * Inserts a value before the first occurrence of the pivot element.
   * @returns The length of the list after the operation, or -1 if pivot not found.
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
   * Inserts a value after the first occurrence of the pivot element.
   * @returns The length of the list after the operation, or -1 if pivot not found.
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
   * Removes all occurrences of a value from the list.
   * @returns The number of elements removed.
   */
  async removeAll(key: K, value: V, options?: WriteOptions): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const valueSerialized = this.serializeItem(value);
    const ttlMs = this.resolveTtl(options);
    const result = await this.cluster.impl.lrem(
      mappedKey,
      0,
      valueSerialized,
      ttlMs,
      source
    );
    return Number(result);
  }

  /**
   * Removes the first N occurrences of a value from the list.
   * @param count - Maximum number of occurrences to remove.
   * @returns The number of elements removed.
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
    const result = await this.cluster.impl.lrem(
      mappedKey,
      count,
      valueSerialized,
      ttlMs,
      source
    );
    return Number(result);
  }

  /**
   * Removes the last N occurrences of a value from the list.
   * @param count - Maximum number of occurrences to remove.
   * @returns The number of elements removed.
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
    const result = await this.cluster.impl.lrem(
      mappedKey,
      -count,
      valueSerialized,
      ttlMs,
      source
    );
    return Number(result);
  }

  /**
   * Atomically moves an element from one list to another.
   * @param src - Source key
   * @param dst - Destination key
   * @param srcPos - Position to pop from in source list
   * @param dstPos - Position to push to in destination list
   * @returns The moved element, or undefined if source list is empty.
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
