VERSION="$(git describe --tags --always --abbrev=0 --match='v[0-9]*.[0-9]*.[0-9]*' 2> /dev/null | sed 's/^.//')"

ARCH="$(uname -m | tr '[:upper:]' '[:lower:]')"

if [ "$ARCH" = "x86_64" ]; then
  ARCH="amd64"
fi

LDFLAGS=(
  "-X 'encr.dev/internal/version.Version=${VERSION}'"
  "-X 'main.defaultGitRemoteName=local'"
  "-X 'main.defaultGitRemoteURL=ssh://localhost:9040/'"
  "-X 'encr.dev/internal/conf.defaultPlatformURL=http://localhost:9000'"
  "-X 'encr.dev/internal/conf.defaultConfigDirectory=encoredev'"
  "-X 'encr.dev/internal/env.alternativeEncoreRuntimePath=${ENCORE_ORG}/encore/runtime'"
  "-X 'encr.dev/internal/env.alternativeEncoreGoPath=${ENCORE_ORG}/go/dist/$(uname -s | tr '[:upper:]' '[:lower:]')_$ARCH/encore-go'"
)

go build -tags dev_build -o $HOME/bin/encoredev -ldflags="${LDFLAGS[*]}" ./cli/cmd/encore
