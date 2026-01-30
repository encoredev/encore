/**
 * CacheMiss is thrown when a cache key is not found.
 */
export class CacheMiss extends Error {
  readonly key: string;

  constructor(key: string) {
    super(`cache miss: key "${key}" not found`);
    this.name = "CacheMiss";
    this.key = key;
  }
}

/**
 * CacheKeyExists is thrown when attempting to set a key that already exists
 * using setIfNotExists.
 */
export class CacheKeyExists extends Error {
  readonly key: string;

  constructor(key: string) {
    super(`cache key exists: key "${key}" already exists`);
    this.name = "CacheKeyExists";
    this.key = key;
  }
}
