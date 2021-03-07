#!/usr/bin/env bash
set -e

if [ -z "$ENCORE_GOROOT" ]; then
    echo "ENCORE_GOROOT must be set"
    exit 1
elif [ -z "$ENCORE_ROOT_FINAL" ]; then
    echo "ENCORE_ROOT_FINAL must be set"
    exit 1
elif [ -z "$ENCORE_VERSION" ]; then
    echo "ENCORE_VERSION must be set"
    exit 1
elif [ -z "$GOOS" ]; then
    echo "GOOS must be set"
    exit 1
elif [ -z "$GOARCH" ]; then
    echo "GOARCH must be set"
    exit 1
fi

export key="${GOOS}_${GOARCH}"
export dst="dist/$key"

case "$key" in
  "darwin_arm64")
    TARGET="arm64-apple-macos11"
    ;;
  "darwin_amd64")
    TARGET="x86_64-apple-macos10.12"
    ;;
  *)
    echo "unsupported architecture $key"
    exit 1
esac

export LDFLAGS="--target=$TARGET"
export CFLAGS="-O3 --target=$TARGET"

# Create our build destination
if [ -e "$dst" ]; then
  rm -r "$dst"
fi
mkdir -p "$dst"
mkdir "$dst/bin"

# Build the dash app
(cd cli/daemon/dash/dashapp && npm install && npm run build)

# Build encore and git-remote-encore
CGO_ENABLED=1 go build \
  -ldflags="-X 'encr.dev/cli/internal/env.encoreRoot=${ENCORE_ROOT_FINAL}' -X 'main.Version=${ENCORE_VERSION}'" \
  -o "$dst/bin/." \
  ./cli/cmd/encore

go build -o "$dst/bin/." ./cli/cmd/git-remote-encore

# Copy encore-go and runtime
cp -r "$ENCORE_GOROOT" "$dst/encore-go"
cp -r "compiler/runtime" "$dst/runtime"

tar -czf "dist/encore-${ENCORE_VERSION}-${key}.tar.gz" -C "$dst" .