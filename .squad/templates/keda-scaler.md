# KEDA External Scaler for GitHub Issue-Driven Agent Autoscaling

> Scale agent pods to zero when idle, up when work arrives — driven by GitHub Issues.

## Overview

When running Squad on Kubernetes, agent pods sit idle when no work exists. [KEDA](https://keda.sh) (Kubernetes Event-Driven Autoscaler) solves this for queue-based workloads, but GitHub Issues isn't a native KEDA trigger.

The `keda-copilot-scaler` is a KEDA External Scaler (gRPC) that bridges this gap:
1. Polls GitHub API for issues matching specific labels (e.g., `squad:copilot`)
2. Reports queue depth as a KEDA metric
3. Handles rate limits gracefully (Retry-After, exponential backoff)
4. Supports composite scaling decisions

## Quick Start

### Prerequisites
- Kubernetes cluster with KEDA v2.x installed
- GitHub personal access token (PAT) with `repo` scope
- Helm 3.x

### 1. Install the Scaler

```bash
helm install keda-copilot-scaler oci://ghcr.io/tamirdresher/keda-copilot-scaler \
  --namespace squad-scaler --create-namespace \
  --set github.owner=YOUR_ORG \
  --set github.repo=YOUR_REPO \
  --set github.token=YOUR_TOKEN
```

Or with Kustomize:
```bash
kubectl apply -k https://github.com/tamirdresher/keda-copilot-scaler/deploy/kustomize
```

### 2. Create a ScaledObject

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: picard-scaler
  namespace: squad
spec:
  scaleTargetRef:
    name: picard-deployment
  minReplicaCount: 0          # Scale to zero when idle
  maxReplicaCount: 3
  pollingInterval: 30         # Check every 30 seconds
  cooldownPeriod: 300         # Wait 5 minutes before scaling down
  triggers:
  - type: external
    metadata:
      scalerAddress: keda-copilot-scaler.squad-scaler.svc.cluster.local:6000
      owner: your-org
      repo: your-repo
      labels: squad:copilot    # Only count issues with this label
      threshold: "1"           # Scale up when >= 1 issue exists
```

### 3. Verify

```bash
# Check the scaler is running
kubectl get pods -n squad-scaler

# Check ScaledObject status
kubectl get scaledobject picard-scaler -n squad

# Watch scaling events
kubectl get events -n squad --watch
```

## Scaling Behavior

| Open Issues | Target Replicas | Behavior |
|------------|----------------|----------|
| 0 | 0 | Scale to zero — save resources |
| 1–3 | 1 | Single agent handles work |
| 4–10 | 2 | Scale up for parallel processing |
| 10+ | 3 (max) | Maximum parallelism |

The threshold and max replicas are configurable per ScaledObject.

## Rate Limit Awareness

The scaler tracks GitHub API rate limits:
- Reads `X-RateLimit-Remaining` from API responses
- Backs off when quota is low (< 100 remaining)
- Reports rate limit metrics as secondary KEDA triggers
- Never exhausts API quota from polling

## Integration with Squad

### Machine Capabilities (#514)

Combine with machine capability labels for intelligent scheduling:

```yaml
# Only scale pods on GPU-capable nodes
spec:
  template:
    spec:
      nodeSelector:
        node.squad.dev/gpu: "true"
  triggers:
  - type: external
    metadata:
      labels: squad:copilot,needs:gpu
```

### Cooperative Rate Limiting (#515)

The scaler exposes rate limit metrics that feed into the cooperative rate limiting system:
- Current `X-RateLimit-Remaining` value
- Predicted time to exhaustion (from predictive circuit breaker)
- Can return 0 target replicas when rate limited → pods scale to zero

## Architecture

```
GitHub API                    KEDA                    Kubernetes
┌──────────┐              ┌──────────┐           ┌──────────────┐
│  Issues   │◄── poll ──►│  Scaler   │──metrics─►│ HPA / KEDA   │
│  (REST)   │             │  (gRPC)   │           │ Controller   │
└──────────┘              └──────────┘           └──────┬───────┘
                                                        │
                                                  scale up/down
                                                        │
                                                 ┌──────▼───────┐
                                                 │ Agent Pods    │
                                                 │ (0–N replicas)│
                                                 └──────────────┘
```

## Configuration Reference

| Parameter | Default | Description |
|-----------|---------|-------------|
| `github.owner` | — | Repository owner |
| `github.repo` | — | Repository name |
| `github.token` | — | GitHub PAT with `repo` scope |
| `github.labels` | `squad:copilot` | Comma-separated label filter |
| `scaler.port` | `6000` | gRPC server port |
| `scaler.pollInterval` | `30s` | GitHub API polling interval |
| `scaler.rateLimitThreshold` | `100` | Stop polling below this remaining |

## Source & Contributing

- **Repository:** [tamirdresher/keda-copilot-scaler](https://github.com/tamirdresher/keda-copilot-scaler)
- **License:** MIT
- **Language:** Go
- **Tests:** 51 passing (unit + integration)
- **CI:** GitHub Actions

The scaler is maintained as a standalone project. PRs and issues welcome.

## References

- [KEDA External Scalers](https://keda.sh/docs/latest/concepts/external-scalers/) — KEDA documentation
- [Squad on AKS](https://github.com/tamirdresher/squad-on-aks) — Full Kubernetes deployment example
- [Machine Capabilities](machine-capabilities.md) — Capability-based routing (#514)
- [Cooperative Rate Limiting](cooperative-rate-limiting.md) — Multi-agent rate management (#515)
