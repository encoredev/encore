---
seotitle: Managing database user credentials
seodesc: Learn how to manage user credentials for databases created by Encore.
title: Managing database user credentials
---

Encore provisions your databases automatically, meaning you don't need to manually create database users. However, in some use cases you need access to the database user credentials, so Encore makes it simple to view them.

As an application **Admin**, open the [Cloud Dashboard](https://app.encore.dev) and go to the **Infrastructure** page for the relevant environment.

In the section for the relevant **Database Cluster**, you will find a **Users** sub-section which lists your database users. Click on the "eye" icon next to each username to reveal the password.

<img src="/assets/docs/db-user.png" title="View Database User Credentials"/>

<Callout type="important">

Do not change or remove the database users created by Encore, as this will prevent Encore from maintaining and handling connections to the databases in your application.

</Callout>
