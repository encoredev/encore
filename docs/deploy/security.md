---
seotitle: Security – How Encore keeps your backend application secure
seodesc: Encore applications come with built-in security best practises. See how Encore keeps your application secure by default.
title: Security
subtitle: Keeping your application secure by default
---

Encore's security practices are informed by our team's decades-long experience working with payments, and other sensitive systems, at Spotify, Google, and Monzo.

This expertise has also been used when designing the Encore infrastructure SDK, ensuring that building secure applications is more convenient than building insecure ones.

For example, Encore's [secrets management](/docs/primitives/secrets) gives you
an incredibly simple way of using secret keys, while at the same time providing
state of the art security behind the scenes, backed by HashiCorp Vault.

When running your application, all communication between Encore and your application uses
mutual TLSv1.3 connections, and all database access is similarly encrypted
with certificate validation and strong security credentials.

For cloud environments, Encore provisions infrastructure using best practises for each of the supported cloud providers (GCP, AWS, Azure). Learn more in the [infrastructure documentation](/docs/deploy/infra).