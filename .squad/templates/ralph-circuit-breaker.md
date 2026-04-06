# Ralph Circuit Breaker — Model Rate Limit Fallback

> Classic circuit breaker pattern (Hystrix / Polly / Resilience4j) applied to Copilot model selection.
> When the preferred model hits rate limits, Ralph automatically degrades to free-tier models, then self-heals.

## Problem

When running multiple Ralph instances across repos, Copilot model rate limits cause cascading failures.
All Ralphs fail simultaneously when the preferred model (e.g., `claude-sonnet-4.6`) hits quota.

Premium models burn quota fast:
| Model | Multiplier | Risk |
|-------|-----------|------|
| `claude-sonnet-4.6` | 1x | Moderate with many Ralphs |
| `claude-opus-4.6` | 10x | High |
| `gpt-5.4` | 50x | Very high |
| `gpt-5.4-mini` | **0x** | **Free — unlimited** |
| `gpt-5-mini` | **0x** | **Free — unlimited** |
| `gpt-4.1` | **0x** | **Free — unlimited** |

## Circuit Breaker States

```
┌─────────┐   rate limit error    ┌────────┐
│ CLOSED  │ ───────────────────►  │  OPEN  │
│ (normal)│                       │(fallback)│
└────┬────┘   ◄──────────────── └────┬────┘
     │        2 consecutive          │
     │        successes              │ cooldown expires
     │                               ▼
     │                          ┌──────────┐
     └───── success ◄────────  │HALF-OPEN │
             (close)            │ (testing) │
                                └──────────┘
```

### CLOSED (normal operation)
- Use preferred model from config
- Every successful response confirms circuit stays closed
- On rate limit error → transition to OPEN

### OPEN (rate limited — fallback active)
- Fall back through the free-tier model chain:
  1. `gpt-5.4-mini`
  2. `gpt-5-mini`
  3. `gpt-4.1`
- Start cooldown timer (default: 10 minutes)
- When cooldown expires → transition to HALF-OPEN

### HALF-OPEN (testing recovery)
- Try preferred model again
- If 2 consecutive successes → transition to CLOSED
- If rate limit error → back to OPEN, reset cooldown

## State File: `.squad/ralph-circuit-breaker.json`

```json
{
  "state": "closed",
  "preferredModel": "claude-sonnet-4.6",
  "fallbackChain": ["gpt-5.4-mini", "gpt-5-mini", "gpt-4.1"],
  "currentFallbackIndex": 0,
  "cooldownMinutes": 10,
  "openedAt": null,
  "halfOpenSuccesses": 0,
  "consecutiveFailures": 0,
  "metrics": {
    "totalFallbacks": 0,
    "totalRecoveries": 0,
    "lastFallbackAt": null,
    "lastRecoveryAt": null
  }
}
```

## PowerShell Functions

Paste these into your `ralph-watch.ps1` or source them from a shared module.

### `Get-CircuitBreakerState`

```powershell
function Get-CircuitBreakerState {
    param([string]$StateFile = ".squad/ralph-circuit-breaker.json")

    if (-not (Test-Path $StateFile)) {
        $default = @{
            state              = "closed"
            preferredModel     = "claude-sonnet-4.6"
            fallbackChain      = @("gpt-5.4-mini", "gpt-5-mini", "gpt-4.1")
            currentFallbackIndex = 0
            cooldownMinutes    = 10
            openedAt           = $null
            halfOpenSuccesses  = 0
            consecutiveFailures = 0
            metrics            = @{
                totalFallbacks  = 0
                totalRecoveries = 0
                lastFallbackAt  = $null
                lastRecoveryAt  = $null
            }
        }
        $default | ConvertTo-Json -Depth 3 | Set-Content $StateFile
        return $default
    }

    return (Get-Content $StateFile -Raw | ConvertFrom-Json)
}
```

### `Save-CircuitBreakerState`

```powershell
function Save-CircuitBreakerState {
    param(
        [object]$State,
        [string]$StateFile = ".squad/ralph-circuit-breaker.json"
    )

    $State | ConvertTo-Json -Depth 3 | Set-Content $StateFile
}
```

### `Get-CurrentModel`

Returns the model Ralph should use right now, based on circuit state.

```powershell
function Get-CurrentModel {
    param([string]$StateFile = ".squad/ralph-circuit-breaker.json")

    $cb = Get-CircuitBreakerState -StateFile $StateFile

    switch ($cb.state) {
        "closed" {
            return $cb.preferredModel
        }
        "open" {
            # Check if cooldown has expired
            if ($cb.openedAt) {
                $opened = [DateTime]::Parse($cb.openedAt)
                $elapsed = (Get-Date) - $opened
                if ($elapsed.TotalMinutes -ge $cb.cooldownMinutes) {
                    # Transition to half-open
                    $cb.state = "half-open"
                    $cb.halfOpenSuccesses = 0
                    Save-CircuitBreakerState -State $cb -StateFile $StateFile
                    Write-Host "  [circuit-breaker] Cooldown expired. Testing preferred model..." -ForegroundColor Yellow
                    return $cb.preferredModel
                }
            }
            # Still in cooldown — use fallback
            $idx = [Math]::Min($cb.currentFallbackIndex, $cb.fallbackChain.Count - 1)
            return $cb.fallbackChain[$idx]
        }
        "half-open" {
            return $cb.preferredModel
        }
        default {
            return $cb.preferredModel
        }
    }
}
```

### `Update-CircuitBreakerOnSuccess`

Call after every successful model response.

```powershell
function Update-CircuitBreakerOnSuccess {
    param([string]$StateFile = ".squad/ralph-circuit-breaker.json")

    $cb = Get-CircuitBreakerState -StateFile $StateFile
    $cb.consecutiveFailures = 0

    if ($cb.state -eq "half-open") {
        $cb.halfOpenSuccesses++
        if ($cb.halfOpenSuccesses -ge 2) {
            # Recovery! Close the circuit
            $cb.state = "closed"
            $cb.openedAt = $null
            $cb.halfOpenSuccesses = 0
            $cb.currentFallbackIndex = 0
            $cb.metrics.totalRecoveries++
            $cb.metrics.lastRecoveryAt = (Get-Date).ToString("o")
            Save-CircuitBreakerState -State $cb -StateFile $StateFile
            Write-Host "  [circuit-breaker] RECOVERED — back to preferred model ($($cb.preferredModel))" -ForegroundColor Green
            return
        }
        Save-CircuitBreakerState -State $cb -StateFile $StateFile
        Write-Host "  [circuit-breaker] Half-open success $($cb.halfOpenSuccesses)/2" -ForegroundColor Yellow
        return
    }

    # closed state — nothing to do
}
```

### `Update-CircuitBreakerOnRateLimit`

Call when a model response indicates rate limiting (HTTP 429 or error message containing "rate limit").

```powershell
function Update-CircuitBreakerOnRateLimit {
    param([string]$StateFile = ".squad/ralph-circuit-breaker.json")

    $cb = Get-CircuitBreakerState -StateFile $StateFile
    $cb.consecutiveFailures++

    if ($cb.state -eq "closed" -or $cb.state -eq "half-open") {
        # Open the circuit
        $cb.state = "open"
        $cb.openedAt = (Get-Date).ToString("o")
        $cb.halfOpenSuccesses = 0
        $cb.currentFallbackIndex = 0
        $cb.metrics.totalFallbacks++
        $cb.metrics.lastFallbackAt = (Get-Date).ToString("o")
        Save-CircuitBreakerState -State $cb -StateFile $StateFile

        $fallbackModel = $cb.fallbackChain[0]
        Write-Host "  [circuit-breaker] RATE LIMITED — falling back to $fallbackModel (cooldown: $($cb.cooldownMinutes)m)" -ForegroundColor Red
        return
    }

    if ($cb.state -eq "open") {
        # Already open — try next fallback in chain if current one also fails
        if ($cb.currentFallbackIndex -lt ($cb.fallbackChain.Count - 1)) {
            $cb.currentFallbackIndex++
            $nextModel = $cb.fallbackChain[$cb.currentFallbackIndex]
            Write-Host "  [circuit-breaker] Fallback also limited — trying $nextModel" -ForegroundColor Red
        }
        # Reset cooldown timer
        $cb.openedAt = (Get-Date).ToString("o")
        Save-CircuitBreakerState -State $cb -StateFile $StateFile
    }
}
```

## Integration with ralph-watch.ps1

In your Ralph polling loop, wrap the model selection:

```powershell
# At the top of your polling loop
$model = Get-CurrentModel

# When invoking copilot CLI
$result = copilot-cli --model $model ...

# After the call
if ($result -match "rate.?limit" -or $LASTEXITCODE -eq 429) {
    Update-CircuitBreakerOnRateLimit
} else {
    Update-CircuitBreakerOnSuccess
}
```

### Full integration example

```powershell
# Source the circuit breaker functions
. .squad-templates/ralph-circuit-breaker-functions.ps1

while ($true) {
    $model = Get-CurrentModel
    Write-Host "Polling with model: $model"

    try {
        # Your existing Ralph logic here, but pass $model
        $response = Invoke-RalphCycle -Model $model

        # Success path
        Update-CircuitBreakerOnSuccess
    }
    catch {
        if ($_.Exception.Message -match "rate.?limit|429|quota|Too Many Requests") {
            Update-CircuitBreakerOnRateLimit
            # Retry immediately with fallback model
            continue
        }
        # Other errors — handle normally
        throw
    }

    Start-Sleep -Seconds $pollInterval
}
```

## Configuration

Override defaults by editing `.squad/ralph-circuit-breaker.json`:

| Field | Default | Description |
|-------|---------|-------------|
| `preferredModel` | `claude-sonnet-4.6` | Model to use when circuit is closed |
| `fallbackChain` | `["gpt-5.4-mini", "gpt-5-mini", "gpt-4.1"]` | Ordered fallback models (all free-tier) |
| `cooldownMinutes` | `10` | How long to wait before testing recovery |

## Metrics

The state file tracks operational metrics:

- **totalFallbacks** — How many times the circuit opened
- **totalRecoveries** — How many times it recovered to preferred model
- **lastFallbackAt** — ISO timestamp of last rate limit event
- **lastRecoveryAt** — ISO timestamp of last successful recovery

Query metrics with:
```powershell
$cb = Get-Content .squad/ralph-circuit-breaker.json | ConvertFrom-Json
Write-Host "Fallbacks: $($cb.metrics.totalFallbacks) | Recoveries: $($cb.metrics.totalRecoveries)"
```
