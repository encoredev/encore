---
seotitle: How to configure a static outbound IP for GCP Cloud Run
seodesc: Learn how to give your Encore application on GCP Cloud Run a stable outbound IP address using Cloud NAT and VPC egress.
title: Configure a static outbound IP
subtitle: Giving your Cloud Run services a stable egress IP address
lang: platform
---

# Overview

By default, Cloud Run services send outbound traffic to the internet using a pool of Google-owned IP addresses that changes over time. This makes it impossible for a third party to allowlist your application's traffic.

To get a stable outbound IP address, you route your services' egress traffic through your environment's VPC and out via a [Cloud NAT][gcp-cloud-nat] gateway that uses a reserved static IP address.

This requires two steps:

1. Create a Cloud NAT gateway with a reserved static IP for the Encore VPC, using the GCP Console.
2. Configure your Cloud Run services to route **All traffic** through the VPC, using the Encore Cloud dashboard.

## Benefits

A static outbound IP allows you to:
- Have partners and third-party APIs allowlist your application's IP address
- Connect to external databases and services that require IP-based access control
- Meet compliance requirements that mandate known, auditable egress addresses

## Step 1: Create a Cloud NAT gateway

Encore Cloud provisions a dedicated GCP project and VPC network for each environment. The Cloud NAT gateway needs to be created in that project, in the same region as your environment.

First, reserve a static external IP address:

1. In the [GCP Console](https://console.cloud.google.com), select the GCP project for your Encore environment
2. Go to **VPC network > IP addresses** and click **Reserve external static IP address**
3. Give it a name (e.g. `encore-egress-ip`)
4. Set **Type** to `Regional` and select the region your environment is deployed in
5. Click **Reserve**

Then create the Cloud NAT gateway:

1. Go to **Network services > Cloud NAT** and click **Get started** / **Create Cloud NAT gateway**
2. Give the gateway a name (e.g. `encore-egress-nat`)
3. Set **NAT type** to `Public`
4. Select the VPC **Network** that Encore Cloud provisioned for the environment, and the same **Region** as above
5. For **Cloud Router**, create a new router (or select an existing one in the same network and region)
6. Under **Cloud NAT mapping**, keep the default source selection so that all subnet IP ranges are able to use the gateway
7. Set **NAT IP addresses** to `Manual` and select the static IP address you reserved
8. Click **Create**

<Callout type="info">

Create the Cloud NAT gateway *before* changing the VPC egress setting in the next step. Once egress is routed through the VPC, outbound internet traffic depends on the NAT gateway — without it, your services will not be able to reach the internet.

</Callout>

## Step 2: Route all Cloud Run traffic through the VPC

By default, Cloud Run only sends traffic destined for private IP ranges through the VPC, and sends internet-bound traffic directly out via Google's IP pool. To make internet-bound traffic use the NAT gateway, you need to change the VPC egress setting:

1. Open your environment in the [Encore Cloud dashboard](https://app.encore.dev)
2. Go to the **Infrastructure** tab
3. Find the Cloud Run configuration and set **VPC egress** to `All traffic`
4. Apply the change

Infrastructure changes made in the dashboard are applied as part of a deployment, so trigger a new deploy (or push a commit) for the setting to take effect. See [Managing Infrastructure](/docs/platform/infrastructure/managing-infrastructure#deployment-phases) for details on the deployment phases.

## Verifying the configuration

Once deployed, make an outbound request from one of your services to a service that echoes back the caller's IP address (for example `https://api.ipify.org`) and confirm that the returned address matches the static IP you reserved.

## Things to keep in mind

- **All outbound traffic is affected.** With `All traffic` egress, every outbound request — including calls to public APIs — goes through the VPC and the NAT gateway. This adds NAT data processing charges, and the NAT gateway becomes a dependency for all internet access from your services.
- **One gateway per region and network.** Cloud NAT is regional. If an environment spans multiple regions, create a gateway with a reserved IP in each region.
- **Multiple IPs for high volume.** A single NAT IP supports a limited number of concurrent connections to the same destination. If your application makes a large number of concurrent outbound connections, assign several reserved static IPs to the gateway and allowlist all of them.
- **Manual changes are preserved.** Encore Cloud does not overwrite resources it doesn't manage, so the Cloud NAT gateway and reserved IP you create in the console will be left untouched by future deployments. See [Infrastructure Configuration](/docs/platform/infrastructure/configuration#manual-configuration-in-your-cloud-providers-console).

<Callout type="info">

If your environment runs on GKE instead of Cloud Run, the VPC egress setting doesn't apply — pods already egress through the VPC network. In that case, check whether a Cloud NAT gateway already exists for the network and attach a reserved static IP address to it rather than creating a second gateway.

</Callout>

[gcp-cloud-nat]: https://cloud.google.com/nat/docs/overview
