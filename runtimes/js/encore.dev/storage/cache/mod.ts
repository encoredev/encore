// Cache cluster
export { CacheCluster } from "./cluster";
export type { CacheClusterConfig, EvictionPolicy } from "./cluster";

// Keyspace configuration
export type { KeyspaceConfig, WriteOptions, GetResult } from "./keyspace";

// Basic keyspaces
export { StringKeyspace, IntKeyspace, FloatKeyspace, StructKeyspace } from "./basic";

// List keyspace
export { ListKeyspace } from "./list";

// Set keyspace
export { SetKeyspace } from "./set";

// Expiry utilities
export {
  ExpireIn,
  ExpireInSeconds,
  ExpireInMinutes,
  ExpireInHours,
  ExpireDailyAt,
  NeverExpire,
  KeepTTL,
} from "./expiry";
export type { Expiry } from "./expiry";

// Error types
export { CacheMiss, CacheKeyExists } from "./errors";
