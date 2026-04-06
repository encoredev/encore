# Cooperative Rate Limiting for Multi-Agent Deployments

> Coordinate API quota across multiple Ralph instances to prevent cascading failures.

## Problem

The [circuit breaker template](ralph-circuit-breaker.md) handles single-instance rate limiting well. But when multiple Ralphs run across machines (or pods on K8s), each instance independently hits API limits:

- **No coordination** — 5 Ralphs each think they have full API quota
- **Thundering herd** — All Ralphs retry simultaneously after rate limit resets
- **Priority inversion** — Low-priority work exhausts quota before critical work runs
- **Reactive only** — Circuit opens AFTER 429, wasting the failed request

## Solution: 6-Pattern Architecture

These patterns layer on top of the existing circuit breaker. Each is independent — adopt one or all.

### Pattern 1: Traffic Light (RAAS — Rate-Aware Agent Scheduling)

Map GitHub API `X-RateLimit-Remaining` to traffic light states:

| State | Remaining % | Behavior |
|-------|------------|----------|
| 🟢 GREEN | >20% | Normal operation |
| 🟡 AMBER | 5–20% | Only P0 agents proceed |
| 🔴 RED | <5% | Block all except emergency P0 |

```typescript
type TrafficLight = 'green' | 'amber' | 'red';

function getTrafficLight(remaining: number, limit: number): TrafficLight {
  const pct = remaining / limit;
  if (pct > 0.20) return 'green';
  if (pct > 0.05) return 'amber';
  return 'red';
}

function shouldProceed(light: TrafficLight, agentPriority: number): boolean {
  if (light === 'green') return true;
  if (light === 'amber') return agentPriority === 0; // P0 only
  return false; // RED — block all
}
```

### Pattern 2: Cooperative Token Pool (CMARP)

A shared JSON file (`~/.squad/rate-pool.json`) distributes API quota:

```json
{
  "totalLimit": 5000,
  "resetAt": "2026-03-22T20:00:00Z",
  "allocations": {
    "picard": { "priority": 0, "allocated": 2000, "used": 450, "leaseExpiry": "2026-03-22T19:55:00Z" },
    "data": { "priority": 1, "allocated": 1750, "used": 200, "leaseExpiry": "2026-03-22T19:55:00Z" },
    "ralph": { "priority": 2, "allocated": 1250, "used": 100, "leaseExpiry": "2026-03-22T19:55:00Z" }
  }
}
```

**Rules:**
- P0 agents (Lead) get 40% of quota
- P1 agents (specialists) get 35%
- P2 agents (Ralph, Scribe) get 25%
- Stale leases (>5 minutes without heartbeat) are auto-recovered
- Each agent checks their remaining allocation before making API calls

```typescript
interface RatePoolAllocation {
  priority: number;
  allocated: number;
  used: number;
  leaseExpiry: string;
}

interface RatePool {
  totalLimit: number;
  resetAt: string;
  allocations: Record<string, RatePoolAllocation>;
}

function canUseQuota(pool: RatePool, agentName: string): boolean {
  const alloc = pool.allocations[agentName];
  if (!alloc) return true; // Unknown agent — allow (graceful)
  
  // Reclaim stale leases from crashed agents
  const now = new Date();
  for (const [name, a] of Object.entries(pool.allocations)) {
    if (new Date(a.leaseExpiry) < now && name !== agentName) {
      a.allocated = 0; // Reclaim
    }
  }
  
  return alloc.used < alloc.allocated;
}
```

### Pattern 3: Predictive Circuit Breaker (PCB)

Opens the circuit BEFORE getting a 429 by predicting when quota will run out:

```typescript
interface RateSample {
  timestamp: number;  // Date.now()
  remaining: number;  // from X-RateLimit-Remaining header
}

class PredictiveCircuitBreaker {
  private samples: RateSample[] = [];
  private readonly maxSamples = 10;
  private readonly warningThresholdSeconds = 120;

  addSample(remaining: number): void {
    this.samples.push({ timestamp: Date.now(), remaining });
    if (this.samples.length > this.maxSamples) {
      this.samples.shift();
    }
  }

  /** Predict seconds until quota exhaustion using linear regression */
  predictExhaustion(): number | null {
    if (this.samples.length < 3) return null;

    const n = this.samples.length;
    const first = this.samples[0];
    const last = this.samples[n - 1];
    
    const elapsedMs = last.timestamp - first.timestamp;
    if (elapsedMs === 0) return null;
    
    const consumedPerMs = (first.remaining - last.remaining) / elapsedMs;
    if (consumedPerMs <= 0) return null; // Not consuming — safe
    
    const msUntilExhausted = last.remaining / consumedPerMs;
    return msUntilExhausted / 1000;
  }

  shouldOpen(): boolean {
    const eta = this.predictExhaustion();
    if (eta === null) return false;
    return eta < this.warningThresholdSeconds;
  }
}
```

### Pattern 4: Priority Retry Windows (PWJG)

Non-overlapping jitter windows prevent thundering herd:

| Priority | Retry Window | Description |
|----------|-------------|-------------|
| P0 (Lead) | 500ms–5s | Recovers first |
| P1 (Specialists) | 2s–30s | Moderate delay |
| P2 (Ralph/Scribe) | 5s–60s | Most patient |

```typescript
function getRetryDelay(priority: number, attempt: number): number {
  const windows: Record<number, [number, number]> = {
    0: [500, 5000],     // P0: 500ms–5s
    1: [2000, 30000],   // P1: 2s–30s
    2: [5000, 60000],   // P2: 5s–60s
  };
  
  const [min, max] = windows[priority] ?? windows[2];
  const base = Math.min(min * Math.pow(2, attempt), max);
  const jitter = Math.random() * base * 0.5;
  return base + jitter;
}
```

### Pattern 5: Resource Epoch Tracker (RET)

Heartbeat-based lease system for multi-machine deployments:

```typescript
interface ResourceLease {
  agent: string;
  machine: string;
  leaseStart: string;
  leaseExpiry: string;  // Typically 5 minutes from now
  allocated: number;
}

// Each agent renews its lease every 2 minutes
// If lease expires (agent crashed), allocation is reclaimed
```

### Pattern 6: Cascade Dependency Detector (CDD)

Track downstream failures and apply backpressure:

```
Agent A (rate limited) → Agent B (waiting for A) → Agent C (waiting for B)
                         ↑ Backpressure signal: "don't start new work"
```

When a dependency is rate-limited, upstream agents should pause new work rather than queuing requests that will fail.

## Kubernetes Integration

On K8s, cooperative rate limiting can use KEDA to scale pods based on API quota:

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
spec:
  scaleTargetRef:
    name: ralph-deployment
  triggers:
  - type: external
    metadata:
      scalerAddress: keda-copilot-scaler:6000
      # Scaler returns 0 when rate limited → pods scale to zero
```

See [keda-copilot-scaler](https://github.com/tamirdresher/keda-copilot-scaler) for a complete implementation.

## Quick Start

1. **Minimum viable:** Adopt Pattern 1 (Traffic Light) — read `X-RateLimit-Remaining` from API responses
2. **Multi-machine:** Add Pattern 2 (Cooperative Pool) — shared `rate-pool.json`
3. **Production:** Add Pattern 3 (Predictive CB) — prevent 429s entirely
4. **Kubernetes:** Add KEDA scaler for automatic pod scaling

## References

- [Circuit Breaker Template](ralph-circuit-breaker.md) — Foundation patterns
- [Squad on AKS](https://github.com/tamirdresher/squad-on-aks) — Production K8s deployment
- [KEDA Copilot Scaler](https://github.com/tamirdresher/keda-copilot-scaler) — Custom KEDA external scaler
