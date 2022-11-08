---
seotitle: Integrate your Encore application with GitHub
seodesc: Learn how to integrate your Go backend application with GitHub to get automatic Preview Environments for each Pull Request using Encore.
title: Integrate with GitHub
---

Encore applications are easy to integrate with GitHub for source code hosting.
To link your application to GitHub, open [Your app](https://app.encore.dev), then head to **Settings &rarr; GitHub**
and follow the instructions.

For a more detailed guide on linking with GitHub, see the [Setup Walkthrough](#setup-walkthrough) below.

## Pull Requests & Preview Environments

Once you've linked with GitHub, Encore will automatically start building and running tests against
your Pull Requests.

Encore will also provision a dedicated Preview Environment for each pull request.
This environment works just like a regular development environment, and lets you test your changes
before merging.

Preview Environments are named after the pull request, so PR #72 will create an environment named `pr:72`.

### Frontend Collaboration

When you integrate Encore with a frontend, Preview Environments makes it really easy to collaborate
and test your changes against the frontend. Just update your frontend API client to point to the
`pr:#` environment. This should just be a one-line change since your API client always specifies
the environment name like `https://<env>-<my-app>.encr.app/`.

If your pull request makes changes to the API, you can also easily [generate an API client](/docs/develop/client-generation)
against the new backend API using `encore gen client --env=pr:72 --lang=typescript my-app`

## Setup Walkthrough

Open your application on [app.encore.dev](https://app.encore.dev), and click on **Settings** in the main navigation.
Then select **GitHub** in the settings menu.

First connect your account to GitHub by clicking the **Connect Account to GitHub** button. This will open GitHub where you can grant access either to all repositories, or only the specific one(s) you want to link with Encore.

<img class="max-w-lg w-full mx-auto" src="/assets/img/git-connect.png" />

When you come back to Encore, click the **Link App to GitHub** button:

<img class="max-w-lg w-full mx-auto" src="/assets/img/git-begin.png" />

In the popup, select the repository you would like to link your app with:

<img class="max-w-lg w-full mx-auto" src="/assets/img/git-modal.png" />

Click **Link** and you're done! Encore will now automatically start building and running tests against
your Pull Requests, and provision Preview Environments for each Pull Request.

<img class="max-w-lg w-full mx-auto" src="/assets/img/git-linked.png" />
