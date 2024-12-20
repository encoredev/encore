---
seotitle: Deploying your Encore application is as simple as git push
seodesc: Learn how to deploy your backend application built with Encore with a single command, while Encore manages your entire CI/CD process.
title: Deploying Applications with Encore Cloud
subtitle: Encore Cloud automates the deployment and infrastructure provisioning process
lang: platform
---

Encore Cloud simplifies the deployment and infrastructure provisioning process, making it as straightforward as pushing to a git repository, removing the need for manual steps.

## Deploying your application

### Step 1: Pre-requisites

Before deploying, ensure that you have created an **Encore application** and an **Encore Cloud account**.

If you haven't created one yet, you can do so by running the following command:

```shell
$ encore app create
```

You will be asked to create a free Encore Cloud account first and can then create a new Encore application.

### Step 2: Integrate with GitHub (Optional)

When you create an Encore application while logged into your Encore Cloud account in the CLI, Encore will automatically create a new Encore managed git repository.
If you are just trying out Encore Cloud, you can use this and skip the rest of this step.

For production applications, we recommend integrating with GitHub instead of the built-in Encore managed git:

**Integrating with GitHub:** Open your app in the **[Encore Cloud dashboard](https://app.encore.cloud/) > (Select your app) > App Settings > Integrations > GitHub**.
Click the **Connect Account to GitHub** button, which will open GitHub where you can grant access either to all repositories or only the specific one(s) you want to link with Encore Cloud.

For more information and details on how to configure different repository structures, [see the full docs on integrating with GitHub](/docs/platform/integrations/github).

Once you have integrated with GitHub, you can push code to GitHub to trigger Encore Cloud's deployment process. If you use Encore Cloud Pro, you will also get automatic [Preview Environments](/docs/platform/deploy/preview-environments) for each pull request.

### Step 3: Connect your AWS / GCP account (Optional)

If you want to deploy to your own cloud on AWS or GCP, you first need to connect your cloud account to Encore Cloud.

If you are just trying out Encore Cloud, you can skip this step and Encore Cloud will automatically deploy to an environment using Encore Cloud's free development hosting, subject to the [fair use limits](/docs/platform/management/usage).

**Connect your cloud account:** Open your app in the **[Encore Cloud dashboard](https://app.encore.cloud/) > (Select your app) > App Settings > Integrations > Connect Cloud**.

Learn more in the [connecting your AWS or GCP account docs](/docs/platform/infrastructure/own-cloud).

### Step 4: Deploying Your Application

To deploy your application, simply push your code to the connected Git repository.

- **If you are using Encore Cloud's managed git**: Run the following command to deploy your application:

```shell
$ git add -A .
$ git commit -m 'Commit message'
$ git push encore
```

- **If you are using GitHub:** Just a standard `git push` to your repository will work:

```shell
$ git add -A .
$ git commit -m 'Commit message'
$ git push origin
```

In both scenarios, this will trigger Encore Cloud's deployment process, consisting of the following phases:
* A build & test phase
* An infrastructure provisioning phase
* A deployment phase

These phases are combined into a unified entity called a *Rollout*.
A rollout represents the coordinated process of rolling out a specific version of an Encore application.
(We use the term *rollout* to disambiguate from the *deployment phase*.)

Once you've pushed your code, you can monitor the progress in the **[Encore Cloud dashboard](https://app.encore.cloud/) > (Select your app) > Deployments**.

### Integrating using Encore Cloud's API

You can also trigger a deployment using Encore Cloud's API, learn more in the [API reference](/docs/platform/integrations/api-reference).

## Configuring deploy trigger

When using GitHub, you can configure Encore Cloud to automatically trigger deploys when you push to a specific branch name.

To configure which branch name is used to trigger deploys, open your app in the [Encore Cloud dashboard](https://app.encore.cloud) and go to the **Overview** page for your intended environment. Click on **Settings** and then in the section **Branch Push** configure the `Branch name`  and hit save.

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
