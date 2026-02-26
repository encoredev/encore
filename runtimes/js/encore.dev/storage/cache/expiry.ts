/**
 * Expiry represents a cache key expiration configuration.
 * Use the helper functions to create expiry configurations.
 */
export type Expiry =
  | { type: "duration"; durationMs: number }
  | { type: "time"; hours: number; minutes: number; seconds: number }
  | { type: "never" }
  | { type: "keepTTL" };

/**
 * ExpireIn sets the cache entry to expire after the specified duration.
 * @param ms - Duration in milliseconds
 */
export function ExpireIn(ms: number): Expiry {
  return { type: "duration", durationMs: ms };
}

/**
 * ExpireInSeconds sets the cache entry to expire after the specified seconds.
 * @param seconds - Duration in seconds
 */
export function ExpireInSeconds(seconds: number): Expiry {
  return { type: "duration", durationMs: seconds * 1000 };
}

/**
 * ExpireInMinutes sets the cache entry to expire after the specified minutes.
 * @param minutes - Duration in minutes
 */
export function ExpireInMinutes(minutes: number): Expiry {
  return { type: "duration", durationMs: minutes * 60 * 1000 };
}

/**
 * ExpireInHours sets the cache entry to expire after the specified hours.
 * @param hours - Duration in hours
 */
export function ExpireInHours(hours: number): Expiry {
  return { type: "duration", durationMs: hours * 60 * 60 * 1000 };
}

/**
 * ExpireDailyAt sets the cache entry to expire at a specific time each day (UTC).
 * @param hours - Hour (0-23)
 * @param minutes - Minutes (0-59)
 * @param seconds - Seconds (0-59)
 */
export function ExpireDailyAt(
  hours: number,
  minutes: number,
  seconds: number
): Expiry {
  return { type: "time", hours, minutes, seconds };
}

/**
 * NeverExpire sets the cache entry to never expire.
 * Note: Redis may still evict the key based on the eviction policy.
 */
export const NeverExpire: Expiry = { type: "never" };

/**
 * KeepTTL preserves the existing TTL when updating a cache entry.
 * If the key doesn't exist, no TTL is set.
 */
export const KeepTTL: Expiry = { type: "keepTTL" };

/**
 * Resolves an Expiry to milliseconds, or undefined if no TTL should be set.
 * Returns null for KeepTTL to indicate TTL should be preserved.
 * @internal
 */
export function resolveExpiry(expiry: Expiry): number | null | undefined {
  switch (expiry.type) {
    case "duration":
      return expiry.durationMs;

    case "time": {
      const now = new Date();
      const target = new Date(now);
      target.setUTCHours(expiry.hours, expiry.minutes, expiry.seconds, 0);

      // If target time has passed today, set for tomorrow
      if (target.getTime() <= now.getTime()) {
        target.setUTCDate(target.getUTCDate() + 1);
      }

      return target.getTime() - now.getTime();
    }

    case "never":
      return undefined;

    case "keepTTL":
      return null;
  }
}
