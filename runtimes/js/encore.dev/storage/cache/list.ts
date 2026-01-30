import { getCurrentRequest } from "../../internal/reqtrack/mod";
import { CacheCluster } from "./cluster";
import { CacheMiss } from "./errors";
import { resolveExpiry } from "./expiry";
import { KeyspaceConfig, WriteOptions } from "./keyspace";

/**
 * Position in a list (left/head or right/tail).
 */
export type ListPosition = "left" | "right";

/**
 * ListKeyspace stores lists of values.
 *
 * @example
 * ```ts
 * const recentViews = new ListKeyspace<string, string>(cluster, {
 *   keyPattern: "recent-views/:userId",
 *   defaultExpiry: ExpireIn(86400000), // 24 hours
 * });
 *
 * await recentViews.pushLeft("user1", "product-123", "product-456");
 * const views = await recentViews.items("user1");
 * ```
 */
export class ListKeyspace<K, V> {
  protected readonly cluster: CacheCluster;
  protected readonly config: KeyspaceConfig<K>;
  protected readonly keyMapper: (key: K) => string;

  constructor(cluster: CacheCluster, config: KeyspaceConfig<K>) {
    this.cluster = cluster;
    this.config = config;
    this.keyMapper = config.__keyMapper ?? this.createRuntimeKeyMapper(config.keyPattern);
  }

  private createRuntimeKeyMapper(pattern: string): (key: K) => string {
    const segments = pattern.split("/").map((seg) => {
      if (seg.startsWith(":")) {
        return { isLiteral: false, field: seg.slice(1) };
      }
      return { isLiteral: true, value: seg };
    });

    return (key: K) => {
      return segments
        .map((seg) => {
          if ("value" in seg) return seg.value;

          let val: unknown;
          if (typeof key === "object" && key !== null) {
            val = (key as Record<string, unknown>)[seg.field];
          } else {
            val = key;
          }

          const str = String(val);
          return str.replace(/\//g, "\\/");
        })
        .join("/");
    };
  }

  protected mapKey(key: K): string {
    return this.keyMapper(key);
  }

  protected serialize(value: V): Buffer {
    return Buffer.from(JSON.stringify(value), "utf-8");
  }

  protected deserialize(data: Buffer): V {
    return JSON.parse(data.toString("utf-8")) as V;
  }

  /**
   * Resolves the TTL in milliseconds for a write operation.
   */
  protected getTtlMs(options?: WriteOptions): number | undefined {
    const expiry = options?.expiry ?? this.config.defaultExpiry;
    if (!expiry) return undefined;

    const resolved = resolveExpiry(expiry);
    if (resolved === null) return undefined;
    return resolved;
  }

  /**
   * Returns a new keyspace wrapper with the specified write options.
   * This allows setting expiry for a chain of operations.
   *
   * @example
   * ```ts
   * await myList.with({ expiry: ExpireIn(5000) }).pushLeft(key, value);
   * ```
   */
  with(options: WriteOptions): ListKeyspace<K, V> {
    const wrapper = Object.create(this) as ListKeyspace<K, V>;
    const originalConfig = this.config;

    Object.defineProperty(wrapper, "config", {
      get() {
        return {
          ...originalConfig,
          defaultExpiry: options.expiry ?? originalConfig.defaultExpiry,
        };
      },
    });

    return wrapper;
  }

  /**
   * Pushes one or more values to the left (head) of the list.
   * @returns The length of the list after the operation.
   */
  async pushLeft(key: K, ...values: V[]): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = values.map((v) => this.serialize(v));
    const result = await this.cluster.impl.lpush(mappedKey, serialized, source);
    return Number(result);
  }

  /**
   * Pushes one or more values to the right (tail) of the list.
   * @returns The length of the list after the operation.
   */
  async pushRight(key: K, ...values: V[]): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = values.map((v) => this.serialize(v));
    const result = await this.cluster.impl.rpush(mappedKey, serialized, source);
    return Number(result);
  }

  /**
   * Pops one or more values from the left (head) of the list.
   * @param count - Number of elements to pop (default: 1)
   * @returns Array of popped values (may be empty if list is empty).
   */
  async popLeft(key: K, count: number = 1): Promise<V[]> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const results = await this.cluster.impl.lpop(mappedKey, count, source);
    return results.map((r) => this.deserialize(r));
  }

  /**
   * Pops one or more values from the right (tail) of the list.
   * @param count - Number of elements to pop (default: 1)
   * @returns Array of popped values (may be empty if list is empty).
   */
  async popRight(key: K, count: number = 1): Promise<V[]> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const results = await this.cluster.impl.rpop(mappedKey, count, source);
    return results.map((r) => this.deserialize(r));
  }

  /**
   * Gets the element at the specified index.
   * Negative indices count from the end (-1 is the last element).
   * @throws {CacheMiss} If the index is out of range or the key doesn't exist.
   */
  async getIndex(key: K, index: number): Promise<V> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const result = await this.cluster.impl.lindex(mappedKey, index, source);

    if (result === null) {
      throw new CacheMiss(mappedKey);
    }

    return this.deserialize(result);
  }

  /**
   * Sets the element at the specified index.
   * Negative indices count from the end (-1 is the last element).
   * @throws Error if index is out of range.
   */
  async setIndex(key: K, index: number, value: V): Promise<void> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = this.serialize(value);
    await this.cluster.impl.lset(mappedKey, index, serialized, source);
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
    const results = await this.cluster.impl.lrange(mappedKey, start, stop, source);
    return results.map((r) => this.deserialize(r));
  }

  /**
   * Gets all elements in the list.
   */
  async items(key: K): Promise<V[]> {
    return this.getRange(key, 0, -1);
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
  async trim(key: K, start: number, stop: number): Promise<void> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    await this.cluster.impl.ltrim(mappedKey, start, stop, source);
  }

  /**
   * Inserts a value before the first occurrence of the pivot element.
   * @returns The length of the list after the operation, or -1 if pivot not found.
   * @throws {CacheMiss} If the pivot is not found in the list.
   */
  async insertBefore(key: K, pivot: V, value: V): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const pivotSerialized = this.serialize(pivot);
    const valueSerialized = this.serialize(value);
    const result = await this.cluster.impl.linsertBefore(
      mappedKey,
      pivotSerialized,
      valueSerialized,
      source
    );
    if (result === -1) {
      throw new CacheMiss(mappedKey);
    }
    return Number(result);
  }

  /**
   * Inserts a value after the first occurrence of the pivot element.
   * @returns The length of the list after the operation.
   * @throws {CacheMiss} If the pivot is not found in the list.
   */
  async insertAfter(key: K, pivot: V, value: V): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const pivotSerialized = this.serialize(pivot);
    const valueSerialized = this.serialize(value);
    const result = await this.cluster.impl.linsertAfter(
      mappedKey,
      pivotSerialized,
      valueSerialized,
      source
    );
    if (result === -1) {
      throw new CacheMiss(mappedKey);
    }
    return Number(result);
  }

  /**
   * Removes all occurrences of a value from the list.
   * @returns The number of elements removed.
   */
  async removeAll(key: K, value: V): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const valueSerialized = this.serialize(value);
    const result = await this.cluster.impl.lrem(mappedKey, 0, valueSerialized, source);
    return Number(result);
  }

  /**
   * Removes the first N occurrences of a value from the list.
   * @param count - Maximum number of occurrences to remove.
   * @returns The number of elements removed.
   */
  async removeFirst(key: K, count: number, value: V): Promise<number> {
    if (count < 0) {
      throw new Error("count must be non-negative");
    }
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const valueSerialized = this.serialize(value);
    const result = await this.cluster.impl.lrem(mappedKey, count, valueSerialized, source);
    return Number(result);
  }

  /**
   * Removes the last N occurrences of a value from the list.
   * @param count - Maximum number of occurrences to remove.
   * @returns The number of elements removed.
   */
  async removeLast(key: K, count: number, value: V): Promise<number> {
    if (count < 0) {
      throw new Error("count must be non-negative");
    }
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const valueSerialized = this.serialize(value);
    // Negative count means remove from tail to head
    const result = await this.cluster.impl.lrem(mappedKey, -count, valueSerialized, source);
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
  async move(src: K, dst: K, srcPos: ListPosition, dstPos: ListPosition): Promise<V | undefined> {
    const source = getCurrentRequest();
    const srcKey = this.mapKey(src);
    const dstKey = this.mapKey(dst);
    const result = await this.cluster.impl.lmove(srcKey, dstKey, srcPos, dstPos, source);
    if (result === null || result === undefined) {
      return undefined;
    }
    return this.deserialize(result);
  }

  /**
   * Deletes one or more keys.
   * @returns The number of keys that were deleted.
   */
  async delete(...keys: K[]): Promise<number> {
    const source = getCurrentRequest();
    const mappedKeys = keys.map((k) => this.mapKey(k));
    return await this.cluster.impl.delete(mappedKeys, source);
  }
}
