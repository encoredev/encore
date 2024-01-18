---
seotitle: Securely storing API keys and secrets
seodesc: Learn how to store API keys, and secrets, securely for your backend application. Encore's built in vault makes it simple to keep your app secure.
title: Storing Secrets and API keys
subtitle: Simply storing secrets securely
---

Wouldn't it be nice to store secret values like API keys, database passwords, and private keys directly in the source code?
Of course, we can’t do that &ndash; it's horrifyingly insecure!
(Unfortunately, it's also [very common](https://www.ndss-symposium.org/ndss-paper/how-bad-can-it-git-characterizing-secret-leakage-in-public-github-repositories/).)

Encore's built-in secrets manager makes it simple to store secrets in a secure way and lets you use them in your program like regular variables.

## Using secrets in your application

To use a secret in your application, first define it directly in your code by creating an unexported struct named `secrets`, where all fields are of type `string`. For example:

```go
var secrets struct {
    SSHPrivateKey string    // ed25519 private key for SSH server
    GitHubAPIToken string   // personal access token for deployments
    // ...
}
```

When you've defined secrets in your program, the Encore compiler will check that they are set before running or deploying your application. If a secret is not set, you will get a compilation error notifying you that a secret value is missing.

Once you've provided values for all secrets, you can just use them in your application like a regular variable. For example:

```go
func callGitHub(ctx context.Context) {
    req, _ := http.NewRequestWithContext(ctx, "GET", "https:///api.github.com/user", nil)
    req.Header.Add("Authorization", "token " + secrets.GitHubAPIToken)
    resp, err := http.DefaultClient.Do(req)
    // ... handle err and resp
}
```

<Callout type="info">

Secret keys are globally unique for your whole application. If multiple services use the same secret name they both receive the same secret value at runtime.

</Callout>

## Storing secret values

### Using the Cloud Dashboard

The simplest way to set up secrets is with the Secrets Manager in the Encore Cloud Dashboard. Open your app in [app.encore.dev](https://app.encore.dev), go to **Settings** in the main navigation, and then click on **Secrets** in the settings menu.

From here you can create secrets, save secret values, and configure different values for different environments.

<img src="/assets/docs/secrets.png" title="Encore's Secrets Manager"/>

### Using the CLI

If you prefer, you can also set up secrets from the CLI using:<br/> `encore secret set --type <types> <secret-name>`

`<types>` defines which environment types the secret value applies to. Use a comma-separated list of `production`, `development`, `preview`, and `local`. Shorthands: `prod`, `dev`, `pr`.

For example `encore secret set --type prod SSHPrivateKey` sets the secret value for production environments,<br/> and `encore secret set --type dev,preview,local GitHubAPIToken` sets the secret value for development, preview, and local environments.

In some cases, it can be useful to define a secret for a specific environment instead of an environment type.
You can do so with `encore secret set --env <env-name> <secret-name>`. Secret values for specific environments
take precedence over values for environment types.

You can use content files, for secrets, imagine you need a certificate for create a database connection example cockroachdb you can set crt.

```sh
encore secrets set --type dev,local Certification <  ~/.postgresql/root.crt
```

### Environment settings

Each secret can only have one secret value for each environment type. For example: If you have a secret value that's shared between `development`, `preview` and `local`, and you want to override the value for `local`, you must first edit the existing secret and remove `local` using the Secrets Manager in the [Cloud Dashboard](https://app.encore.dev). You can then add a new secret value for `local`. The end result should look something like the picture below.

<img src="/assets/docs/secretoverride.png" title="Overriding a secret in Encore's Secrets Manager"/>

## How it works: Where secrets are stored

When you store a secret Encore stores it encrypted using Google Cloud Platform's [Key Management Service](https://cloud.google.com/security-key-management) (KMS).

- **Production / Your own cloud:** When you deploy to production using your own cloud account on GCP or AWS, Encore provisions a secrets manager in your account (using either KMS or AWS Secrets Manager) and replicates your secrets to it. The secrets are then injected into the container using secret environment variables.
- **Local:** For local secrets Encore automatically replicates them to developers' machines when running `encore run`.
- **Development / Encore Cloud:** Environments on Encore's development cloud (running on GCP under the hood) work the same as self-hosted GCP environments, using GCP Secrets Manager.
