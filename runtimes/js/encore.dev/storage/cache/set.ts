import { getCurrentRequest } from "../../internal/reqtrack/mod";
import { CacheCluster } from "./cluster";
import { resolveExpiry } from "./expiry";
import { KeyspaceConfig, WriteOptions } from "./keyspace";

/**
 * SetKeyspace stores sets of unique values.
 *
 * @example
 * ```ts
 * const tags = new SetKeyspace<string, string>(cluster, {
 *   keyPattern: "tags/:articleId",
 * });
 *
 * await tags.add("article1", "typescript", "programming", "web");
 * const hasTech = await tags.contains("article1", "typescript");
 * const allTags = await tags.members("article1");
 * ```
 */
export class SetKeyspace<K, V> {
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
   * await mySet.with({ expiry: ExpireIn(5000) }).add(key, "value");
   * ```
   */
  with(options: WriteOptions): SetKeyspace<K, V> {
    const wrapper = Object.create(this) as SetKeyspace<K, V>;
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
   * Adds one or more members to the set.
   * @returns The number of members that were added (not already present).
   */
  async add(key: K, ...members: V[]): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = members.map((m) => this.serialize(m));
    const result = await this.cluster.impl.sadd(mappedKey, serialized, source);
    return Number(result);
  }

  /**
   * Removes one or more members from the set.
   * @returns The number of members that were removed.
   */
  async remove(key: K, ...members: V[]): Promise<number> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = members.map((m) => this.serialize(m));
    const result = await this.cluster.impl.srem(mappedKey, serialized, source);
    return Number(result);
  }

  /**
   * Checks if a member exists in the set.
   */
  async contains(key: K, member: V): Promise<boolean> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = this.serialize(member);
    return await this.cluster.impl.sismember(mappedKey, serialized, source);
  }

  /**
   * Gets all members of the set.
   */
  async members(key: K): Promise<V[]> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const results = await this.cluster.impl.smembers(mappedKey, source);
    return results.map((r) => this.deserialize(r));
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
  async pop(key: K): Promise<V | undefined> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const results = await this.cluster.impl.spop(mappedKey, 1, source);
    if (results.length === 0) {
      return undefined;
    }
    return this.deserialize(results[0]);
  }

  /**
   * Removes and returns multiple random members from the set.
   * @param count - Number of members to pop.
   * @returns Array of removed members (may be fewer than count if set is small).
   */
  async popMany(key: K, count: number): Promise<V[]> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const results = await this.cluster.impl.spop(mappedKey, count, source);
    return results.map((r) => this.deserialize(r));
  }

  /**
   * Returns a random member from the set without removing it.
   * @returns A random member, or undefined if the set is empty.
   */
  async sample(key: K): Promise<V | undefined> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const results = await this.cluster.impl.srandmember(mappedKey, 1, source);
    if (results.length === 0) {
      return undefined;
    }
    return this.deserialize(results[0]);
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
    const results = await this.cluster.impl.srandmember(mappedKey, count, source);
    return results.map((r) => this.deserialize(r));
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
    const results = await this.cluster.impl.srandmember(mappedKey, -count, source);
    return results.map((r) => this.deserialize(r));
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
    return results.map((r) => this.deserialize(r));
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
    const result = await this.cluster.impl.sdiffstore(destKey, mappedKeys, source);
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
    return results.map((r) => this.deserialize(r));
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
    const result = await this.cluster.impl.sinterstore(destKey, mappedKeys, source);
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
    return results.map((r) => this.deserialize(r));
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
    const result = await this.cluster.impl.sunionstore(destKey, mappedKeys, source);
    return Number(result);
  }

  /**
   * Moves a member from one set to another.
   * @param src - Source set key.
   * @param dst - Destination set key.
   * @param member - The member to move.
   * @returns true if the member was moved, false if not found in source.
   */
  async move(src: K, dst: K, member: V): Promise<boolean> {
    const source = getCurrentRequest();
    const srcKey = this.mapKey(src);
    const dstKey = this.mapKey(dst);
    const serialized = this.serialize(member);
    return await this.cluster.impl.smove(srcKey, dstKey, serialized, source);
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
