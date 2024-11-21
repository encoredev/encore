---
seotitle: Using Drizzle with Encore
seodesc: Learn how to use Drizzle with Encore to interact with SQL databases.
title: Using Drizzle ORM with Encore
lang: ts
---
Encore.ts supports integrating [Drizzle](https://orm.drizzle.team/), a TypeScript ORM for Node.js and the browser. To use Drizzle with Encore, start by creating a `SQLDatabase` instance and providing the connection string to Drizzle.
 
<GitHubLink href="https://github.com/encoredev/examples/tree/main/ts/drizzle" desc="Using Drizzle ORM with Encore.ts" />

## 1. Setting Up the Database Connection

In `database.ts`, initialize the `SQLDatabase` and configure Drizzle:

```typescript
// database.ts
import { api } from "encore.dev/api";
import { SQLDatabase } from "encore.dev/storage/sqldb";
import { drizzle } from "drizzle-orm/node-postgres";
import { users } from "./schema";

// Create SQLDatabase instance with migrations configuration
const db = new SQLDatabase("test", {
  migrations: {
    path: "migrations",
    source: "drizzle",
  },
});

// Initialize Drizzle ORM with the connection string
const orm = drizzle(db.connectionString);

// Query all users
await orm.select().from(users);
```

## 2. Configuring Drizzle

Create a Drizzle configuration file `drizzle.config.ts` to specify settings like migration output, schema, and database dialect:

```typescript
// drizzle.config.ts
import 'dotenv/config';
import { defineConfig } from 'drizzle-kit';

export default defineConfig({
  out: 'migrations',
  schema: 'schema.ts',
  dialect: 'postgresql',
});
```

## 3. Defining the Database Schema

Define your database tables in `schema.ts` using Drizzle's `pg-core` module:

```typescript
// schema.ts
import * as p from "drizzle-orm/pg-core";

export const users = p.pgTable("users", {
  id: p.serial().primaryKey(),
  name: p.text(),
  email: p.text().unique(),
});
```

## 4. Generating Migrations

Run the following command in the directory containing `drizzle.config.ts` to generate migrations:

```bash
drizzle-kit generate
```

## 5. Applying Migrations

Migrations are automatically applied when you run your Encore application, so you don’t need to run `drizzle-kit migrate` or any similar commands manually.
