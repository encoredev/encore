---
seotitle: How to deploy your Encore application to a new Kubernetes cluster
seodesc: Learn how to automatically deploy your Encore application to a new Kubernetes cluster.
title: Kubernetes deployment
subtitle: Deploying your app to a new Kubernetes cluster
---

When you build your app using Encore's [Infrastructure SDK](/docs/primitives), you can deploy the same code to many different types of [cloud infrastructure](/docs/deploy/infra). Encore will automatically handle provisioning different infrastructure for each environment — depending on your goals. Configuring what type of compute platform you want is done through the [Cloud Dashboard](https://app.encore.dev) rather than in the application code.

If you already have a Kubernetes cluster, you may want to deploy your Encore application into this pre-existing cluster. [See the docs](/docs/how-to/import-kubernetes-cluster) for how to do this.

## Deploying to a new Kubernetes cluster

**1. Connect your cloud account:** Ensure your cloud account (such as Google Cloud Platform or AWS) is connected to Encore. ([See docs](/docs/deploy/own-cloud))

**2. Create environment:** Open your app in the [Cloud Dashboard](https://app.encore.dev) and go to **Environments**, then click on **Create Environment**.  

Next, select your cloud (AWS or GCP) and then specify Kubernetes as the compute platform. Encore supports deploying to GKE on GCP, and EKS Fargate on AWS.

You can also configure if you want to allocate all services in one process or run one process per service.

<img src="/assets/docs/k8s-config.jpg" title="Environment Settings" className="mx-auto"/>

**3. Push your code:** To deploy, commit and push your code to the branch you configured as the deployment trigger. You can also trigger a manual deploy from the Cloud Dashboard by going to the **Environment Overview** page and clicking on **Deploy**.

**4. Automatic deployment by Encore:** Once you've triggered the deploy, Encore will automatically provision and deploy the necessary infrastructure on Kubernetes, per your environment configuration in the Cloud Dashboard. You can monitor the status of your deploy and view your environment's details through the Encore Cloud Dashboard.

**5. Accessing your cluster with kubectl:** You can access your cluster using the `kubectl` CLI tool. [See the docs](/docs/deploy/kubernetes/kubectl) for how to do this.