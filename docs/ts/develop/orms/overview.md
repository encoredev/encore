---
seotitle: Using ORMs with Encore.ts
seodesc: Learn how to use ORMs with Encore.ts to seamlessly interact with SQL databases from your TypeScript / Node.js backend.
title: Using ORMs and Migration Frameworks with Encore.ts
lang: ts
---
Encore provides built-in support for ORMs and migration frameworks by offering named databases and SQL-based migration files. For developers who prefer not to write raw SQL, Encore allows seamless integration with popular ORMs and migration tools.

## Overview

Encore’s approach to database management is flexible. It uses standard SQL migration files, allowing integration with ORMs like [Sequelize](https://sequelize.org/) and migration tools like [Atlas](https://atlasgo.io/).

- **ORM Compatibility:** If your ORM can connect to a database via a standard SQL driver, it will work with Encore.
- **Migration Tool Compatibility:** If your migration tool generates SQL migration files without additional customization, it can be used with Encore.

## Connecting to a Database

Encore provides the `SQLDatabase` class, which allows you to create a named database and retrieve its connection string. This connection string can be used by your chosen ORM or migration framework to establish a database connection.

Example setup:

```typescript
import { SQLDatabase } from "encore.dev/storage/sqldb";

// Initialize a named database with migration directory
const SiteDB = new SQLDatabase("siteDB", {
  migrations: "./migrations",
});

// Retrieve the connection string for ORM use
const connStr = SiteDB.connectionString;
```

## Example ORM implementations

Here are some guides to using different ORMs with Encore:

- [Using Knex.js with Encore](/docs/ts/develop/orms/knex)
- [Using Sequelize with Encore](/docs/ts/develop/orms/sequelize)
- [Using Drizzle with Encore](/docs/ts/develop/orms/drizzle)
- [Using Prisma with Encore](/docs/ts/develop/orms/prisma)

This setup enables Encore to support a wide variety of ORMs and migration frameworks, making database management both flexible and straightforward.