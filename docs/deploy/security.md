---
seotitle: Security – How Encore keeps your backend application sure
seodesc: Encore applications come with built-in security best practises. See how Encore keeps your application secure by default.
title: Security
subtitle: Industry standard best practices
---

Encore applications are secure by default.

We've carefully designed the framework to make building secure applications
more convenient than insecure ones.

For example, Encore's [secret management](/docs/primitives/secrets) provides
an incredibly easy way of using secret keys, while at the same time providing
state of the art security behind the scenes, backed by HashiCorp Vault.

All communication between Encore and running applications leverage
mutual TLSv1.3 connections, and all database access is similarly encrypted
with certificate validation and strong security credentials.

Production databases provisioned through Encore with major cloud providers
provide managed, daily backups.