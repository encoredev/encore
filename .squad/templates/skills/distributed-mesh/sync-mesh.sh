#!/bin/bash
# sync-mesh.sh — Materialize remote squad state locally
#
# Reads mesh.json, fetches remote squads into local directories.
# Run before agent reads. No daemon. No service. ~40 lines.
#
# Usage: ./sync-mesh.sh [path-to-mesh.json]
#        ./sync-mesh.sh --init [path-to-mesh.json]
# Requires: jq (https://github.com/jqlang/jq), git, curl

set -euo pipefail

# Handle --init mode
if [ "${1:-}" = "--init" ]; then
  MESH_JSON="${2:-mesh.json}"
  
  if [ ! -f "$MESH_JSON" ]; then
    echo "❌ $MESH_JSON not found"
    exit 1
  fi
  
  echo "🚀 Initializing mesh state repository..."
  squads=$(jq -r '.squads | keys[]' "$MESH_JSON")
  
  # Create squad directories with placeholder SUMMARY.md
  for squad in $squads; do
    if [ ! -d "$squad" ]; then
      mkdir -p "$squad"
      echo "  ✓ Created $squad/"
    else
      echo "  • $squad/ exists (skipped)"
    fi
    
    if [ ! -f "$squad/SUMMARY.md" ]; then
      echo -e "# $squad\n\n_No state published yet._" > "$squad/SUMMARY.md"
      echo "  ✓ Created $squad/SUMMARY.md"
    else
      echo "  • $squad/SUMMARY.md exists (skipped)"
    fi
  done
  
  # Generate root README.md
  if [ ! -f "README.md" ]; then
    {
      echo "# Squad Mesh State Repository"
      echo ""
      echo "This repository tracks published state from participating squads."
      echo ""
      echo "## Participating Squads"
      echo ""
      for squad in $squads; do
        zone=$(jq -r ".squads.\"$squad\".zone" "$MESH_JSON")
        echo "- **$squad** (Zone: $zone)"
      done
      echo ""
      echo "Each squad directory contains a \`SUMMARY.md\` with their latest published state."
      echo "State is synchronized using \`sync-mesh.sh\` or \`sync-mesh.ps1\`."
    } > README.md
    echo "  ✓ Created README.md"
  else
    echo "  • README.md exists (skipped)"
  fi
  
  echo ""
  echo "✅ Mesh state repository initialized"
  exit 0
fi

MESH_JSON="${1:-mesh.json}"

# Zone 2: Remote-trusted — git clone/pull
for squad in $(jq -r '.squads | to_entries[] | select(.value.zone == "remote-trusted") | .key' "$MESH_JSON"); do
  source=$(jq -r ".squads.\"$squad\".source" "$MESH_JSON")
  ref=$(jq -r ".squads.\"$squad\".ref // \"main\"" "$MESH_JSON")
  target=$(jq -r ".squads.\"$squad\".sync_to" "$MESH_JSON")

  if [ -d "$target/.git" ]; then
    git -C "$target" pull --rebase --quiet 2>/dev/null \
      || echo "⚠ $squad: pull failed (using stale)"
  else
    mkdir -p "$(dirname "$target")"
    git clone --quiet --depth 1 --branch "$ref" "$source" "$target" 2>/dev/null \
      || echo "⚠ $squad: clone failed (unavailable)"
  fi
done

# Zone 3: Remote-opaque — fetch published contracts
for squad in $(jq -r '.squads | to_entries[] | select(.value.zone == "remote-opaque") | .key' "$MESH_JSON"); do
  source=$(jq -r ".squads.\"$squad\".source" "$MESH_JSON")
  target=$(jq -r ".squads.\"$squad\".sync_to" "$MESH_JSON")
  auth=$(jq -r ".squads.\"$squad\".auth // \"\"" "$MESH_JSON")

  mkdir -p "$target"
  auth_flag=""
  if [ "$auth" = "bearer" ]; then
    token_var="$(echo "${squad}" | tr '[:lower:]-' '[:upper:]_')_TOKEN"
    [ -n "${!token_var:-}" ] && auth_flag="--header \"Authorization: Bearer ${!token_var}\""
  fi

  eval curl --silent --fail $auth_flag "$source" -o "$target/SUMMARY.md" 2>/dev/null \
    || echo "# ${squad} — unavailable ($(date))" > "$target/SUMMARY.md"
done

echo "✓ Mesh sync complete"
