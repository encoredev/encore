/**
 * CacheError is the base class for all cache-related errors.
 */
export class CacheError extends Error {
  readonly operation: string;
  readonly key?: string;

  constructor(operation: string, key: string | undefined, message: string) {
    super(`cache ${operation}${key ? ` "${key}"` : ""}: ${message}`);
    this.name = "CacheError";
    this.operation = operation;
    this.key = key;
  }
}

/**
 * CacheMiss is thrown when a cache key is not found.
 */
export class CacheMiss extends CacheError {
  constructor(key: string) {
    super("get", key, "key not found");
    this.name = "CacheMiss";
  }
}

/**
 * CacheKeyExists is thrown when attempting to set a key that already exists
 * using setIfNotExists.
 */
export class CacheKeyExists extends CacheError {
  constructor(key: string) {
    super("setIfNotExists", key, "key already exists");
    this.name = "CacheKeyExists";
  }
}
