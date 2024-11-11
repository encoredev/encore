---
seotitle: Using Sequelize with Encore
seodesc: Learn how to use Prisma with Encore to interact with SQL databases.
title: Using Prisma with Encore
lang: ts
---

### Prisma

With Prisma you make your schema changes in a prisma schema file `schema.prisma`, and then use prismas cli tools to generate sql migrations and an TypeScript ORM client.

Here is an example of using [Prisma](https://prisma.io/) with Encore.ts. We use `DB.connectionString` supply the connection string to Prisma:

```ts
-- database.ts --
import { SQLDatabase } from "encore.dev/storage/sqldb";
import { PrismaClient } from "@prisma/client";

// Define a database named 'encore_sequelize_test', using the database migrations
// in the "./migrations" folder. Encore automatically provisions,
// migrates, and connects to the database.
const DB = new SQLDatabase('encore_sequelize_test', {
  migrations: {
    path: './prisma/migrations',
    source: 'prisma',
  },
});

// Setup prisma client with connection string
const prisma = new PrismaClient({
  datasources: {
    db: {
      url: DB.connectionString,
    },
  },
});

// Select all users
const allUsers = prisma.user.findMany();

-- prisma/schema.prisma --
model User {
  id      Int      @id @default(autoincrement())
  name    String?
  surname String?
}

```

## Configure Prisma

You can configure Prisma to operate on Encores shadow database, that way Encore.ts and Prisma won't interfere with each other, and you can then use the Prisma CLI to generate migrations and the ORM client. Encore will take care of applying the migrations both in production and locally.

To get the shadow db connection url to Encore.ts shadow database, run:

```
encore db conn-uri <database name> --shadow
```

To initialize Prisma, run the following command from within your service folder:

```
npx prisma init --url <shadow db connection url>
```

## Generate migrations

Run `npx prisma migrate dev` in the same directory as you ran the init (where the prisma folder exist).
This will create new migrations if you have made any changes to the `schema.prisma` file.

## Migration

The migration files will be automatically applied by Encores migration tool both locally (when you run `encore run`) and in production.


<GitHubLink
href="https://github.com/encoredev/examples/tree/main/ts/prisma"
desc="Using Prisma ORM with Encore.ts"
/>
