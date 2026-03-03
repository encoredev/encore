import { getCurrentRequest } from "../../internal/reqtrack/mod";
import { CacheCluster } from "./cluster";
import { Keyspace, KeyspaceConfig, WriteOptions } from "./keyspace";

/**
 * Base class for set keyspaces with all set operations.
 * Subclasses provide typed serialization/deserialization.
 * @internal
 */
abstract class SetKeyspace<K, V> extends Keyspace<K> {
  constructor(cluster: CacheCluster, config: KeyspaceConfig<K>) {
    super(cluster, config);
  }

  protected abstract serializeItem(value: V): Buffer;
  protected abstract deserializeItem(data: Buffer): V;

  /**
   * Adds one or more values to the set stored at key.
   * If the key does not already exist, it is first created as an empty set.
   *
   * @returns The number of values that were added to the set,
   * not including values already present beforehand.
   * @see https://redis.io/commands/sadd/
   */
  async add(key: K, ...members: V[]): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = members.map((m) => this.serializeItem(m));
    const ttlMs = this.resolveTtl();
    const result = await this.cluster.impl.sadd(
      mappedKey,
      serialized,
      ttlMs,
      source
    );
    return Number(result);
  }

  /**
   * Removes one or more values from the set stored at key.
   * Values not present in the set are ignored.
   * If the key does not already exist, it is a no-op.
   *
   * @returns The number of values that were removed from the set.
   * @see https://redis.io/commands/srem/
   */
  async remove(key: K, ...members: V[]): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = members.map((m) => this.serializeItem(m));
    const ttlMs = this.resolveTtl();
    const result = await this.cluster.impl.srem(
      mappedKey,
      serialized,
      ttlMs,
      source
    );
    return Number(result);
  }

  /**
   * Removes a random element from the set stored at key and returns it.
   *
   * @returns The removed member, or `undefined` if the set is empty.
   * @see https://redis.io/commands/spop/
   */
  async popOne(key: K, options?: WriteOptions): Promise<V | undefined> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const ttlMs = this.resolveTtl(options);
    const result = await this.cluster.impl.spop(mappedKey, ttlMs, source);
    if (result === null || result === undefined) {
      return undefined;
    }
    return this.deserializeItem(result);
  }

  /**
   * Removes up to `count` random elements (bounded by the set's size)
   * from the set stored at key and returns them.
   *
   * If the set is empty it returns an empty array.
   *
   * @param key - The cache key.
   * @param count - Number of members to pop.
   * @returns The removed members (may be fewer than `count` if the set is small).
   * @see https://redis.io/commands/spop/
   */
  async pop(key: K, count: number, options?: WriteOptions): Promise<V[]> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const ttlMs = this.resolveTtl(options);
    const results = await this.cluster.impl.spopN(
      mappedKey,
      count,
      ttlMs,
      source
    );
    return results.map((r) => this.deserializeItem(r));
  }

  /**
   * Reports whether the set stored at key contains the given value.
   *
   * If the key does not exist it returns `false`.
   *
   * @returns `true` if the member exists in the set, `false` otherwise.
   * @see https://redis.io/commands/sismember/
   */
  async contains(key: K, member: V): Promise<boolean> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = this.serializeItem(member);
    return await this.cluster.impl.sismember(mappedKey, serialized, source);
  }

  /**
   * Returns the number of elements in the set stored at key.
   *
   * If the key does not exist it returns 0.
   *
   * @returns The set cardinality.
   * @see https://redis.io/commands/scard/
   */
  async len(key: K): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const result = await this.cluster.impl.scard(mappedKey, source);
    return Number(result);
  }

  /**
   * Returns the elements in the set stored at key.
   *
   * If the key does not exist it returns an empty array.
   *
   * @returns All members of the set.
   * @see https://redis.io/commands/smembers/
   */
  async items(key: K): Promise<V[]> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const results = await this.cluster.impl.smembers(mappedKey, source);
    return results.map((r) => this.deserializeItem(r));
  }

  /**
   * Identical to {@link items} except it returns the values as a `Set`.
   *
   * If the key does not exist it returns an empty `Set`.
   *
   * @returns All members of the set as a `Set`.
   * @see https://redis.io/commands/smembers/
   */
  async itemsSet(key: K): Promise<Set<V>> {
    const members = await this.items(key);
    return new Set(members);
  }

  /**
   * Computes the set difference between the first set and all the consecutive sets.
   *
   * Set difference means the values present in the first set that are not present
   * in any of the other sets.
   *
   * Keys that do not exist are considered as empty sets.
   *
   * @param keys - Keys of sets to compute difference for. At least one must be provided.
   * @returns Members in the first set but not in any of the other sets.
   * @throws {Error} If no keys are provided.
   * @see https://redis.io/commands/sdiff/
   */
  async diff(...keys: K[]): Promise<V[]> {
    if (keys.length === 0) {
      throw new Error("at least one key must be provided");
    }
    const source = getCurrentRequest();
    const mappedKeys = keys.map((k) => this.mapKey(k));
    const results = await this.cluster.impl.sdiff(mappedKeys, source);
    return results.map((r) => this.deserializeItem(r));
  }

  /**
   * Identical to {@link diff} except it returns the values as a `Set`.
   *
   * @see https://redis.io/commands/sdiff/
   */
  async diffSet(...keys: K[]): Promise<Set<V>> {
    const items = await this.diff(...keys);
    return new Set(items);
  }

  /**
   * Computes the set difference between keys (like {@link diff}) and stores the result
   * in `destination`.
   *
   * @param destination - Key to store the result.
   * @param keys - Keys of sets to compute difference for.
   * @returns The size of the resulting set.
   * @see https://redis.io/commands/sdiffstore/
   */
  async diffStore(destination: K, ...keys: K[]): Promise<number> {
    if (keys.length === 0) {
      throw new Error("at least one key must be provided");
    }
    const source = getCurrentRequest();
    const destKey = this.mapKey(destination);
    const mappedKeys = keys.map((k) => this.mapKey(k));
    const ttlMs = this.resolveTtl();
    const result = await this.cluster.impl.sdiffstore(
      destKey,
      mappedKeys,
      ttlMs,
      source
    );
    return Number(result);
  }

  /**
   * Computes the set intersection between the sets stored at the given keys.
   *
   * Set intersection means the values common to all the provided sets.
   *
   * Keys that do not exist are considered to be empty sets.
   * As a result, if any key is missing the final result is the empty set.
   *
   * @param keys - Keys of sets to compute intersection for. At least one must be provided.
   * @returns Members common to all sets.
   * @throws {Error} If no keys are provided.
   * @see https://redis.io/commands/sinter/
   */
  async intersect(...keys: K[]): Promise<V[]> {
    if (keys.length === 0) {
      throw new Error("at least one key must be provided");
    }
    const source = getCurrentRequest();
    const mappedKeys = keys.map((k) => this.mapKey(k));
    const results = await this.cluster.impl.sinter(mappedKeys, source);
    return results.map((r) => this.deserializeItem(r));
  }

  /**
   * Identical to {@link intersect} except it returns the values as a `Set`.
   *
   * @see https://redis.io/commands/sinter/
   */
  async intersectSet(...keys: K[]): Promise<Set<V>> {
    const items = await this.intersect(...keys);
    return new Set(items);
  }

  /**
   * Computes the set intersection between keys (like {@link intersect}) and stores the result
   * in `destination`.
   *
   * @param destination - Key to store the result.
   * @param keys - Keys of sets to compute intersection for.
   * @returns The size of the resulting set.
   * @see https://redis.io/commands/sinterstore/
   */
  async intersectStore(destination: K, ...keys: K[]): Promise<number> {
    if (keys.length === 0) {
      throw new Error("at least one key must be provided");
    }
    const source = getCurrentRequest();
    const destKey = this.mapKey(destination);
    const mappedKeys = keys.map((k) => this.mapKey(k));
    const ttlMs = this.resolveTtl();
    const result = await this.cluster.impl.sinterstore(
      destKey,
      mappedKeys,
      ttlMs,
      source
    );
    return Number(result);
  }

  /**
   * Computes the set union between the sets stored at the given keys.
   *
   * Set union means the values present in at least one of the provided sets.
   *
   * Keys that do not exist are considered to be empty sets.
   *
   * @param keys - Keys of sets to compute union for. At least one must be provided.
   * @returns Members in any of the provided sets.
   * @throws {Error} If no keys are provided.
   * @see https://redis.io/commands/sunion/
   */
  async union(...keys: K[]): Promise<V[]> {
    if (keys.length === 0) {
      throw new Error("at least one key must be provided");
    }
    const source = getCurrentRequest();
    const mappedKeys = keys.map((k) => this.mapKey(k));
    const results = await this.cluster.impl.sunion(mappedKeys, source);
    return results.map((r) => this.deserializeItem(r));
  }

  /**
   * Identical to {@link union} except it returns the values as a `Set`.
   *
   * @see https://redis.io/commands/sunion/
   */
  async unionSet(...keys: K[]): Promise<Set<V>> {
    const items = await this.union(...keys);
    return new Set(items);
  }

  /**
   * Computes the set union between sets (like {@link union}) and stores the result
   * in `destination`.
   *
   * @param destination - Key to store the result.
   * @param keys - Keys of sets to compute union for.
   * @returns The size of the resulting set.
   * @see https://redis.io/commands/sunionstore/
   */
  async unionStore(destination: K, ...keys: K[]): Promise<number> {
    if (keys.length === 0) {
      throw new Error("at least one key must be provided");
    }
    const source = getCurrentRequest();
    const destKey = this.mapKey(destination);
    const mappedKeys = keys.map((k) => this.mapKey(k));
    const ttlMs = this.resolveTtl();
    const result = await this.cluster.impl.sunionstore(
      destKey,
      mappedKeys,
      ttlMs,
      source
    );
    return Number(result);
  }

  /**
   * Returns a random member from the set stored at key without removing it.
   *
   * @returns A random member, or `undefined` if the key does not exist.
   * @see https://redis.io/commands/srandmember/
   */
  async sampleOne(key: K): Promise<V | undefined> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const result = await this.cluster.impl.srandmember(mappedKey, source);
    if (result === null || result === undefined) {
      return undefined;
    }
    return this.deserializeItem(result);
  }

  /**
   * Returns up to `count` distinct random elements from the set stored at key.
   * The same element is never returned multiple times.
   *
   * If the key does not exist it returns an empty array.
   *
   * @param key - The cache key.
   * @param count - Number of distinct members to return.
   * @returns Random members (may be fewer than `count` if the set is small).
   * @see https://redis.io/commands/srandmember/
   */
  async sample(key: K, count: number): Promise<V[]> {
    if (count < 0) {
      throw new Error("count must be non-negative");
    }
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const results = await this.cluster.impl.srandmemberN(
      mappedKey,
      count,
      source
    );
    return results.map((r) => this.deserializeItem(r));
  }

  /**
   * Returns `count` random elements from the set stored at key.
   * The same element may be returned multiple times.
   *
   * If the key does not exist it returns an empty array.
   *
   * @param key - The cache key.
   * @param count - Number of members to return (may include duplicates).
   * @returns Random members, possibly with duplicates.
   * @see https://redis.io/commands/srandmember/
   */
  async sampleWithReplacement(key: K, count: number): Promise<V[]> {
    if (count < 0) {
      throw new Error("count must be non-negative");
    }
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    // Negative count in Redis SRANDMEMBER allows duplicates
    const results = await this.cluster.impl.srandmemberN(
      mappedKey,
      -count,
      source
    );
    return results.map((r) => this.deserializeItem(r));
  }

  /**
   * Atomically moves the given member from the set stored at `src`
   * to the set stored at `dst`.
   *
   * If the element already exists in `dst` it is still removed from `src`.
   *
   * @param src - Source set key.
   * @param dst - Destination set key.
   * @param member - The member to move.
   * @returns `true` if the member was moved, `false` if not found in `src`.
   * @see https://redis.io/commands/smove/
   */
  async move(
    src: K,
    dst: K,
    member: V,
    options?: WriteOptions
  ): Promise<boolean> {
    const source = getCurrentRequest();
    const srcKey = this.mapKey(src);
    const dstKey = this.mapKey(dst);
    const serialized = this.serializeItem(member);
    const ttlMs = this.resolveTtl(options);
    return await this.cluster.impl.smove(
      srcKey,
      dstKey,
      serialized,
      ttlMs,
      source
    );
  }
}

/**
 * StringSetKeyspace stores sets of unique string values.
 *
 * @example
 * ```ts
 * const tags = new StringSetKeyspace<string>(cluster, {
 *   keyPattern: "tags/:articleId",
 * });
 *
 * await tags.add("article1", "typescript", "programming", "web");
 * const hasTech = await tags.contains("article1", "typescript");
 * const allTags = await tags.items("article1");
 * const tagSet = await tags.itemsSet("article1");
 * ```
 */
export class StringSetKeyspace<K> extends SetKeyspace<K, string> {
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
 * NumberSetKeyspace stores sets of unique numeric values.
 *
 * @example
 * ```ts
 * const scores = new NumberSetKeyspace<string>(cluster, {
 *   keyPattern: "unique-scores/:gameId",
 * });
 *
 * await scores.add("game1", 100, 200, 300);
 * const hasScore = await scores.contains("game1", 100);
 * ```
 */
export class NumberSetKeyspace<K> extends SetKeyspace<K, number> {
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
