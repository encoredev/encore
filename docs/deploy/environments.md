---
seotitle: Environments – Creating local, preview, and prod environments
seodesc: Learn how to create all the environments you need for your backend application, local, preview, testing and production. Here's how you keep them in sync!
title: Environments
subtitle: The environments you want, with none of the work
---

Encore makes it simple to create the environments you need to build your application with confidence: local, preview, testing, and production.
Each environment is an isolated fully working instance of your backend, automatically provisioned by Encore.

Environments always stay in sync, as they are created based on the needs of your application, using the [Encore Application Model](/docs/introduction#meet-the-encore-application-model). Environments are provisioned using contextually appropriate [infrastructure](/docs/deploy/infra) depending on the type of environment.

## Creating environments

To create an environment for your app, open your app in the [Cloud Dashboard](https://app.encore.dev) and go to the **Environments** page,
then click on `Create env` in the top right.

There you can pick a name, and decide if you want a **production**
or **development environment** (see [Environment Types](#environment-types) below).

You can also choose how you would like to **trigger deploys** for the environment (either by pushing
to a Git branch or manually), and if you want to **manually approve infrastructure changes**.

Finally, select which **cloud provider** you want to deploy to (see the [Cloud Providers](/docs/deploy/own-cloud) documentation to learn more) and decide which type of **process allocation** you want: should all services be deployed to one process or separately?

Click `Create` and you're done!

![Creating an environment](/assets/docs/createenv.png "Creating an environment")

## Environment Types

Encore has four types of environments:
- `production`
- `development`
- `preview`
- `local`

Some environment types differ in how infrastructure is provisioned:
- `preview` environments are provisioned in Encore Cloud and are optimized to be cost-efficient and fast to provision.
- `local` is provisioned by Encore's CLI using local versions of infrastructure.

Aside from determining infrastructure, environment type is also used for [Secrets management](/docs/develop/secrets).

### Local environment

When you've installed the [Encore CLI](/docs/install), you start your local environment by simply running `encore run`.
This builds and tests your application, and provisions all the necessary infrastructure to run your application locally (see the [infra docs](/docs/deploy/infra#local-development) to learn exactly how local infrastructure is provisioned).

By default, the local environment runs on `http://localhost:4000`.

### Preview environments

When you [connect your application to GitHub](/docs/how-to/github), Encore will automatically provision ephemeral Preview Environments for each Pull Request. See the [Preview Environments documentation](/docs/deploy/preview-environments) to learn more.

#### Frontend Collaboration

Preview Environments make it really easy to collaborate and test changes with your frontend.
Just update your frontend API client to point to the `pr:#` environment.
This is a one-line change since your API client always specifies the environment name, e.g. `https://<env>-<my-app>.encr.app/`.

If your pull request makes changes to the API, you can [generate a new API client](/docs/develop/client-generation)
for the new backend API using `encore gen client --env=pr:72 --lang=typescript my-app`

### Cloud environments

Encore makes it simple to create multiple cloud environments using different cloud providers, by [connecting your cloud account](/docs/deploy/own-cloud). Cloud environments can be created as `Development`, or `Production`, depending on your use case (see the [infra docs](/docs/deploy/infra#production-infrastructure) to learn exactly what infrastructure is provisioned in each cloud).

#### Process Allocation

Just because you want to deploy each service separately in some environments, doesn't mean you want to do it in all environments.

Handily, Encore lets you decide how you want to deploy your services for cloud environments. You don't need to change a single line of code.

When you create an environment, you can decide which process allocation you want for that environment.

<img src="/assets/docs/microservices-process-allocation.png" title="Microservices - Process Allocation" />