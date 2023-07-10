---
seotitle: How to deploy your Encore application to an existing Kubernetes cluster
seodesc: Learn how to easily import your existing Kubernetes cluster and deploy your Encore application into it.
title: Import an existing Kubernetes cluster
subtitle: Deploying to your pre-existing cluster instead of provisioning a new one
---

When you deploy your application to your own cloud, Encore can provision infrastructure for it in many different ways – including setting up a Kubernetes cluster.

However, if you already have a Kubernetes cluster, you may want to deploy your Encore application into this pre-existing cluster. This is often useful if you want to integrate your Encore application with other parts of your system that are not built using Encore.

To support this use case, Encore enables you to import existing Kubernetes clusters on Google Cloud Platform (AWS coming soon).

## Importing a cluster

To import your cluster, go to **Create Environment** in the [Cloud Dashboard](https://app.encore.dev), select **Kubernetes: Existing GKE Cluster** as the compute platform, and then specify your cluster's `Project ID`, `Region`, and `Cluster Name`.

When you deploy to this environment, Encore will use your imported cluster as the compute instance.

<img src="/assets/docs/import-k8s.png" title="Import your existing Kubernetes cluster" className="mx-auto"/>
