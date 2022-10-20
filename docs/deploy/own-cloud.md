---
title: Connect your cloud account
subtitle: Whatever cloud you prefer is fine by us
---

Encore lets you deploy your application to any of the major cloud providers, using your own cloud account.
This lets you use Encore to improve your experience and productivity, while keeping the reliability of a major cloud provider.

Each [environment](/docs/deploy/environments) can be configured to use a different cloud provider, and you can have as many environments as you wish.
This functionality also lets you easily deploy a hybrid or multi-cloud application, as you see fit.

<Callout type="info">

Encore will provision infrastructure in your cloud account, but for safety reasons Encore does not destroy infrastructure
once it's no longer required.

This means if you disconnect your app from your cloud provider, or delete the environment
within Encore, you need to manually remove the infrastructure that was created by Encore.

</Callout>

## Google Cloud Platform (GCP)

Encore provides a GCP Service Account for each Encore application, letting you grant Encore access to provision all the necessary infrastructure directly in your own GCP Organization account.

To find your app's Service Account email and configure GCP deployments, head over to the Cloud Deploy page by going to
**[the Encore web platform](https://app.encore.dev/) > (Select your app) > App Settings > Integrations > Cloud Deploy**.

![Connect GCP account](/assets/docs/connectgcp.png "Connect GCP account")


## Amazon Web Services (AWS)
To configure your Encore app to deploy to your AWS account, head over to the Cloud Deploy page by going to
**[the Encore web platform](https://app.encore.dev/) > (Select your app) > App Settings > Integrations > Cloud Deploy**.

Follow the instructions to create an IAM Role, and then connect the role with Encore.
[Learn more in the AWS docs](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_create_for-user.html).

![Connect AWS account](/assets/docs/connectaws.png "Connect AWS account")


<Callout type="warning">

For your security, make sure to check `Require external ID` and specify the
external ID provided in the instructions.

</Callout>

After connecting your app to AWS, you will be asked to choose which region you want Encore to provision resources in. [Learn more about AWS regions here](https://aws.amazon.com/about-aws/global-infrastructure/regions_az/).

## Microsoft Azure

To configure for Azure deployments, head over to the Cloud Deploy page by going to
**[the Encore web platform](https://app.encore.dev/) > (Select your app) > App Settings > Integrations > Cloud Deploy**.

Start by filling in your Azure Tenant ID, then click on **Connect app to Azure**.
This will redirect you to Microsoft's Azure portal requesting that you grant permission for
Encore to perform actions on your account.

![Connect Azure account](/assets/docs/connectazure.png "Connect Azure account")

Once you have approved the permissions, you will be redirected back to Encore and in-place of the connect button you should see
**Connected to my-organisation.com**. This verifies that Encore has been able to successfully connect to your Azure account.

Next, go to [subscriptions in the Azure portal](https://portal.azure.com/#blade/Microsoft_Azure_Billing/SubscriptionsBlade)
and select the subscription that you want to deploy your Encore app into. Select **Access Control (IAM)** and click
**Add Role Assignment** and select the **Owner** role. Then under **Members** click on **Select Members** and search for
`Encore` (it will not appear in the list of members until you search by name), selecting it when it comes up.

Once the role has been assigned, you can continue to [create an Azure environment](/docs/deploy/environments#creating-environments). 

<Callout type="info">

Encore requires Owner permissions to your Azure subscription in order to provision resources in Azure.
This is because Encore creates custom roles with minimal permissions, and assigns these to provisioned resources. 
The owner level grants the necessary permissions required to manage and assign roles in Azure RBAC, and manage the resources
within the subscription.

</Callout>