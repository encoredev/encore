---
seotitle: Deploying your Encore application is as simple as git push
seodesc: Learn how to deploy your backend application built with Encore with a single command, while Encore manages your entire CI/CD process.
title: Deploying Applications with Encore Cloud
subtitle: Encore Cloud automates the deployment and infrastructure provisioning process
lang: platform
---

Encore Cloud simplifies deploying your application, making it as simple as pushing to a git repository, removing the need for manual steps.

## Deploying your application

### Step 1: Create account & application

Before deploying, ensure that you have an **Encore Cloud account** and have created an **Encore application**.

You can create both an account and an application by running the following command:

```shell
$ encore app create
```

You will be asked to create a free Encore Cloud account first, and then proceed to create a new Encore application.

#### Already created an application locally?

Follow these steps if you've already created an app and want to link it to an account on Encore Cloud:

**1. Ensure you are logged in with the CLI**

```bash
encore auth signup # If you haven't created an Encore Cloud account
encore auth login # If you've already created an Encore Cloud account
```

**2. Link your local app to Encore Cloud**

Run this command from you application's root folder:

```bash
encore app init
```

**3. Set up Encore's git remote to enable pushing directly to Encore Cloud**

Run this command from you application's root folder:

```bash
git remote add encore encore://<app-id>
```


### Step 2: Integrate with GitHub (Optional)

When creating an Encore application, Encore will automatically create a new Encore managed git repository. If you are just trying out Encore Cloud, you can use this and skip the rest of this step.

For production applications we recommend integrating with GitHub instead of using the built-in Encore managed git:

#### **Connecting your GitHub account**

Open your app in the **[Encore Cloud dashboard](https://app.encore.cloud/) > (Select your app) > App Settings > Integrations > GitHub**.
Click the **Connect Account to GitHub** button, which will open GitHub where you can grant access either to the relevant repositorie(s).

[See the full docs](/docs/platform/integrations/github) on integrating with GitHub to learn how to configure different repository structures.

Once connected to GitHub, pushing code will trigger deployments automatically. Encore Cloud Pro users get [Preview Environments](/docs/platform/deploy/preview-environments) for each pull request.

### Step 3: Connect your AWS / GCP account (Optional)

Deploy to your own cloud on AWS or GCP by connecting your cloud account to Encore Cloud.

If you're just trying out Encore Cloud, skip this step to deploy to a free development environment using Encore Cloud's hosting, subject to [fair use limits](/docs/platform/management/usage).

#### **Connecting your cloud account**

Open your app in the **[Encore Cloud dashboard](https://app.encore.cloud/) > (Select your app) > App Settings > Integrations > Connect Cloud**.

Learn more in the [connecting your cloud docs](/docs/platform/deploy/own-cloud).

### Step 4: Push to deploy

Deploy your application by pushing your code to the connected Git repository.

- **Using Encore Cloud's managed git**:

```shell
$ git add -A .
$ git commit -m 'Commit message'
$ git push encore
```

- **If you have connected your GitHub account:**

```shell
$ git add -A .
$ git commit -m 'Commit message'
$ git push origin
```

This will trigger Encore Cloud's deployment process, consisting of the following phases:
* A build & test phase
* An infrastructure provisioning phase
* A deployment phase

Once you've pushed your code, you can monitor the progress in the **[Encore Cloud dashboard](https://app.encore.cloud/) > (Select your app) > Deployments**.

## Configuring deploy trigger

When using GitHub, you can configure Encore Cloud to automatically trigger deploys when you push to a specific branch name.

To configure which branch name is used to trigger deploys, open your app in the [Encore Cloud dashboard](https://app.encore.cloud) and go to the **Overview** page for your intended environment. Click on **Settings** and then in the section **Branch Push** configure the `Branch name`  and hit save.

### Integrating using Encore Cloud's API

You can trigger deployments using Encore Cloud's API, learn more in the [API reference](/docs/platform/integrations/api-reference).

## Configuring custom build settings

If you want, you can override certain aspects of the CI/CD process in the `encore.app` file:

* The Docker base image to use when deploying
* Whether to build with Cgo enabled
* Whether to bundle the source code in the docker image (useful for [Sentry stack traces](https://docs.sentry.io/platforms/go/usage/serverless/))

Below are the available build settings configurable in the `encore.app` file,
with their default values:

```cue
{
    "build": {
        // Enables cgo when building the application and running tests
        // in Encore's CI/CD system.
        "cgo_enabled": false,

        // Docker-related configuration
        "docker": {
        	// The Docker base image to use when deploying the application.
        	// It must be a publicly accessible image, and defaults to "scratch".
            "base_image": "scratch",

            // Whether to bundle the source code in the docker image.
            // The source code will be copied into /workspace as part
            // of the build process. This is primarily useful for tools like
            // Sentry that need access to the source code to generate stack traces.
            "bundle_source": false,

            // The working directory to start the docker image in.
            // If empty it defaults to "/workspace" if the source code is bundled, and to "/" otherwise.
            "working_dir": ""
        }
    }
}
```
