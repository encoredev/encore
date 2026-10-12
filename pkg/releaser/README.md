# releaser

The build machinery behind the [Latest Release workflow](../../.github/workflows/latest-release.yml)

Two commands, run in sequence by the workflow:

- `cmd/build` compiles the artifacts for the platforms given in `R_BUILD` and uploads them to
  `encore-releases2` under a prefix (`R_RELEASE_PREFIX`), in the layout the releaser uses:

  ```
  gs://encore-releases2/<prefix>/
    npmpkg-encore-dev.tar.gz      the encore.dev npm package (npm pack output)
    encore-go-runtime.tar.gz      runtimes/go
    tsparser-wasm/                wasm-pack output for the tsparser wasm crate
    <os>-<arch>/
      bin/                        encore, tsparser-encore, tsbundler-encore, git-remote-encore
      encore-runtime.node         the JS runtime native module (+ .sha256)
      supervisor-encore           (+ .sha256)
  ```

  The releaser keeps the supervisor and the checksums in a separate `encore-optional` bucket, from
  which the CLI downloads them on demand (`encore-optional/encore/<version>/<os>-<arch>/`); here
  they sit next to the other per-platform artifacts instead. The prefix defaults to
  `encore/<version>`; the workflow builds `main` under `latest/<commit sha>` with version
  `v0.0.0-develop+<commit sha>`. A develop-channel CLI never downloads on demand anyway (it expects
  local development builds), so nothing reads those two files yet.

- `cmd/finalize-release` downloads those artifacts, adds the patched Go toolchain from
  `encore-go/<version>/` in `encore-releases2` (the version named by `encore-go/latest`, or
  `R_ENCORE_GO_VERSION`), and uploads one distribution tarball per platform next to the build
  artifacts:

  ```
  gs://encore-releases2/<prefix>/encore-<os>_<arch>.tar.gz   (+ .sha256)
  ```

  Each tarball contains `bin/`, `runtimes/go`, `runtimes/js` and `encore-go/`, the same layout the
  install script and the Docker image expect.

- `cmd/update-latest` then points `gs://encore-releases2/latest/COMMIT` (one line, the sha) at the
  commit, so tooling can resolve the newest build of `main` without knowing its sha. It refuses to
  run for any prefix other than `latest/<that commit>`, so a proper release can't end up behind it.

## Toolchain

Each platform is built on a runner of its own OS, so the only cross-compiling is darwin/amd64 on an
Apple Silicon Mac (same SDK, `-arch x86_64`). What the build expects on the host:

| Host | Go binaries | Rust binaries |
|---|---|---|
| Linux | `zig cc` targeting glibc 2.31 | `cargo zigbuild` targeting glibc 2.31; musl for the supervisor |
| macOS | Apple clang (Xcode command line tools) | `cargo build` for either Apple target |
| Windows | `zig cc` targeting mingw-w64 | `cargo build` with MSVC |

So Linux and Windows hosts need `zig` on `PATH` (the workflow pins 0.11.0, the version the release
builds have always linked with); Linux additionally needs `cargo-zigbuild`, and the host that builds
`tsparserwasm` needs `wasm-pack` and the one that builds `npmpkg` needs Node. Cross-compiling to
macOS from another OS is still possible by pointing `R_MACOS_SDK` at a macOS SDK, as the old
self-hosted runner did.

Both commands authenticate to GCS with Application Default Credentials. In the workflow those come
from `google-github-actions/auth`, which exchanges the run's OIDC token for GCP access through
Workload Identity Federation. The identity needs to write under the prefix and to read `encore-go/`
in `encore-releases2`.

Configuration is passed as `R_*` environment variables; see the package docs of each command.
