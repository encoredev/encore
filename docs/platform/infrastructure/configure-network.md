---
seotitle: How to configure custom network settings for your Encore environment
seodesc: Learn how to configure IP ranges when connecting your Encore application to existing networks.
title: Configure network settings
subtitle: Customizing IP ranges for network peering
lang: platform
---

# Overview

When deploying applications with Encore Cloud, a network is automatically provisioned with default settings. However, if you plan to peer your Encore network with an existing network, you can manually configure the IP range for your environment.

## Benefits

Configuring custom network settings allows you to:
- Connect your Encore application to existing networks via peering
- Prevent IP range conflicts with other networks in your organization
- Plan your network topology with predictable addressing

## Configuring network settings

Follow these steps to configure custom network settings:

1. Navigate to **Create Environment** in the Encore Cloud dashboard
2. Select the AWS or GCP cloud provider
3. Expand the **Network** section
4. Enter your desired IP range
   - The range must be at least a /16 block to reserve enough IPs for your application to grow
   - Choose a range that doesn't conflict with your existing networks

Once configured, Encore will use your specified IP range instead of assigning a random private network.

## Default network behavior

By default, Encore will reserve a randomly assigned /16 block in one of the private IP ranges. This is suitable for most deployments that don't require network peering.
