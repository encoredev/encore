#!/bin/bash

# Set environment variables for development build
# Use our custom runtimes but the existing Go runtime
export ENCORE_RUNTIMES_PATH="/Users/jdaily/Repositories/encoredev/encore/runtimes"
export ENCORE_GOROOT="/opt/homebrew/Cellar/encore/1.48.4/libexec/encore-go"

# Run the custom encore binary with all arguments passed through
exec "/Users/jdaily/Repositories/encoredev/encore/encore-dev" "$@" 