---
seotitle: Security – How Encore keeps your backend application secure
seodesc: Encore applications come with built-in security best practises. See how Encore keeps your application secure by default.
title: Application Security
subtitle: Encore makes strong security the default path
---

The security practices implemented by Encore are informed by our team's decades-long experience working with banking, payments, and other sensitive systems, at companies like Google, Spotify, and Monzo.

We have designed Encore to make building secure applications an effortless task, rather than an inconvenience, allowing you to focus on functionality instead of laborious security concerns.

For example, Encore's [secrets management](/docs/primitives/secrets) gives you a simple way of using secret keys, while at the same time providing state of the art security behind the scenes.

Furthermore, thanks to the Backend SDK, Encore understands which services require access to specific resources. **Encore automatically manages IAM policies** based on the _principle of least privilege_ by default. This ensures each service only has the minimum necessary permissions.

When your application is running, all communication to Encore uses mutual TLSv1.3 connections, and all database access is encrypted with certificate validation and strong security credentials.

For cloud environments, Encore automatically provisions infrastructure using security best practises for each of the supported cloud providers (GCP, AWS). Learn more in the [infrastructure documentation](/docs/deploy/infra).
