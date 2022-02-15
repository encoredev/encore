---
title: Bring your own cloud
subtitle: Better than your favorite beverage
---

Encore supports deploying your application to any of the major cloud providers,
as well as using Encore's own cloud (internally deployed using GCP), using your own cloud account.

This gives you enormous flexibility, letting you use Encore for improving your productivity
while maintaining the existing trust relationship you have with your cloud provider of choice.
This functionality also lets you easily deploy a hybrid or multi-cloud application, if desired.

## Google Cloud Platform (GCP)

To deploy to GCP we provide a service account for each Encore application that you grant access
to provisioning a GCP Project and attaching a billing account to it.

To configure GCP deployments, head over to the Cloud Deploy page by going to
**[Your apps](https://app.encore.dev/) > (Select your app) > App Settings > Cloud Deploy**.

## Amazon Web Services (AWS)

Deploying to AWS is actively in development and is available for preview. Let us know if you wish to use it
during the preview and we'll enable it for your application.

<!--
Follow the instructions to create an IAM Role, and then connect the role with Encore.
[Learn more in the AWS docs](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_create_for-user.html).

<Callout type="warning">

For your security, make sure to check `Require external ID` and specify the
external ID provided in the instructions.

</Callout>
-->
