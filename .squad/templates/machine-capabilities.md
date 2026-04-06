# Machine Capability Discovery & Label-Based Routing

> Enable Ralph to skip issues requiring capabilities the current machine lacks.

## Overview

When running Squad across multiple machines (laptops, DevBoxes, GPU servers, Kubernetes nodes), each machine has different tooling. The capability system lets you declare what each machine can do, and Ralph automatically routes work accordingly.

## Setup

### 1. Create a Capabilities Manifest

Create `~/.squad/machine-capabilities.json` (user-wide) or `.squad/machine-capabilities.json` (project-local):

```json
{
  "machine": "MY-LAPTOP",
  "capabilities": ["browser", "personal-gh", "onedrive"],
  "missing": ["gpu", "docker", "azure-speech"],
  "lastUpdated": "2026-03-22T00:00:00Z"
}
```

### 2. Label Issues with Requirements

Add `needs:*` labels to issues that require specific capabilities:

| Label | Meaning |
|-------|---------|
| `needs:browser` | Requires Playwright / browser automation |
| `needs:gpu` | Requires NVIDIA GPU |
| `needs:personal-gh` | Requires personal GitHub account |
| `needs:emu-gh` | Requires Enterprise Managed User account |
| `needs:azure-cli` | Requires authenticated Azure CLI |
| `needs:docker` | Requires Docker daemon |
| `needs:onedrive` | Requires OneDrive sync |
| `needs:teams-mcp` | Requires Teams MCP tools |

Custom capabilities are supported — any `needs:X` label works if `X` is in the machine's `capabilities` array.

### 3. Run Ralph

```bash
squad watch --interval 5
```

Ralph will log skipped issues:
```
⏭️ Skipping #42 "Train ML model" — missing: gpu
✓ Triaged #43 "Fix CSS layout" → Picard (routing-rule)
```

## How It Works

1. Ralph loads `machine-capabilities.json` at startup
2. For each open issue, Ralph extracts `needs:*` labels
3. If any required capability is missing, the issue is skipped
4. Issues without `needs:*` labels are always processed (opt-in system)

## Kubernetes Integration

On Kubernetes, machine capabilities map to node labels:

```yaml
# Node labels (set by capability DaemonSet or manually)
node.squad.dev/gpu: "true"
node.squad.dev/browser: "true"

# Pod spec uses nodeSelector
spec:
  nodeSelector:
    node.squad.dev/gpu: "true"
```

A DaemonSet can run capability discovery on each node and maintain labels automatically. See the [squad-on-aks](https://github.com/tamirdresher/squad-on-aks) project for a complete Kubernetes deployment example.