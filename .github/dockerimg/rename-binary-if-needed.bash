#!/usr/bin/env bash
set -eo pipefail

# Check if `encore-nightly`, `encore-beta` or `encore-develop` are present, and if one of them are, rename it to `encore`.
for binary in encore-nightly encore-beta encore-develop; do
  if [ -f "/encore/bin/$binary" ]; then
    echo "Renaming $binary to encore..."
    mv /encore/bin/$binary /encore/bin/encore
  fi
done

# Sanity check that /ecore/bin/encore exists.
if [ ! -f "/encore/bin/encore" ]; then
  echo "ERROR: /encore/bin/encore does not exist. Did you mount the Encore binary directory to /encore/bin?"
  exit 1
fi
