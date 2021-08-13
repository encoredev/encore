---
title: Bring your own cloud
subtitle: Better than your favorite beverage
---

Encore supports deploying your application to any of the major cloud providers,
as well as using Encore's own cloud (internally deployed using AWS), using your own cloud account.

This gives you enormous flexibility, letting you use Encore for improving your productivity
while maintaining the existing trust relationship you have with your cloud provider of choice.
This functionality also lets you easily deploy a hybrid or multi-cloud application, if desired.

<Callout type="important">

Note that Encore currently provisions a managed Kubernetes cluster when deploying to an external
cloud provider, which means the baseline costs are higher than when using Encore Cloud.

If you are evaluating Encore or aren't ready to scale to real production traffic yet,
we recommend starting with an Encore Cloud environment and later deploying an environment
with one of the major cloud providers.

</Callout>

## Amazon Web Services (AWS)

To deploy to AWS, you must link your app to your AWS account.
Go to your app's settings page, and then click **Cloud Deploy**.

Follow the instructions to create an IAM Role, and then connect the role with Encore.
[Learn more in the AWS docs](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_create_for-user.html).

<Callout type="warning">

For your security, make sure to check `Require external ID` and specify the
external ID provided in the instructions.

</Callout>

## Microsoft Azure

To deploy to Azure, you must link your app to your Azure organization.
Go to your app's settings page, and then click **Cloud Deploy**.
Follow the instructions to connect your account.

Note that currently you cannot link Encore with a personal Microsoft account.
To work around this, create a user using Azure AD (with at least `Cloud application administrator` permissions)
and use this user to link with Encore.

## Google Cloud Platform (GCP)

Deploying to GCP is actively in development and is available for preview. Let us know if you wish to use it
during the preview and we'll enable it for your application.