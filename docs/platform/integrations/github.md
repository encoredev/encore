---
seotitle: Integrate your Encore application with GitHub
seodesc: Learn how to integrate your backend application with GitHub to get automatic Preview Environments for each Pull Request using Encore.
title: Integrate with GitHub
lang: platform
---

Encore applications are easy to integrate with GitHub for source code hosting.

To link your application to GitHub, open your application in the [Encore Cloud dashboard](https://app.encore.cloud), and click on **Settings** in the main navigation.
Then select **GitHub** in the settings menu.

Next, connect your account to GitHub by clicking the **Connect Account to GitHub** button. This will open GitHub where you can grant access either to all repositories or only the specific one(s) you want to link with Encore.

<img class="max-w-lg w-full mx-auto" src="/assets/img/git-connect.png" />

When you come back to Encore, click the **Link App to GitHub** button:

<img class="max-w-lg w-full mx-auto" src="/assets/img/git-begin.png" />

In the popup, select the repository you would like to link your app with:

<img class="max-w-lg w-full mx-auto" src="/assets/img/git-modal.png" />

Click **Link** and you're done! Encore will now automatically start building and running tests against
your Pull Requests, and provision Preview Environments for each Pull Request.

<img class="max-w-lg w-full mx-auto" src="/assets/img/git-linked.png" />

## Placing your Encore app in a monorepo sub-folder

If you already have a monorepo and want to place your Encore application in a sub-folder, you need to tell Encore which folder the `encore.app` file is in.

Do this by opening your app in the [Encore Cloud dashboard](https://app.encore.cloud) and go to **Settings** > **General**. Then in the **Root Directory** section, you specify the directory within your Git repository in which your `encore.app` file is located.

## Configure deploy trigger

When using GitHub, you can configure Encore to automatically trigger deploys when you push to a specific branch name.
To configure which branch name is used to trigger deploys, open your app in the [Encore Cloud dashboard](https://app.encore.cloud) and go to the **Overview** page for your intended environment. Click on **Settings** and then in the section **Branch Push** configure the `Branch name`  and hit save.

## Preview Environments for each Pull Request

Once you've linked your app with GitHub, Encore will automatically start building and running tests against
your Pull Requests.

Encore will also provision a dedicated Preview Environment for each pull request.
This environment works just like a regular development environment, and lets you test your changes
before merging.

Learn more in the [Preview Environments documentation](/docs/platform/deploy/preview-environments).

![Preview environment linked in GitHub](/assets/docs/ghpreviewenv.png "Preview environment linked in GitHub")
