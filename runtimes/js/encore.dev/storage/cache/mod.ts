// Cache cluster
export { CacheCluster } from "./cluster";
export type { CacheClusterConfig, EvictionPolicy } from "./cluster";

// Keyspace configuration
export type { KeyspaceConfig, WriteOptions } from "./keyspace";

// Basic keyspaces
export {
  StringKeyspace,
  IntKeyspace,
  FloatKeyspace,
  StructKeyspace
} from "./basic";
export type { GetResult } from "./basic";

// List keyspaces
export { StringListKeyspace, NumberListKeyspace } from "./list";
export type { ListPosition } from "./list";

// Set keyspaces
export { StringSetKeyspace, NumberSetKeyspace } from "./set";

// Expiry utilities
export {
  ExpireIn,
  ExpireInSeconds,
  ExpireInMinutes,
  ExpireInHours,
  ExpireDailyAt,
  NeverExpire,
  KeepTTL
} from "./expiry";
export type { Expiry } from "./expiry";

// Error types
export { CacheError, CacheMiss, CacheKeyExists } from "./errors";
