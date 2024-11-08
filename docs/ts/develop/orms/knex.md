---
seotitle: Using Knex.js with Encore
seodesc: Learn how to use Knex.js with Encore to interact with SQL databases.
title: Using Knex.js with Encore
lang: ts
---

Here is an example of using [Knex.js](http://knexjs.org/) with Encore.ts. We use `SiteDB.connectionString` supply the connection string to Knex.js:

```ts
-- site.ts --
import { SQLDatabase } from "encore.dev/storage/sqldb";
import knex from "knex";

const SiteDB = new SQLDatabase("siteDB", {
  migrations: "./migrations",
});

const orm = knex({
  client: "pg",
  connection: SiteDB.connectionString,
});

export interface Site {
  id: number;
  url: string;
}

// Create a query builder for the "site" table
const Sites = () => orm<Site>("site");

// Query all sites
await Sites().select();

// Query a site by id
await Sites().where("id", id).first();

// Insert a new site
await Sites().insert({ url: params.url })
-- migrations/1_create_table.up.sql --
CREATE TABLE site (
    id SERIAL PRIMARY KEY,
    url TEXT NOT NULL UNIQUE
);
```