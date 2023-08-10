---
seotitle: Preview Environments – Temporary dev environments per Pull Request 
seodesc: Learn how to use Encore to activate automatic Preview Environments for every Pull Request to simplify testing and collaborating.
title: Preview Environments
subtitle: Connect your Encore app with GitHub to activate Preview Environments
---

When you [connect your application to GitHub](/docs/how-to/github), Encore will automatically provision ephemeral Preview Environments for each Pull Request. This is a fully functional, isolated environment where you can test your application as it would run in production. This environment runs in Encore's free development cloud, giving you an efficient way to validate your changes before they are merged and deployed to the primary environment.

Preview Environments are named after the pull request, for example PR #72 creates a preview environment named `pr:72` with the API base url `https://pr72-$APP_ID.encr.app`.

You can also view the environment in the Cloud Dashboard, where the url will be `https://app.encore.dev/$APP_ID/envs/pr:72`.

See the [infra docs](/docs/deploy/infra#preview-environments) if you're curious about exactly how Preview Environments are provisioned.

![Preview environment linked in GitHub](/assets/docs/ghpreviewenv.png "Preview environment linked in GitHub")

### Frontend Collaboration

Preview Environments make it really easy to collaborate and test changes with your frontend.
Just update your frontend API client to point to the `pr:#` environment.
This is a one-line change since your API client always specifies the environment name, e.g. `https://<env>-<my-app>.encr.app/`.

If your pull request makes changes to the API, you can [generate a new API client](/docs/develop/client-generation)
for the new backend API using `encore gen client --env=pr:72 --lang=typescript my-app`
