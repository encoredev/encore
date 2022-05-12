---
title: Environments
subtitle: Single cloud, multi cloud, or hybrid
---

When using Encore to build applications you create one or more *environments*.
Each environment is an isolated, fully working instance of your backend.

With Encore you can create as many or as few environments as you wish,
all with the click of a button.

## Creating environments

To create an environment for your app, open [Your app](https://app.encore.dev) and go to the **Environments** page,
then click on `Create env` in the top right.

There you can pick a name, and decide if you want a production
or development environment (see [Environment Types](#environment-types) below).

Choose how you would like to deploy to the environment (either on pushing
to a Git branch or manually triggered), and whether or not you want to manually approve infrastructure provisioning or simply let Encore handle it.

Finally, select which cloud provider to deploy to (see [Cloud Providers](#cloud-providers) below),
and click `Create`. That's it!

## Environment Types

Encore offers two types of environments: **Production** and **Development**.
They differ in the type of infrastructure that is provisioned.

Production environments are provisioned for maximum reliability, availability and scalability.
Databases are provisioned as proper, managed databases with automatic backups.
Your backend code runs with auto-scaling to match your traffic requirements.

Development environments are provisioned for simplicity, cost efficiency and speed.
The databases are provisioned with persistent disks using Kubernetes, to offer
reasonable durability and scalability, suitable for the most development needs.

## Cloud Providers

Encore supports deploying your application to any of the major cloud providers,
as well as using Encore's own cloud (internally deployed using GCP), using your own cloud account.

This gives you enormous flexibility, letting you use Encore for improving your productivity
while maintaining the existing trust relationship you have with your cloud provider of choice.
This functionality also lets you easily deploy a hybrid or multi-cloud application, if desired.

### Provisioning infrastructure

When deploying to an external cloud, Encore will add a preliminary deployment phase
to provision the necessary infrastructure based on what your app needs.
This is computed with static analysis using the [Encore Application Model](/docs/application-model).

For certain infrastructure resources, you may be asked to tell Encore a bit more about the performance requirements
you have. This lets Encore provision appropriately sized infrastructure for your needs.
*(This is only necessary the first time you add a new infrastructure component, and only for Production environments.)*
