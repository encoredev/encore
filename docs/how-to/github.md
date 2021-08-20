---
title: Integrate with GitHub
---

Encore applications are easy to integrate with GitHub for source code hosting.
To link your application to GitHub, head over to your app and open **App Settings &rarr; Git integration**
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
the environment name like `https://<my-app>.encoreapi.com/<env>`.

If your pull request makes changes to the API, you can also easily [generate an API client](/docs/how-to/integrate-frontend)
against the new backend API using `encore gen client --env=pr:72 --lang=typescript my-app`

## Setup Walkthrough

Open your application on [app.encore.dev](https://app.encore.dev), and click
the app id in the top left. Select **App Settings** and then navigate to **Git Integration**.

First link your account with GitHub. Grant access either to all repositories or only the one(s)
you want to link with Encore.

When you come back to Encore, click the **Link app to GitHub repository** button:

<img class="max-w-lg w-full mx-auto rounded-lg shadow-lg border border-gray-100" src="/assets/img/git-begin.png" />

In the popup, select the repository you would like to link:

<img class="max-w-lg w-full mx-auto rounded-lg shadow-lg border border-gray-100" src="/assets/img/git-modal.png" />

Click **Link** and you're done:

<img class="max-w-lg w-full mx-auto rounded-lg shadow-lg border border-gray-100" src="/assets/img/git-linked.png" />