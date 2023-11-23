---
seotitle: Connect your cloud account to deploy to any cloud
seodesc: Learn how to deploy your backend application to all the major cloud providers (AWS or GCP) using Encore.
title: Connect your cloud account
subtitle: Whatever cloud you prefer is fine by us
---

Encore lets you deploy your application to any of the major cloud providers, using your own cloud account.
This lets you use Encore to improve your experience and productivity, while keeping the reliability of a major cloud provider.

Each [environment](/docs/deploy/environments) can be configured to use a different cloud provider, and you can have as many environments as you wish.
This also lets you easily deploy a hybrid or multi-cloud application, as you see fit.

<Callout type="info">

Encore will provision infrastructure in your cloud account, but for safety reasons Encore does not destroy infrastructure once it's no longer required.

This means if you disconnect your app from your cloud provider, or delete the environment
within Encore, you need to manually remove the infrastructure that was created by Encore.

</Callout>

## Google Cloud Platform (GCP)

Encore provides a GCP Service Account for each Encore application, letting you grant Encore access to provision all the necessary infrastructure directly in your own GCP Organization account.

To find your app's Service Account email and configure GCP deployments, head over to the Connect Cloud page by going to Encore's **[Cloud Dashboard](https://app.encore.dev/) > (Select your app) > App Settings > Integrations > Connect Cloud**.

![Connect GCP account](/assets/docs/connectgcp.png "Connect GCP account")


## Amazon Web Services (AWS)
To configure your Encore app to deploy to your AWS account, head over to the Connect Cloud page by going to Encore's
**[Cloud Dashboard](https://app.encore.dev/) > (Select your app) > App Settings > Integrations > Connect Cloud**.

Follow the instructions to create an IAM Role, and then connect the role with Encore.
[Learn more in the AWS docs](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_create_for-user.html).

![Connect AWS account](/assets/docs/connectaws.png "Connect AWS account")


<Callout type="warning">

For your security, make sure to check `Require external ID` and specify the
external ID provided in the instructions.

</Callout>

After connecting your app to AWS, you will be asked to choose which region you want Encore to provision resources in. [Learn more about AWS regions here](https://aws.amazon.com/about-aws/global-infrastructure/regions_az/).
