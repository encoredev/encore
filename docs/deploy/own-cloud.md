---
title: Bring your own cloud
subtitle: Better than your favorite beverage
---

Encore supports deploying your application to any of the major cloud providers,
as well as using Encore's own cloud (internally deployed using GCP), using your own cloud account.

This gives you enormous flexibility, letting you use Encore for improving your productivity
while maintaining the existing trust relationship you have with your cloud provider of choice.
This functionality also lets you easily deploy a hybrid or multi-cloud application, if desired.

Each [environment](/docs/deploy/environments) your application has can be configured to use a different cloud provider.

<Callout type="important">
Please note, that while Encore will provision infrastructure within your cloud account, for safety reasons Encore does not destroy infrastructure
once it's no longer required. This means if you disconnect your app from your cloud provider or delete the environment
within Encore, you will still have to manually remove the infrastructure that was created by Encore.
</Callout>

## Google Cloud Platform (GCP)

To deploy to GCP we provide a service account for each Encore application that you grant access
to provisioning a GCP Project and attaching a billing account to it.

To configure GCP deployments, head over to the Cloud Deploy page by going to
**[Your apps](https://app.encore.dev/) > (Select your app) > App Settings > Integrations > Cloud Deploy**.

## Amazon Web Services (AWS)
To configure for Azure deployments, head over to the Cloud Deploy page by going to
**[Your apps](https://app.encore.dev/) > (Select your app) > App Settings > Integrations > Cloud Deploy**. Follow the instructions to create an IAM Role, and then connect the role with Encore.
[Learn more in the AWS docs](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_create_for-user.html).

<Callout type="warning">

For your security, make sure to check `Require external ID` and specify the
external ID provided in the instructions.

</Callout>

After connecting your app to AWS, you will be asked to choose which region you want Encore to provision resources in. [Learn more about AWS regions here](https://aws.amazon.com/about-aws/global-infrastructure/regions_az/).

## Microsoft Azure

To configure for Azure deployments, head over to the Cloud Deploy page by going to
**[Your apps](https://app.encore.dev/) > (Select your app) > App Settings > Integrations > Cloud Deploy**. Then click on
**Connect app to Azure**. This will redirect you to Microsoft's Azure portal requesting that you grant permission for
Encore to perform actions on your account.

Once you have approved the permissions, you will be redirected back to Encore and in-place of the connect button you should see
**Connected to my-organisation.com**. This verifies that Encore has been able to successfully connect to your Azure account.

Next you need to go to your [subscriptions in the Azure portal](https://portal.azure.com/#blade/Microsoft_Azure_Billing/SubscriptionsBlade)
and select the subscription that you want to deploy your Encore app into. Select **Access Control (IAM)** and click
**Add Role Assignment** and select the **Owner** role. Then under **Members** click on **Select Members** and search for
`Encore` (it will not appear in the list of members until you search by name), selecting it when it comes up.

Once the role has been assigned, you can continue to [create an Azure environment](/docs/deploy/environments#creating-environments). 

### Why does Encore require Owner permissions to my Azure subscription?

When Encore provisions resources in Azure, we create custom roles with minimal permissions to be assigned to those resources. 
The owner level grants the permissions required to manage and assign roles in Azure RBAC as well as manage all the resources
within the subscription.
