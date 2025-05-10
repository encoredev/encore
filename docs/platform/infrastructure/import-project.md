---
seotitle: How to deploy your Encore application to an existing GCP project
seodesc: Learn how to easily import your existing GCP project and connect your Encore application to it.
title: Import an existing GCP project
subtitle: Using your pre-existing GCP project instead of provisioning a new one
lang: platform
---

# Overview

When deploying applications to your own cloud, Encore Cloud can provision all necessary infrastructure—including new GCP projects. However, if you already have a GCP project, you can deploy your Encore application directly to this existing project.

## Benefits

Using an existing GCP project allows you to:
- Keep all your infrastructure in a single project
- Maintain existing IAM policies and permissions
- Utilize existing billing settings and quotas
- Consolidate resources for easier management

## Importing a GCP project

Follow these steps to import your existing GCP project:

1. Navigate to **Create Environment** in the [Encore Cloud dashboard](https://app.encore.cloud)
2. Select the GCP cloud provider
3. Choose **Import Project**
4. Add permissions for the Encore Service Account:
   - Copy the `Encore GCP Service Account` from the cloud dashboard
   - Go to your project's IAM page in the GCP Console
   - Grant the `Owner` role to the `Encore GCP Service Account`
5. Return to the Encore Cloud dashboard
6. Enter your `Project ID` 
7. Click the `Resolve` button to validate the project

Once validated, you can create the environment. When you deploy to this environment, Encore Cloud will automatically deploy your application to your imported GCP project rather than provisioning a new one. 