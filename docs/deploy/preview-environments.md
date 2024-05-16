---
seotitle: Preview Environments – Temporary dev environments per Pull Request 
seodesc: Learn how to use Encore to activate automatic Preview Environments for every Pull Request to simplify testing and collaborating.
title: Preview Environments
subtitle: Accelerate development with isolated test environments for each Pull Request
---

When you [connect your application to GitHub](/docs/how-to/github), Encore will begin to automatically provision ephemeral Preview Environments for each Pull Request. Preview Environments are fully functional, isolated, environments where you can test your application as it would run in production.

Preview Environments are named after the pull request, for example PR #72 creates a Preview Environment named `pr:72` with the API base url `https://pr72-$APP_ID.encr.app`.

You can also view the environment in the Cloud Dashboard, where the url will be `https://app.encore.dev/$APP_ID/envs/pr:72`.

## Preview Infrastructure

Preview Environments run in Encore's free development cloud, removing the need to manually manage and pay for your own sandbox environments. This gives you a cost-efficient way to validate your changes before they are merged and deployed to the [primary environment](/docs/deploy/environments#primary-environment).

See the [infra docs](/docs/deploy/infra#preview-environments) if you're curious about exactly how Preview Environments are provisioned.

![Preview environment linked in GitHub](/assets/docs/ghpreviewenv.png "Preview environment linked in GitHub")

## Populate databases with data automatically

Encore Cloud, and Encore managed environmens on AWS and GCP, can be provisioned using [Neon](/docs/deploy/neon) as the database provider.

Neon is a serverless postgres provider that supports [database branches](https://neon.tech/docs/introduction/branching), which work similar to branches in your code.
Branches enable you to automatically seed your Preview Environments with test data by branching off a populated database, e.g. the database in a staging environment. 

To configure which branch to use for PR environments, head to Encore's Cloud Dashboard > (Select your app) > App Settings > Preview Environments 
and select the environment with the database you want to branch from. Hit save and you're all done.

Keep in mind that you can only branch from environments that use Neon as the database provider; this is the default for Encore Cloud environments, but is a configurable option when creating AWS and GCP environments.

<img src="/assets/docs/pr-neon.png" title="Use Neon for PR environments" className="mx-auto"/>


## Frontend Collaboration

Preview Environments make it really easy to collaborate and test changes with your frontend. Just update your frontend API client to point to the `pr:#` environment.
This is a one-line change since your API client always specifies the environment name, e.g. `https://<env>-<my-app>.encr.app/`.

If your pull request makes changes to the API, you can [generate a new API client](/docs/develop/client-generation)
for the new backend API using `encore gen client --env=pr:72 --lang=typescript my-app`
