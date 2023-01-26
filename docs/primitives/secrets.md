---
seotitle: Securely storing API keys and secrets
seodesc: Learn how to store API keys, and secrets, securely for your backend application. Encore's built in vault makes it simple to keep your app secure.
title: Storing Secrets and API keys
subtitle: Simply storing secrets securely
---

Wouldn't it be nice to store secret values like API keys, database passwords, and private keys directly in the source code?
Of course, we can’t do that &ndash; it's horrifyingly insecure!
(Unfortunately, it's also [very common](https://www.ndss-symposium.org/ndss-paper/how-bad-can-it-git-characterizing-secret-leakage-in-public-github-repositories/).)

Encore's built-in secrets manager makes it simple to store secrets in a secure way, and lets you use them in your program like regular variables.

When creating new environments, Encore automatically sets up secrets management using best practices for each cloud provider. See the [infrastructure documentation](/docs/deploy/infra#production-infrastructure) for more details.

## Defining secrets

With Encore you define secrets directly in your code by creating a struct:

```go
var secrets struct {
    SSHPrivateKey string    // ed25519 private key for SSH server
    GitHubAPIToken string   // personal access token for deployments
    // ...
}
```

<Callout type="important">

The variable must be an unexported struct named `secrets`, and all the fields must be of type `string` like you see above.

</Callout>

Then you set the secret value using `encore secret set --type <types...> <secret-name>`.

`<types>` defines which environment types the secret value applies to. Use a comma-separated list of `production`, `development`, `preview`, and `local`. Shorthands: `prod`, `dev`, `pr`.

For example `encore secret set --type prod SSHPrivateKey` sets the secret value for production environments,<br/> and `encore secret set --type dev,preview,local GitHubAPIToken` sets the secret value for development, preview, and local environments.

<Callout type="important">

There can only be one secret value for each environment type. For example, if you already have a secret value that's shared between `development`, `preview` and `local`
and you want to override the value for `local`, you must first edit the existing secret value and remove `local`. Only then can you define a new secret value
specifically for `local`. (Same goes for the other environment types.)

You can edit existing secret values on the [Encore web platform](https://app.encore.dev) under Settings > Secrets.

</Callout>

For certain use cases it can be useful to define a secret for a specific environment instead of an environment type.
You can do so with `encore secret set --env <env-name> <secret-name>`. Secret values for specific environments
take precedence over values for environment types.

The values are stored safely using [GCP's Key Management Service](https://cloud.google.com/security-key-management), and delivered securely directly to your application.

## Using secrets

Once you've provided values for all the secrets, you can just use them in your program like a regular variable. For example:

```go
func callGitHub(ctx context.Context) {
    req, _ := http.NewRequestWithContext(ctx, "GET", "https:///api.github.com/user", nil)
    req.Header.Add("Authorization", "token " + secrets.GitHubAPIToken)
    resp, err := http.DefaultClient.Do(req)
    // ... handle err and resp
}
```

Secret keys are globally unique for your whole application; if multiple services use the same secret name they both receive the same secret value at runtime.

<Callout type="info">

Once you've used secrets in your program, the Encore compiler will check that they are set before running or deploying your application.

</Callout>
