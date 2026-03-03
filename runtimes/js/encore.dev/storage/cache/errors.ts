/**
 * CacheError is the base class for all cache-related errors.
 */
export class CacheError extends Error {
  constructor(msg: string) {
    // extending errors causes issues after you construct them, unless you apply the following fixes
    super(msg);

    // set error name as constructor name, make it not enumerable to keep native Error behavior
    // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/new.target#new.target_in_constructors
    Object.defineProperty(this, "name", {
      value: "CacheError",
      enumerable: false,
      configurable: true
    });

    // Fix the prototype chain, capture stack trace.
    Object.setPrototypeOf(this, CacheError.prototype);
    Error.captureStackTrace(this, this.constructor);
  }
}

/**
 * CacheMiss is thrown when a cache key is not found.
 */
export class CacheMiss extends CacheError {
  constructor(key: string) {
    // extending errors causes issues after you construct them, unless you apply the following fixes
    super(`cache key "${key}" not found`);

    // set error name as constructor name, make it not enumerable to keep native Error behavior
    // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/new.target#new.target_in_constructors
    Object.defineProperty(this, "name", {
      value: "CacheMiss",
      enumerable: false,
      configurable: true
    });

    // Fix the prototype chain, capture stack trace.
    Object.setPrototypeOf(this, CacheMiss.prototype);
    Error.captureStackTrace(this, this.constructor);
  }
}

/**
 * CacheKeyExists is thrown when attempting to set a key that already exists
 * using setIfNotExists.
 */
export class CacheKeyExists extends CacheError {
  constructor(key: string) {
    // extending errors causes issues after you construct them, unless you apply the following fixes
    super(`cache key "${key}" already exists`);

    // set error name as constructor name, make it not enumerable to keep native Error behavior
    // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/new.target#new.target_in_constructors
    Object.defineProperty(this, "name", {
      value: "CacheKeyExists",
      enumerable: false,
      configurable: true
    });

    // Fix the prototype chain, capture stack trace.
    Object.setPrototypeOf(this, CacheKeyExists.prototype);
    Error.captureStackTrace(this, this.constructor);
  }
}
