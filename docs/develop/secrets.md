---
title: Store API keys and secrets
subtitle: Simply storing secrets securely
---

Wouldn't it be nice to store API keys, database passwords, and private keys directly in the source code?

Of course we can’t do that &ndash; it's horrifyingly insecure!
Unfortunately it's also [very common](https://www.ndss-symposium.org/ndss-paper/how-bad-can-it-git-characterizing-secret-leakage-in-public-github-repositories/).

So why does it happen? Because storing secrets securely used to be quite annoying.
Fortunately, Encore makes it easy.

## Defining secrets

With Encore you define secrets directly in your code:

```go
var secrets struct {
    SSHPrivateKey string    // ed25519 private key for SSH server
    GitHubAPIToken string   // personal access token for deployments
    // ...
}
```

<Callout type="important">

The variable must be an unexported struct named `secrets`, and all
the fields must be of type `string` like you see above.

</Callout>

Then, you can set the secret value using `encore secret set --<dev|prod> <name>`.
For example, `encore secret set --prod SSHPrivateKey`.

The values are stored safely using HashiCorp Vault, and delivered securely directly
to your production environment.

You can also set secret values for your development environments (including local development),
using `encore secret set --dev GitHubAPIToken`.

### Using secrets

Once you've provided values for all the secrets, you can just use them in your program
like a regular variable. For example:

```go
func callGitHub(ctx context.Context) {
    req, _ := http.NewRequestWithContext(ctx, "GET", "https:///api.github.com/user", nil)
    req.Header.Add("Authorization", "token " + secrets.GitHubAPIToken)
    resp, err := http.DefaultClient.Do(req)
    // ... handle err and resp
}
```

Secret keys are globally unique for your whole application; if multiple services use the same
secret name they both receive the same secret value at runtime.