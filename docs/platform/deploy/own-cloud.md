---
seotitle: Connect your cloud account to deploy to any cloud
seodesc: Learn how to deploy your backend application to all the major cloud providers (AWS or GCP) using Encore.
title: Connect your cloud account
subtitle: Whatever cloud you prefer is fine by us
lang: platform
---

Encore Cloud lets you deploy your application to any of the major cloud providers, using your own cloud account.
This lets you use Encore to improve your experience and productivity, while keeping the reliability of a major cloud provider.

Each [environment](/docs/platform/deploy/environments) can be configured to use a different cloud provider, and you can have as many environments as you wish.
This also lets you easily deploy a hybrid or multi-cloud application, as you see fit.

<Callout type="info">

Encore Cloud will provision infrastructure in your cloud account, but for safety reasons Encore Cloud does not automatically destroy infrastructure once it's no longer required. To do this, you need to manually approve the deletion of the infrastructure in your Encore Cloud dashboard.

This means if you disconnect your app from your cloud provider, or delete the environment
within Encore, you need to explicitly approve the deletion of the infrastructure in your Encore Cloud dashboard.

</Callout>

## Google Cloud Platform (GCP)

Encore Cloud provides a GCP Service Account for each Encore Cloud application, letting you grant Encore Cloud access to provision all the necessary infrastructure directly in your own GCP Organization account.

To find your app's Service Account email and configure GCP deployments, head over to the Connect Cloud page by going to the **[Encore Cloud dashboard](https://app.encore.cloud/) > (Select your app) > App Settings > Integrations > Connect Cloud**.

![Connect GCP account](/assets/docs/connectgcp.png "Connect GCP account")

### Troubleshooting

**I can't access/edit the `Policy for Domain restricted sharing` page**

To edit Organization policies, you need to have the `Organization Policy Administrator` role. If you don't have this role, you can ask your GCP Organization Administrator to grant you the necessary permissions.
If you're a GCP Organization Administrator, you can grant yourself the necessary permissions by following the steps below:

1. Go to the [IAM & Admin page](https://console.cloud.google.com/iam-admin/iam) in the GCP Console.
2. Find your user account in the list of members.
3. Click the pencil icon to edit your user account.
4. Add the `Organization Policy Administrator` role to your user account.
5. Click Save.

**I can't grant access to the Encore Cloud service account**

If you're unable to grant access to the Encore Cloud service account, you may have failed to add Encore Cloud to your `Domain restricted sharing` policy.
Make sure you've followed all the steps in the Connect Cloud page to add Encore Cloud to the policy.
If you're using several GCP accounts, make sure you're logged in with the correct account and that the correct organization is selected in the GCP Console.

**Encore Cloud returns "Could not find Organization ID"**

If you see this error message, it means that Encore Cloud was unable to connect to your GCP Organization. Make sure you've followed all the steps in the Connect Cloud page to grant Encore Cloud access to your GCP Organization.
If you're using several GCP accounts, make sure you're logged in with the correct account and that the correct organization is selected in the GCP Console.

Still having issues? Drop us an email at [support@encore.dev](mailto:support@encore.dev) or chat with us in the [Encore Discord](https://encore.dev/discord.

## Amazon Web Services (AWS)
To configure your Encore Cloud app to deploy to your AWS account, head over to the Connect Cloud page by going to the
**[Encore Cloud dashboard](https://app.encore.cloud/) > (Select your app) > App Settings > Integrations > Connect Cloud**.

Follow the instructions to create an IAM Role, and then connect the role with Encore Cloud.
[Learn more in the AWS docs](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_create_for-user.html).

![Connect AWS account](/assets/docs/connectaws.png "Connect AWS account")


<Callout type="warning">

For your security, make sure to check `Require external ID` and specify the
external ID provided in the instructions.

</Callout>

After connecting your app to AWS, you will be asked to choose which region you want Encore Cloud to provision resources in. [Learn more about AWS regions here](https://aws.amazon.com/about-aws/global-infrastructure/regions_az/).
