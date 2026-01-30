import { getCurrentRequest } from "../../internal/reqtrack/mod";
import { CacheCluster } from "./cluster";
import { KeyspaceConfig } from "./keyspace";

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
   * Deletes one or more keys.
   * @returns The number of keys that were deleted.
   */
  async delete(...keys: K[]): Promise<number> {
    const source = getCurrentRequest();
    const mappedKeys = keys.map((k) => this.mapKey(k));
    return await this.cluster.impl.delete(mappedKeys, source);
  }
}
