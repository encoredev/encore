---
seotitle: Using Sequelize with Encore
seodesc: Learn how to use Sequelize with Encore to interact with SQL databases.
title: Using Sequelize with Encore
lang: ts
---

### Sequelize
Here is an example of using [Knex.js](http://knexjs.org/) with Encore.ts. We use `SiteDB.connectionString` supply the connection string to Knex.js:

```ts
-- database.ts --
import { SQLDatabase } from "encore.dev/storage/sqldb";
import { Sequelize } from "sequelize";

// Define a database named 'encore_sequelize_test', using the database migrations
// in the "./migrations" folder. Encore automatically provisions,
// migrates, and connects to the database.
const DB = new SQLDatabase('encore_sequelize_test', {
  migrations: './migrations',
});

// Query all users
const sequelize = new Sequelize(DB.connectionString);
```

<GitHubLink
href="https://github.com/encoredev/examples/tree/main/ts/sequelize"
desc="Using Sequelize ORM with Encore.ts"
/>