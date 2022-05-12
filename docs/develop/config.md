---
title: Configuration
subtitle: When you want to change something, but not everywhere
---

As developers, we often find ourselves in a situation where we need to change something in our codebase, but we don't
want to change it everywhere the code is deployed. This is where configuration files come in, allowing us to define the
default behaviour of our applications, but then allow us to override these values for other environments.

Using Encore's [Application metadata API](https://pkg.go.dev/encore.dev/#AppMetadata) we can query the runtime to find
out what [environment](/docs/environments) the application is running in and load the appropriate configuration files into memory.

<Callout type="important">

For sensitive data use Encore's [secrets management](/docs/develop/secrets) functionality instead of configuration.

</Callout>
<br />

## Example using Go Embed

Using Go's ability to [embed files](https://pkg.go.dev/embed) into the compiled binary, we could embed two JSON files
into the binary, one for when we are running locally and one for when we're running in the cloud. Using an `init` function
we can check what type of environment we're running in and unmarshal the appropriate configuration file into our `Cfg`
variable.

```go
package config

import (
    _ "embed" // required to enable go:embed support
    "encoding/json"

    "encore.dev"
)

var (
    //go:embed env_cloud.json
    cloudConfig []byte

    //go:embed env_local.json
    localConfig []byte
)

// Our configuration structure which matches our two JSON files
var Cfg struct {
    Example string `json:"example"`
}

// Parse the configuration during startup.
func init() {
    // Load the appropriate configuration file based on the environment we're in.
    configToLoad := cloudConfig
    if  encore.Meta().Environment.Type == encore.EnvLocal {
        configToLoad = localConfig
    }
    if err := json.Unmarshal(configToLoad, &Cfg); err != nil {
        panic("unable to load config: " + err.Error())
    }
}
```
