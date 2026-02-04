# Encore CLI Local HTTPS Flag Feasibility & Implementation Plan

This document outlines the feasibility and a pragmatic implementation plan for adding a local HTTPS flag to the Encore CLI.

## 1) Findings: Where to hook in

- **`cli/cmd/encore/run.go`**: Defines the `encore run` command and its flags. This is the primary entry point for a developer running their application locally. The new `--https` flag will be added here.
- **`cli/cmd/encore/daemon.go`**: Defines the `encore daemon` command. The `run` command communicates with the daemon to start and manage the application.
- **`proto/encore/daemon/daemon.proto`**: The protobuf definition for the communication between the CLI and the daemon. The `RunRequest` message will be modified to include the new HTTPS flag.
- **`cli/daemon/run/run.go`**: Contains the logic for running an application. The `http.Server` is created here, and this is where the switch from `http.Serve` to `http.ListenAndServeTLS` will happen.
- **`cli/cmd/encore/daemon/daemon.go`**: The main entry point for the daemon. It initializes all the listeners and services.
- **`pkg/svcproxy/svcproxy.go`**: The reverse proxy used to route requests to different services. The `Rewrite` function will need to be updated to handle the `https` scheme.
- **`cli/daemon/dash/server.go`**: The server for the developer dashboard. It uses a reverse proxy and a websocket, both of which will need to be updated to support HTTPS.

## 2) Current local serving model

The Encore CLI's local serving model is based on a central daemon process that manages various services, each listening on a different port. The `encore run` command starts an application, which listens on a specified port (defaulting to 4000). A reverse proxy (`svcproxy`) is used to route requests to the correct service. The developer dashboard is also served via a reverse proxy.

```ascii
+------------------+
|   Encore CLI     |
| (`encore run`)   |
+------------------+
         |
         | (gRPC)
         v
+------------------+
|  Encore Daemon   |
|------------------|
| - run.Manager    |
| - svcproxy       |
| - dash.Server    |
| - ... (other     |
|   services)      |
+------------------+
         |
         | (HTTP)
         v
+------------------+
|  User Application|
| (listens on :4000)|
+------------------+
```

Currently, all communication is over HTTP. There is no built-in support for TLS in the local development environment.

## 3) HTTPS feasibility

It is **feasible** to add local HTTPS support to the Encore CLI. The architecture is modular enough to accommodate the necessary changes. The main challenges are:

- **Hardcoded `http://` schemes**: There are numerous places in the code where URLs are constructed with a hardcoded `http://` prefix. These will all need to be updated to be scheme-aware.
- **Certificate Management**: A strategy for generating, storing, and trusting local development certificates needs to be implemented.

There are no major blockers that would prevent the implementation of this feature.

## 4) Proposed flag(s) and config plumbing

- **CLI Flag**: A new boolean flag `--https` will be added to the `encore run` command in `cli/cmd/encore/run.go`. It will default to `false`.
- **Protobuf**: The `RunRequest` message in `proto/encore/daemon/daemon.proto` will be updated to include the new flag:

  ```protobuf
  message RunRequest {
    // ... existing fields
    optional bool https = 13;
  }
  ```

- **Config Propagation**: The `--https` flag will be passed from the `encore run` command to the daemon via the `RunRequest`. The daemon will then use this flag to configure the listeners and services accordingly.

## 5) Certificate approach

The recommended approach is to **auto-generate self-signed certificates**. This provides a good balance of security and ease of use for local development.

- **Certificate Generation**: A new utility package, for example `cli/internal/tlsutil`, will be created to handle certificate generation using the `crypto/x509` and `crypto/tls` packages.
- **Storage**: The generated certificate and key will be stored in Encore's configuration directory, for example `~/.config/encore/certs/`.
- **Trust**: The user will be responsible for trusting the self-signed certificate. Instructions on how to do this for different operating systems and browsers will be provided in the documentation.

An alternative approach would be to integrate with `mkcert`, but this would add an external dependency and complexity that is not necessary for a minimal viable product.

## 6) Implementation plan

### PR-1: Minimal Viable HTTPS

- Add the `--https` flag to the `encore run` command.
- Update the `RunRequest` protobuf message.
- Create the `tlsutil` package for certificate generation.
- In `cli/daemon/run/run.go`, if the `--https` flag is true, generate the certificate and key, and use `http.ListenAndServeTLS` instead of `http.Serve`.

### PR-2: URL Scheme Propagation

- Create a helper function that returns the appropriate scheme (`http` or `https`) based on the `--https` flag.
- Go through the codebase and replace all hardcoded `http://` URLs with the scheme-aware helper function. This will include updating the `DashBaseURL` in `run.Manager` and the `Rewrite` function in `pkg/svcproxy/svcproxy.go`.

### PR-3: Polish and Documentation

- Update the developer dashboard to use `wss://` for the websocket connection when HTTPS is enabled.
- Add documentation on how to use the `--https` flag and how to trust the self-signed certificate.

## 7) Test plan

- **Unit Tests**: Add unit tests for the `tlsutil` package and the scheme-aware URL construction helper function.
- **Integration Tests**: Create an integration test that starts the daemon with the `--https` flag, makes a request to an HTTPS endpoint, and verifies that the request is successful.
- **Cross-platform Testing**: Manually test the feature on macOS, Linux, and Windows to ensure that it works as expected.

## 8) Patch sketch

**`cli/cmd/encore/run.go`**

```go
// ...
var https bool

func init() {
    // ...
    runCmd.Flags().BoolVar(&https, "https", false, "Enable HTTPS for local development")
}

func runApp(appRoot, wd string) {
    // ...
    daemon.Run(ctx, &daemonpb.RunRequest{
        // ...
        Https: https,
    })
}
```

**`proto/encore/daemon/daemon.proto`**

```protobuf
message RunRequest {
  // ...
  optional bool https = 13;
}
```

**`cli/daemon/run/run.go`**

```go
func (r *Run) start(ctx context.Context) error {
    // ...
    if r.Params.Https {
        certFile, keyFile, err := tlsutil.GenerateCert()
        if err != nil {
            return err
        }
        go func() {
            if err := srv.ListenAndServeTLS(certFile, keyFile); !errors.Is(err, http.ErrServerClosed) {
                r.log.Error().Err(err).Msg("could not serve")
            }
        }()
    } else {
        go func() {
            if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
                r.log.Error().Err(err).Msg("could not serve")
            }
        }()
    }
    // ...
}
```

**`pkg/svcproxy/svcproxy.go`**

```go
func (p *SvcProxy) createReverseProxy(what, name string, listener netip.AddrPort, scheme string) *httputil.ReverseProxy {
    return &httputil.ReverseProxy{
        // ...
        Rewrite: func(request *httputil.ProxyRequest) {
            request.Out.URL.Scheme = scheme
            // ...
        },
    }
}
```
