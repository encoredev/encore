#!/usr/bin/env bash
set -eo pipefail

# If the ENCORE_AUTHKEY environment variable is set, log in with it.
if [ -n "$ENCORE_AUTHKEY" ]; then
  echo "Logging in to Encore using provided auth key..."
  encore auth login --auth-key "$ENCORE_AUTHKEY"
fi

# Run the encore command.
encore "$@"
