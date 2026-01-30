---
seotitle: Using caches in your TypeScript backend application
seodesc: Learn how to implement caches to optimize response times and reduce cost in your TypeScript microservices cloud backend.
title: Caching
subtitle: Optimize response times and reduce costs by avoiding re-work
infobox: {
  title: "Caching",
  import: "encore.dev/storage/cache",
}
lang: ts
---

A cache is a high-speed storage layer, commonly used in distributed systems to improve user experiences
by reducing latency, improving system performance, and avoiding expensive computation.

For scalable systems you typically want to deploy the cache as a separate
infrastructure resource, allowing you to run multiple instances of your application concurrently.

Encore's built-in Caching API lets you use high-performance caches (using [Redis](https://redis.io/)) in a cloud-agnostic declarative fashion. At deployment, Encore will automatically [provision the required infrastructure](/docs/platform/infrastructure/infra).

## Cache clusters

To use caching in Encore, you must first define a *cache cluster*.
Each cache cluster defined in your application will be provisioned as a separate Redis instance
by Encore.

This gives you fine-grained control over which service(s) should use the same cache cluster
and which should have a separate one.

It looks like this:

```typescript
import { CacheCluster } from "encore.dev/storage/cache";

const cluster = new CacheCluster("my-cache", {
  // EvictionPolicy tells Redis how to evict keys when the cache reaches
  // its memory limit. For typical cache use cases, "allkeys-lru" is a good default.
  evictionPolicy: "allkeys-lru",
});
```

<Callout type="info">

When starting out it's recommended to use a single cache cluster
that's shared between your different services.

</Callout>

### Eviction policies

The eviction policy determines how Redis handles keys when the cache reaches its memory limit:

- `"noeviction"` - Returns errors when memory limit is reached (default)
- `"allkeys-lru"` - Evicts least recently used keys first
- `"allkeys-lfu"` - Evicts least frequently used keys first
- `"allkeys-random"` - Evicts random keys
- `"volatile-lru"` - Evicts least recently used keys with an expiry set
- `"volatile-lfu"` - Evicts least frequently used keys with an expiry set
- `"volatile-ttl"` - Evicts keys with shortest TTL first
- `"volatile-random"` - Evicts random keys with an expiry set

## Keyspaces

When using a cache, each cached item is stored at a particular key, which is typically an arbitrary string.
If you use a cache cluster to cache different sets of data, it's important that distinct data sets have non-overlapping keys.

Each value stored in the cache also has a specific type, and certain cache operations can only be performed on certain types. For example, a common cache operation is to increment an integer value that is stored in the cache. If you try to apply this operation on a value that is not an integer, an error is returned.

Encore provides a simple, type-safe solution to these problems through Keyspaces.

In order to begin storing data in your cache, you must first define a Keyspace.

Each keyspace has a Key type and a Value type. The Key type is much like a map key, in that it tells Encore where in the cache the item is stored. The Key type is combined with the Key Pattern to produce a string that is the Redis cache key.

The Value type is the type of the values stored in that keyspace. For many keyspaces this is specified in the name of the constructor.
For example, `StringKeyspace` stores `string` values, `IntKeyspace` stores `number` values (as 64-bit integers).

### Example: Rate limiting

For example, if you want to rate limit the number of requests per user ID it looks like this:

```typescript
import { CacheCluster, IntKeyspace, ExpireIn } from "encore.dev/storage/cache";
import { api, APIError } from "encore.dev/api";
import { getAuthData } from "~encore/auth";

const cluster = new CacheCluster("rate-limit", {
  evictionPolicy: "allkeys-lru",
});

// RequestsPerUser tracks the number of requests per user.
// The cache items expire after 10 seconds without activity.
const requestsPerUser = new IntKeyspace<{ userId: string }>(cluster, {
  keyPattern: "requests/:userId",
  defaultExpiry: ExpireIn(10 * 1000), // 10 seconds in milliseconds
});

export const myEndpoint = api(
  { expose: true, method: "GET", path: "/my-endpoint", auth: true },
  async (): Promise<{ message: string }> => {
    const auth = getAuthData();
    if (!auth) {
      throw APIError.unauthenticated("not authenticated");
    }

    const count = await requestsPerUser.increment({ userId: auth.userID }, 1);

    // NOTE: this "fails open", meaning if we can't communicate with the cache
    // we default to allowing the requests.
    //
    // Consider whether that's the correct behavior for your application,
    // or if you want to return an error to the user in that case.
    if (count > 10) {
      throw APIError.resourceExhausted("rate limit exceeded");
    }

    return { message: "Hello!" };
  }
);
```

As you can see, the `requestsPerUser` defines a `keyPattern` which is set to `"requests/:userId"`.
Here `:userId` refers to the field in the key type object. When you call `requestsPerUser.increment({ userId: "user123" }, 1)`,
Encore generates the Redis key `"requests/user123"`.

### Key patterns with multiple fields

You can define key types with multiple fields to create more complex key patterns:

```typescript
interface ResourceKey {
  userId: string;
  resourcePath: string;
}

// ResourceRequestsPerUser tracks the number of requests per user and resource.
const resourceRequestsPerUser = new IntKeyspace<ResourceKey>(cluster, {
  keyPattern: "requests/:userId/:resourcePath",
  defaultExpiry: ExpireIn(10 * 1000),
});

// Usage:
await resourceRequestsPerUser.increment(
  { userId: "user123", resourcePath: "api/users" },
  1
);
```

## Keyspace types

Encore comes with several keyspace types, each designed for different use cases:

### StringKeyspace

Stores string values.

```typescript
import { StringKeyspace } from "encore.dev/storage/cache";

const tokens = new StringKeyspace<{ tokenId: string }>(cluster, {
  keyPattern: "token/:tokenId",
  defaultExpiry: ExpireIn(3600 * 1000), // 1 hour
});

// Set a value
await tokens.set({ tokenId: "abc123" }, "user-token-value");

// Get a value
const token = await tokens.get({ tokenId: "abc123" });

// Get or undefined (doesn't throw on cache miss)
const maybeToken = await tokens.getOrUndefined({ tokenId: "abc123" });

// Delete a value
await tokens.delete({ tokenId: "abc123" });
```

Additional string operations:
- `append(key, value)` - Appends to the existing value
- `getRange(key, start, end)` - Gets a substring
- `setRange(key, offset, value)` - Overwrites part of the string
- `strlen(key)` - Gets the string length

### IntKeyspace

Stores 64-bit integer values.

```typescript
import { IntKeyspace } from "encore.dev/storage/cache";

const counters = new IntKeyspace<{ counterId: string }>(cluster, {
  keyPattern: "counter/:counterId",
});

// Set a value
await counters.set({ counterId: "visits" }, 0);

// Increment and get new value
const newCount = await counters.increment({ counterId: "visits" }, 1);

// Decrement
const decremented = await counters.decrement({ counterId: "visits" }, 1);
```

### FloatKeyspace

Stores 64-bit floating-point values.

```typescript
import { FloatKeyspace } from "encore.dev/storage/cache";

const scores = new FloatKeyspace<{ oddsId: string }>(cluster, {
  keyPattern: "odds/:oddsId",
});

// Set a value
await scores.set({ oddsId: "game1" }, 1.5);

// Increment by a float amount
const newOdds = await scores.increment({ oddsId: "game1" }, 0.1);
```

### StructKeyspace

Stores structured data (objects) serialized as JSON.

```typescript
import { StructKeyspace } from "encore.dev/storage/cache";

interface UserProfile {
  name: string;
  email: string;
  preferences: {
    theme: "light" | "dark";
    notifications: boolean;
  };
}

const profiles = new StructKeyspace<{ oddsId: string }, UserProfile>(cluster, {
  keyPattern: "profile/:userId",
  defaultExpiry: ExpireIn(3600 * 1000),
});

// Set a structured value
await profiles.set(
  { userId: "user123" },
  {
    name: "Alice",
    email: "alice@example.com",
    preferences: { theme: "dark", notifications: true },
  }
);

// Get the value
const profile = await profiles.get({ userId: "user123" });
```

### ListKeyspace

Stores ordered lists of values.

```typescript
import { ListKeyspace } from "encore.dev/storage/cache";

const recentItems = new ListKeyspace<{ oddsId: string }, string>(cluster, {
  keyPattern: "recent/:userId",
});

// Push items to the list
await recentItems.pushRight({ userId: "user123" }, "item1", "item2");

// Get items from the list
const items = await recentItems.getRange({ userId: "user123" }, 0, -1); // Get all

// Pop an item
const lastItem = await recentItems.popRight({ userId: "user123" });
```

### SetKeyspace

Stores unordered sets of unique values.

```typescript
import { SetKeyspace } from "encore.dev/storage/cache";

const tags = new SetKeyspace<{ oddsId: string }, string>(cluster, {
  keyPattern: "tags/:articleId",
});

// Add members to the set
await tags.add({ articleId: "post1" }, "typescript", "encore", "backend");

// Check membership
const hasTag = await tags.contains({ articleId: "post1" }, "typescript");

// Get all members
const allTags = await tags.members({ articleId: "post1" });

// Remove members
await tags.remove({ articleId: "post1" }, "backend");
```

## Expiry options

Encore provides several ways to set cache entry expiration:

```typescript
import {
  ExpireIn,
  ExpireInSeconds,
  ExpireInMinutes,
  ExpireInHours,
  ExpireDailyAt,
  NeverExpire,
  KeepTTL,
} from "encore.dev/storage/cache";

// Expire in milliseconds
const expiry1 = ExpireIn(5000); // 5 seconds

// Expire in seconds
const expiry2 = ExpireInSeconds(30);

// Expire in minutes
const expiry3 = ExpireInMinutes(5);

// Expire in hours
const expiry4 = ExpireInHours(24);

// Expire at a specific time each day (UTC)
const expiry5 = ExpireDailyAt({ hour: 0, minute: 0 }); // Midnight UTC

// Never expire (Redis may still evict based on eviction policy)
const expiry6 = NeverExpire;

// Keep existing TTL when updating (for write operations)
const expiry7 = KeepTTL;
```

## Write options

When setting values, you can specify additional options:

```typescript
// Only set if key doesn't exist
await keyspace.set(key, value, { onlyIfNotExists: true });

// Set with a specific expiry (overrides default)
await keyspace.set(key, value, { expiry: ExpireInMinutes(30) });

// Keep existing TTL
await keyspace.set(key, value, { expiry: KeepTTL });
```

## Error handling

Cache operations can throw two special error types:

```typescript
import { CacheMiss, CacheKeyExists } from "encore.dev/storage/cache";

try {
  const value = await keyspace.get(key);
} catch (err) {
  if (err instanceof CacheMiss) {
    // Key doesn't exist in cache
    console.log("Cache miss!");
  }
  throw err;
}

// Or use getOrUndefined to avoid exceptions for missing keys
const value = await keyspace.getOrUndefined(key);
if (value === undefined) {
  // Key doesn't exist
}
```

## Testing

When running tests, Encore spins up an isolated cache environment for each test.

This way you don't have to think about clearing the cache between tests,
or worrying about whether one test affects another.
Each test is automatically fully isolated.

## Local development

For local development, Encore uses Redis running in Docker.
Make sure Docker is installed and running on your machine before running `encore run`.
