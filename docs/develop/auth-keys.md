---
seotitle: Auth Keys let you authenticate without a browser
seodesc: Learn how to use pre-authentication keys to authenticate without needing to sign in via a web browser. See how to setup reusable and ephemeral auth keys.
title: Generating Auth Keys
---

Pre-authentication keys (“auth keys” for short) let you authenticate the Encore CLI without needing to sign in via a web browser. This is most useful when setting up CI/CD pipelines.

<video autoPlay playsInline loop controls muted className="w-full h-full">
	<source src="/assets/docs/authkeys.mp4" className="w-full h-full" type="video/mp4" />
</video>

## Types of auth keys

- **Reusable Keys** for authenticating more than one machine.

- **Ephemeral Keys** - Machines authenticated by this key will be automatically logged out after one our.

<Callout type="important">

**Be very careful with reusable keys!** These can be very dangerous if stolen. They're best kept in a key vault product (eg. 1Password, LastPass) specifically designed for the purpose.

</Callout>

## Authentication

**Auth keys** authenticate a machine as the Encore app for which the key was generated. If Ada generates an auth key, and uses it to set up her CI/CD pipeline, then that machine is authenticated as Ada's Encore app.

## Generating a key

### Step 1: Generate an auth key

As an Encore user, visit the auth key page by going to **[Your apps](https://app.encore.dev/) > (Select your app) > App Settings > Auth Keys**.

A key can be both **reusable** and **ephemeral** at the same time (you can decide the combination based on your particular use case).

<Callout type="important">

**Don't forget to store your key!** Once generated, you will need to copy and store your key in a vault product (eg. 1Password, LastPass). We do not display the full contents of a key in our dashboard for security reasons.

</Callout>

This page also gives you the ability to revoke existing keys.

### Step 2: Authenticate with the auth key

Using the Encore CLI, you can authenticate with your newly generated key:

```shell
$ encore auth login --auth-key=ena_nEQIkfeM43t7oxpleMsIULbhbtLAbYnnLf1D
```

## Revoking a key

You can revoke a key simply by pressing the **Delete** button next to it. This will prevent any machines currently using it to authenticate to the Encore platform (regardless of the key type).
