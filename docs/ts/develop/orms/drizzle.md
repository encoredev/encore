---
seotitle: Using Drizzle with Encore
seodesc: Learn how to use Drizzle with Encore to interact with SQL databases.
title: Using Drizzle with Encore
lang: ts
---

Here is an example of using [drizzle](https://orm.drizzle.team/) with Encore.ts. We use `db.connectionString` supply the connection string to drizzle:

```ts
-- database.ts --
import { api } from "encore.dev/api";
import { SQLDatabase } from "encore.dev/storage/sqldb";
import { drizzle } from "drizzle-orm/node-postgres"
import {users} from "./schema";

const db = new SQLDatabase("test", {
  migrations: {
    path: "migrations",
    orm: "drizzle"
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
Migrations will automatically be applied when running your Encore application.


<GitHubLink
href="https://github.com/encoredev/examples/tree/main/ts/drizzle"
desc="Using Drizzle ORM with Encore.ts"
/>