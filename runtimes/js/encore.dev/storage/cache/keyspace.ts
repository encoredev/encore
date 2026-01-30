import { getCurrentRequest } from "../../internal/reqtrack/mod";
import { CacheCluster } from "./cluster";
import { Expiry, resolveExpiry } from "./expiry";

/**
 * Configuration for a cache keyspace.
 */
export interface KeyspaceConfig<K> {
  /**
   * The pattern for generating cache keys.
   * Use `:fieldName` to include a field from the key type.
   *
   * @example
   * // For a simple key type (string, number)
   * keyPattern: "user/:id"
   *
   * // For a struct key type
   * keyPattern: "user/:userId/region/:region"
   */
  keyPattern: string;

  /**
   * Default expiry for cache entries in this keyspace.
   * If not set, entries do not expire.
   */
  defaultExpiry?: Expiry;
}

/**
 * Options for write operations.
 */
export interface WriteOptions {
  /**
   * Expiry for this specific write operation.
   * Overrides the keyspace's defaultExpiry.
   */
  expiry?: Expiry;
}

/**
 * Base class for all keyspace types (basic, list, set).
 * Provides key mapping, TTL resolution, with(), and delete().
 * @internal
 */
export abstract class Keyspace<K> {
  protected readonly cluster: CacheCluster;
  protected readonly config: KeyspaceConfig<K>;
  protected readonly keyMapper: (key: K) => string;
  private _effectiveExpiry?: Expiry;

  constructor(cluster: CacheCluster, config: KeyspaceConfig<K>) {
    this.cluster = cluster;
    this.config = config;
    this.keyMapper = this.createKeyMapper(config.keyPattern);
  }

  /**
   * Creates a key mapper by parsing the key pattern.
   */
  private createKeyMapper(pattern: string): (key: K) => string {
    const segments = pattern.split("/").map((seg) => {
      if (seg.startsWith(":")) {
        return { isLiteral: false, value: seg.slice(1), field: seg.slice(1) };
      }
      return { isLiteral: true, value: seg };
    });

    return (key: K) => {
      return segments
        .map((seg) => {
          if (seg.isLiteral) return seg.value;

          let val: unknown;
          if (typeof key === "object" && key !== null && seg.field) {
            val = (key as Record<string, unknown>)[seg.field];
          } else {
            val = key;
          }

          // Escape forward slashes in string values
          const str = String(val);
          return str.replace(/\//g, "\\/");
        })
        .join("/");
    };
  }

  /**
   * Maps a key to its Redis key string.
   */
  protected mapKey(key: K): string {
    const mapped = this.keyMapper(key);
    if (mapped.startsWith("__encore")) {
      throw new Error('use of reserved key prefix "__encore"');
    }
    return mapped;
  }

  /**
   * Resolves the TTL for a write operation.
   * Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms
   */
  protected resolveTtl(options?: WriteOptions): number | undefined {
    const expiry =
      options?.expiry ?? this._effectiveExpiry ?? this.config.defaultExpiry;
    if (!expiry) return undefined;

    const resolved = resolveExpiry(expiry);
    if (resolved === null) return -1; // KeepTTL
    if (resolved === undefined) return -2; // NeverExpire → Persist
    return resolved; // milliseconds
  }

  /**
   * Returns a shallow clone of this keyspace with the specified write options applied.
   * This allows setting expiry for a chain of operations.
   *
   * @example
   * ```ts
   * await myKeyspace.with({ expiry: ExpireIn(5000) }).set(key, value);
   * ```
   */
  with(options: WriteOptions): this {
    const clone = Object.create(Object.getPrototypeOf(this)) as this;
    Object.assign(clone, this);
    (clone as any)._effectiveExpiry = options.expiry ?? this._effectiveExpiry;
    return clone;
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
