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
   * Adds one or more members to the set.
   * @returns The number of members that were added (not already present).
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
   * Removes one or more members from the set.
   * @returns The number of members that were removed.
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
   * Checks if a member exists in the set.
   */
  async contains(key: K, member: V): Promise<boolean> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = this.serializeItem(member);
    return await this.cluster.impl.sismember(mappedKey, serialized, source);
  }

  /**
   * Gets all members of the set.
   */
  async members(key: K): Promise<V[]> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const results = await this.cluster.impl.smembers(mappedKey, source);
    return results.map((r) => this.deserializeItem(r));
  }

  /**
   * Gets all members of the set as a Set object.
   */
  async membersSet(key: K): Promise<Set<V>> {
    const members = await this.members(key);
    return new Set(members);
  }

  /**
   * Gets the number of members in the set.
   */
  async len(key: K): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const result = await this.cluster.impl.scard(mappedKey, source);
    return Number(result);
  }

  /**
   * Removes and returns a random member from the set.
   * @returns The removed member, or undefined if the set is empty.
   */
  async pop(key: K, options?: WriteOptions): Promise<V | undefined> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const ttlMs = this.resolveTtl(options);
    const result = await this.cluster.impl.spopOne(mappedKey, ttlMs, source);
    if (result === null || result === undefined) {
      return undefined;
    }
    return this.deserializeItem(result);
  }

  /**
   * Removes and returns multiple random members from the set.
   * @param count - Number of members to pop.
   * @returns Array of removed members (may be fewer than count if set is small).
   */
  async popMany(key: K, count: number, options?: WriteOptions): Promise<V[]> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const ttlMs = this.resolveTtl(options);
    const results = await this.cluster.impl.spop(
      mappedKey,
      count,
      ttlMs,
      source
    );
    return results.map((r) => this.deserializeItem(r));
  }

  /**
   * Returns a random member from the set without removing it.
   * @returns A random member, or undefined if the set is empty.
   */
  async sample(key: K): Promise<V | undefined> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const result = await this.cluster.impl.srandmemberOne(mappedKey, source);
    if (result === null || result === undefined) {
      return undefined;
    }
    return this.deserializeItem(result);
  }

  /**
   * Returns multiple distinct random members from the set without removing them.
   * @param count - Number of members to return.
   * @returns Array of random members (may be fewer than count if set is small).
   */
  async sampleMany(key: K, count: number): Promise<V[]> {
    if (count < 0) {
      throw new Error("count must be non-negative");
    }
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const results = await this.cluster.impl.srandmember(
      mappedKey,
      count,
      source
    );
    return results.map((r) => this.deserializeItem(r));
  }

  /**
   * Returns multiple random members from the set, possibly with duplicates.
   * @param count - Number of members to return.
   * @returns Array of random members (may contain duplicates).
   */
  async sampleWithReplacement(key: K, count: number): Promise<V[]> {
    if (count < 0) {
      throw new Error("count must be non-negative");
    }
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    // Negative count in Redis SRANDMEMBER allows duplicates
    const results = await this.cluster.impl.srandmember(
      mappedKey,
      -count,
      source
    );
    return results.map((r) => this.deserializeItem(r));
  }

  /**
   * Computes the difference between sets (members in the first set but not in others).
   * @param keys - Keys of sets to compute difference for.
   * @returns Members that are in the first set but not in any of the other sets.
   */
  async diffSet(...keys: K[]): Promise<Set<V>> {
    const items = await this.diff(...keys);
    return new Set(items);
  }

  /**
   * Computes the intersection of sets (members common to all sets).
   * @param keys - Keys of sets to compute intersection for.
   * @returns Members that are in all of the specified sets, as a Set.
   */
  async intersectSet(...keys: K[]): Promise<Set<V>> {
    const items = await this.intersect(...keys);
    return new Set(items);
  }

  /**
   * Computes the union of sets (members in any of the sets).
   * @param keys - Keys of sets to compute union for.
   * @returns Members that are in any of the specified sets, as a Set.
   */
  async unionSet(...keys: K[]): Promise<Set<V>> {
    const items = await this.union(...keys);
    return new Set(items);
  }

  /**
   * Computes the difference between sets (members in the first set but not in others).
   * @param keys - Keys of sets to compute difference for.
   * @returns Members that are in the first set but not in any of the other sets.
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
   * Computes the difference between sets and stores the result.
   * @param destination - Key to store the result.
   * @param keys - Keys of sets to compute difference for.
   * @returns The number of elements in the resulting set.
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
   * Computes the intersection of sets (members common to all sets).
   * @param keys - Keys of sets to compute intersection for.
   * @returns Members that are in all of the specified sets.
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
   * Computes the intersection of sets and stores the result.
   * @param destination - Key to store the result.
   * @param keys - Keys of sets to compute intersection for.
   * @returns The number of elements in the resulting set.
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
   * Computes the union of sets (members in any of the sets).
   * @param keys - Keys of sets to compute union for.
   * @returns Members that are in any of the specified sets.
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
   * Computes the union of sets and stores the result.
   * @param destination - Key to store the result.
   * @param keys - Keys of sets to compute union for.
   * @returns The number of elements in the resulting set.
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
   * Moves a member from one set to another.
   * @param src - Source set key.
   * @param dst - Destination set key.
   * @param member - The member to move.
   * @returns true if the member was moved, false if not found in source.
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
 * const allTags = await tags.members("article1");
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
