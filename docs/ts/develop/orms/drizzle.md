---
seotitle: Using Drizzle with Encore
seodesc: Learn how to use Drizzle with Encore to interact with SQL databases.
title: Using Drizzle with Encore
lang: ts
---

Encore supports using [Drizzle](https://orm.drizzle.team/) with TypeScript. Drizzle is a TypeScript ORM for Node.js and the browser.
To use Drizzle with Encore, you need to create a new `SQLDatabase` instance and pass the connection string to Drizzle.

```ts
-- database.ts --
import { api } from "encore.dev/api";
import { SQLDatabase } from "encore.dev/storage/sqldb";
import { drizzle } from "drizzle-orm/node-postgres"
import {users} from "./schema";

const db = new SQLDatabase("test", {
  migrations: {
    path: "migrations",
    source: "drizzle"
  }
})

const orm = drizzle(db.connectionString);

// Query all users
await orm.select().from(users);

-- drizzle.config.ts --
import 'dotenv/config';
import { defineConfig } from 'drizzle-kit';
export default defineConfig({
  out: 'migrations',
  schema: 'schema.ts',
  dialect: 'postgresql',
});

-- schema.ts --
import * as p from "drizzle-orm/pg-core";

export const users = p.pgTable("users", {
  id: p.serial().primaryKey(),
  name: p.text(),
  email: p.text().unique(),
});
```

## Generate migrations
Run `drizzle-kit generate` in the same directory as `drizzle.config.ts` to generate the migrations.

## Apply migrations
Migrations will automatically be applied when running your Encore application. You should not run `drizzle-kit migrate` or similar commands.


<GitHubLink
href="https://github.com/encoredev/examples/tree/main/ts/drizzle"
desc="Using Drizzle ORM with Encore.ts"
/>