---
seotitle: Deploying your Encore application is as simple as git push
seodesc: Learn how to deploy your backend application built with Encore with a single command, while Encore manages your entire CI/CD process.
title: Deploying Applications with Encore
subtitle: Encore comes with built-in CI/CD and integrates with GitHub
---

Encore simplifies the deployment process, making it as straightforward as a `git push`. Encore's built-in integration with Git and GitHub, automated CI/CD pipeline, and automatic provisioning of [Preview Environments](/docs/deploy/preview-environments) and [cloud infrastructure](/docs/deploy/infra), is designed to speed up development and remove manual steps.

## Setting Up Your Encore Application 

1. **Create your Application**: If you haven't already, create an application using the Encore CLI. This automatically creates a new git repository managed by Encore.

```shell
$ encore app create
```

2. **Integrate with GitHub (Optional)**: If you prefer to use GitHub, you can [integrate your app with GitHub](/docs/how-to/github). This way, you can push code to GitHub, which triggers Encore's deployment process. This is especially handy for teams as it enables collaborative development, version control, and other GitHub functionality.

## Deploying Your Application

With Encore, deploys are triggered simply by pushing changes to the connected Git repository.

- If you are using Encore's Git, run the following command to deploy your application:

```shell
$ git push encore
```

- If you are using GitHub, a standard `git push` to your repository will work:

```shell
$ git push origin
```

In both scenarios, this will trigger Encore's built-in CI/CD pipeline. This includes building your application, running tests, provisioning the necessary infrastructure, and deploying your application.

### Configure deploy trigger

When using GitHub, you can configure Encore to automatically trigger deploys when you push to a specific branch name.

To configure which branch name is used to trigger deploys, open your app in the [Cloud Dashboard](https://app.encore.dev) and go to the **Overview** page for your intended environment. Click on **Settings** and then in the section **Branch Push** configure the `Branch name`  and hit save.

### Preview Environments

When you connect your GitHub account and push changes to a pull request, Encore will automatically create a [Preview Environment](/docs/deploy/preview-environments). This is a fully functional, isolated environment where you can test your application as it would run in production. This environment runs in Encore's free development cloud, giving you an efficient way to validate your changes before they are merged and deployed to the primary environment.

## Custom build settings

You can override certain aspects of the CI/CD process in the `encore.app` file:

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
            "bundle_source": false
        }
    }
}
```
