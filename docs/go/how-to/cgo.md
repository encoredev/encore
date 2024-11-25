---
seotitle: Build Go applications with cgo using Encore
seodesc: Learn how to build Go applications with cgo using Encore
title: Build with cgo
lang: go
---

Cgo is a feature of the Go compiler that enables Go programs to interface
with libraries written in other languages using C bindings.

By default, for improved portability Encore builds applications with cgo support disabled.

To enable cgo for your application, add `"build": {"cgo_enabled": true}` to your `encore.app` file.

For example:

```json
-- encore.app --
{
  "id": "my-app-id",
  "build": {
    "cgo_enabled": true
  }
}
```

With this setting Encore's build system will compile the application using an Ubuntu builder image
with gcc pre-installed.

## Static linking

To keep the resulting Docker images as minimal as possible, Encore compiles applications with static linking.
This happens even with cgo enabled. As a result the cgo libraries you use must support static linking.

In some cases, you may need to add additional linker flags to properly work with static linking of cgo libraries.
See the [official cgo docs](https://pkg.go.dev/cmd/cgo) for more information on how to do this.
