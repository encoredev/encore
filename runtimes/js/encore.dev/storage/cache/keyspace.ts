import { getCurrentRequest } from "../../internal/reqtrack/mod";
import { CacheCluster } from "./cluster";
import { CacheMiss, CacheKeyExists } from "./errors";
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

  /**
   * Internal: Injected key mapper function from code generation.
   * @internal
   */
  __keyMapper?: (key: K) => string;
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
 * Result of a get operation that may or may not find a value.
 */
export interface GetResult<V> {
  /** Whether the key was found */
  found: boolean;
  /** The value if found */
  value?: V;
}

/**
 * Internal interface for key pattern segments.
 * @internal
 */
interface KeyPatternSegment {
  isLiteral: boolean;
  value: string;
  field?: string;
}

/**
 * Base keyspace class with key mapping, TTL resolution, with(), and delete().
 * Shared by all keyspace types (basic, list, set).
 * @internal
 */
export abstract class KeyspaceBase<K> {
  protected readonly cluster: CacheCluster;
  protected readonly config: KeyspaceConfig<K>;
  protected readonly keyMapper: (key: K) => string;
  private _effectiveExpiry?: Expiry;

  constructor(cluster: CacheCluster, config: KeyspaceConfig<K>) {
    this.cluster = cluster;
    this.config = config;
    this.keyMapper = config.__keyMapper ?? this.createRuntimeKeyMapper(config.keyPattern);
  }

  /**
   * Creates a runtime key mapper by parsing the key pattern.
   * Used as fallback when code generation is not available.
   */
  private createRuntimeKeyMapper(pattern: string): (key: K) => string {
    const segments: KeyPatternSegment[] = pattern.split("/").map((seg) => {
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
    return this.keyMapper(key);
  }

  /**
   * Resolves the TTL for a write operation.
   * Returns i64 sentinel for NAPI: undefined=no config, -1=KeepTTL, -2=Persist/NeverExpire, >=0=ms
   */
  protected resolveTtl(options?: WriteOptions): number | undefined {
    const expiry = options?.expiry ?? this._effectiveExpiry ?? this.config.defaultExpiry;
    if (!expiry) return undefined;

    const resolved = resolveExpiry(expiry);
    if (resolved === null) return -1;       // KeepTTL
    if (resolved === undefined) return -2;   // NeverExpire → Persist
    return resolved;                         // milliseconds
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

/**
 * Base keyspace class with common operations for value-based keyspaces.
 * Extends KeyspaceBase with get/set/replace/etc operations.
 * @internal
 */
export abstract class BaseKeyspace<K, V> extends KeyspaceBase<K> {
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
   * @throws {CacheMiss} If the key is not found.
   */
  async get(key: K): Promise<V> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const result = await this.cluster.impl.get(mappedKey, source);

    if (result === null) {
      throw new CacheMiss(mappedKey);
    }

    return this.deserialize(result);
  }

  /**
   * Gets the value for the given key, returning undefined if not found.
   */
  async getOrUndefined(key: K): Promise<V | undefined> {
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
  async setIfNotExists(key: K, value: V, options?: WriteOptions): Promise<void> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = this.serialize(value);
    const ttlMs = this.resolveTtl(options);

    const set = await this.cluster.impl.setIfNotExists(mappedKey, serialized, ttlMs, source);

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

    const replaced = await this.cluster.impl.replace(mappedKey, serialized, ttlMs, source);

    if (!replaced) {
      throw new CacheMiss(mappedKey);
    }
  }

  /**
   * Gets the current value and sets a new value atomically.
   * Returns undefined if the key did not exist before.
   */
  async getAndSet(key: K, value: V, options?: WriteOptions): Promise<V | undefined> {
    const source = getCurrentRequest();
    const mappedKey = this.mapKey(key);
    const serialized = this.serialize(value);
    const ttlMs = this.resolveTtl(options);

    const oldValue = await this.cluster.impl.getAndSet(mappedKey, serialized, ttlMs, source);

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
