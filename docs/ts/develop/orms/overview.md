---
seotitle: Using ORMs with Encore
seodesc: Learn how to use ORMs with Encore to interact with SQL databases.
title: Using ORMs with Encore
lang: ts
---

Encore has all the tools needed to support ORMs and migration frameworks out-of-the-box through named databases and migration files. Writing plain SQL might not work for your use case, or you may not want to use SQL in the first place.

ORMs like [Sequelize](https://sequelize.org/) and migration frameworks like [Atlas](https://atlasgo.io/) can be used with Encore by integrating their logic with a system's database. Encore is not restrictive, it uses plain SQL migration files for its migrations.

* If your ORM of choice can connect to any database using a standard SQL driver, then it can be used with Encore.
* If your migration framework can generate SQL migration files without any modifications, then it can be used with Encore.

Here are some examples of using ORMs with Encore:
* [Using Knex.js with Encore](/docs/ts/develop/orms/knex)
* [Using Sequelize with Encore](/docs/ts/develop/orms/sequelize)
* [Using Drizzle with Encore](/docs/ts/develop/orms/drizzle)
* [Using Prisma with Encore](/docs/ts/develop/orms/prisma)