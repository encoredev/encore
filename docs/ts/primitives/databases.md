---
seotitle: Using SQL databases for your backend application
seodesc: Learn how to use SQL databases for your backend application. See how to provision, migrate, and query PostgreSQL databases using Go and Encore.
title: Using SQL databases
subtitle: Provisioning, migrating, querying
infobox: {
  title: "SQL Databases",
  import: "encore.dev/storage/sqldb",
  example_link: "/docs/tutorials/rest-api"
}
lang: ts
---

Encore treats SQL databases as logical resources and natively supports **PostgreSQL** databases.

<GitHubLink
    href="https://github.com/encoredev/examples/tree/main/ts/url-shortener"
    desc="URL Shortener example that uses a PostgreSQL database."
/>

## Creating a database

To create a database, import `encore.dev/storage/sqldb` and call `new SQLDatabase`, assigning the result to a top-level variable.
Use a migration file in a directory `migrations` to define the database schema.

For example:

```typescript
-- todo/todo.ts --
import { SQLDatabase } from "encore.dev/storage/sqldb";

// Create the todo database and assign it to the "db" variable
const db = new SQLDatabase("todo", {
  migrations: "./migrations",
});

// Then, query the database using db.query, db.exec, etc.
-- todo/migrations/1_create_table.up.sql --
CREATE TABLE todo_item (
  id BIGSERIAL PRIMARY KEY,
  title TEXT NOT NULL,
  done BOOLEAN NOT NULL DEFAULT false
  -- etc...
);
```

As seen above, the `new SQLDatabase()` call takes two parameters: the name of the database, and a configuration object.
The configuration object specifies the directory containing the database migration files, which is how you define the database schema.

See the [Defining the database schema](#defining-the-database-schema) section below for more details.

With this code in place, Encore will automatically create the database using [Docker](https://docker.com) when you run the command `encore run` in your local environment. Make sure Docker is installed and running on your machine before running `encore run`.

<Callout type="info">

If your application is already running when you define a new database, you will need to stop and restart `encore run`. This is necessary for Encore to create the new database using Docker.

</Callout>


In cloud environments, Encore automatically injects the appropriate configuration to authenticate and connect to the database, so once the application starts up the database is ready to be used.

## Database Migrations

Encore automatically handles `up` migrations, while `down` migrations must be run manually. Each `up` migration runs sequentially, expressing changes in the database schema from the previous migration.

### Naming Conventions

**File Name Format:** Migration files must start with a number followed by an underscore (`_`), and must increase sequentially. Each file name must end with `.up.sql`.

**Examples:**
- `1_first_migration.up.sql`
- `2_second_migration.up.sql`
- `3_migration_name.up.sql`

You can also prefix migration files with leading zeroes for better ordering in the editor (e.g., `0001_migration.up.sql`).

### Defining the Database Schema

The first migration typically defines the initial table structure. For instance, a `todo` service might create `todo/migrations/1_create_table.up.sql` with the following content:

```sql
CREATE TABLE todo_item (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    done BOOLEAN NOT NULL DEFAULT false
);
```

### Migration File Structure

Migration files are created in a `migrations` directory within an Encore service. Each file is named `<number>_<name>.up.sql`, where `<number>` is a sequence number for ordering and `<name>` describes the migration.

**Example Directory Structure:**
```
/my-app
├── encore.app                       // ... other top-level project files
│
└── todo                             // todo service
    ├── migrations                   // database migrations (directory)
    │   ├── 1_create_table.up.sql    // first migration file
    │   └── 2_add_field.up.sql       // second migration file
    ├── todo.ts                      // todo service code
    └── todo.test.ts                 // tests for todo service
```

## Using databases

Once you have created the database using `const db = new SQLDatabase(...)` you can start querying and inserting data into the database by calling methods on the `db` variable.

### Querying data

To query data, use the following methods:

- `db.query`: Returns an asynchronous iterator, yielding rows one by one.
- `db.queryRow`: Returns a single row, or `null` if no rows are found.
- `db.queryAll`: Returns an array of all rows.
- `db.rawQuery`: Similar to `db.query`, but takes a raw SQL string and parameters.
- `db.rawQueryRow`: Similar to `db.queryRow`, but takes a raw SQL string and parameters.
- `db.rawQueryAll`: Similar to `db.queryAll`, but takes a raw SQL string and parameters.


Typical usage looks like this:

```ts
const allTodos = await db.query`SELECT * FROM todo_item`;
for await (const todo of allTodos) {
  // Process each todo
}
```

Or to query a single todo item by id:

```ts
async function getTodoTitle(id: number): string | undefined {
  const row = await db.queryRow`SELECT title FROM todo_item WHERE id = ${id}`;
  return row?.title;
}
```

Or to query using raw SQL and parameters:

```ts
async function getTodoTitle(id: number): string | undefined {
  const row = await db.rawQueryRow("SELECT title FROM todo_item WHERE id = $1", id);
  return row?.title;
}
```


### Inserting data

To insert data, or to make database queries that don't return any rows, use `db.exec` or `db.rawExec`.

For example:

```ts
await db.exec`
  INSERT INTO todo_item (title, done)
  VALUES (${title}, false)
`;
```

Or using raw SQL and parameters:

```ts
await db.rawExec(
  "INSERT INTO todo_item (title, done) VALUES ($1, $2)",
  title,
  false
);
```

### Transactions

Transactions allow you to group multiple database operations into a single unit of work. If any operation within the transaction fails, the entire transaction is rolled back, ensuring data consistency.

The transaction type implements `AsyncDisposable`, which automatically rolls back the transaction if it is not explicitly committed or rolled back. This ensures that no open transactions are left accidentally.

For example:

```ts
await using tx = await db.begin();

await db.exec`
  INSERT INTO todo_item (title, done)
  VALUES (${title1}, false)
`;

await db.exec`
  INSERT INTO todo_item (title, done)
  VALUES (${title2}, false)
`;

await tx.commit();
```


## Connecting to databases

It's often useful to be able to connect to the database from outside the backend application. For example for scripts, ad-hoc querying, or dumping data for analysis.

Currently Encore does not expose user credentials for databases in the local environment or for environments on Encore Cloud. You can use a connection string to connect instead, see below.

### Using the Encore CLI

Encore's CLI comes with built-in support for connecting to databases:

* `encore db shell <database-name> [--env=<name>]` opens a [psql](https://www.postgresql.org/docs/current/app-psql.html)
  shell to the database named `<database-name>` in the given environment. Leaving out `--env` defaults to the local development environment. `encore db shell` defaults to read-only permissions. Use `--write`, `--admin` and `--superuser` flags to modify which permissions you connect with.

* `encore db conn-uri <database-name> [--env=<name>]` outputs a connection string for the database named `<database-name>`.
  When specifying a cloud environment, the connection string is temporary. Leaving out `--env` defaults to the local development environment.

* `encore db proxy [--env=<name>]` sets up a local proxy that forwards any incoming connection
  to the databases in the specified environment.
  Leaving out `--env` defaults to the local development environment.

See `encore help db` for more information on database management commands.

### Using database user credentials

For cloud environments on AWS/GCP you can view database user credentials (created by Encore when provisioning databases) via the Cloud Dashboard:

* Open your app in the [Encore Cloud dashboard](https://app.encore.cloud), navigate to the **Infrastructure** page for the appropriate environment, and locate the `USERS` section within the relevant **Database Cluster**.

## Handling migration errors

When Encore applies database migrations, there's always a possibility the migrations don't apply cleanly.

This can happen for many reasons:
- There's a problem with the SQL syntax in the migration
- You tried to add a `UNIQUE` constraint but the values in the table aren't actually unique
- The existing database schema didn't look like you thought it did, so the database object you tried to change doesn't actually exist
- ... and so on

If that happens, Encore rolls back the migration. If it happens during a cloud deployment, the deployment is aborted.
Once you fix the problem, re-run `encore run` (locally) or push the updated code (in the cloud) to try again.

Encore tracks which migrations have been applied in the `schema_migrations` table:

```sql
database=# \d schema_migrations
          Table "public.schema_migrations"
 Column  |  Type   | Collation | Nullable | Default
---------+---------+-----------+----------+---------
 version | bigint  |           | not null |
 dirty   | boolean |           | not null |
Indexes:
    "schema_migrations_pkey" PRIMARY KEY, btree (version)
```

The `version` column tracks which migration was last applied. If you wish to skip a migration or re-run a migration,
change the value in this column. For example, to re-run the last migration, run `UPDATE schema_migrations SET version = version - 1;`.
*Note that Encore does not use the `dirty` flag by default.*

## Using an ORM

Encore has all the tools needed to support ORMs and migration frameworks out-of-the-box through named databases and migration files. Writing plain SQL might not work for your use case, or you may not want to use SQL in the first place.

ORMs like [Prisma](/docs/ts/develop/orms/prisma) and [Drizzle](/docs/ts/develop/orms/drizzle) can be used with Encore by integrating their logic with a system's database. Encore is not restrictive, it uses plain SQL migration files for its migrations.

* If your ORM of choice can connect to any database using a standard SQL driver, then it can be used with Encore.
* If your migration framework can generate SQL migration files without any modifications, then it can be used with Encore.

For more information on using ORMs with Encore, see the [ORMs](/docs/ts/develop/orms) page.

## Sharing databases between services

There are two primary ways of sharing a database between services:

- You can define the `SQLDatabase` object in a shared module as an exported variable, and reference this object
from every service that needs to access the database.
- You can define the `SQLDatabase` object in one service using `new SQLDatabase("name", ...)`, and have other services access it by creating a reference using `SQLDatabase.named("name")`.

Both approaches have the same effect, but the latter is more explicit.

## PostgreSQL Extensions

Encore uses the [encoredotdev/postgres](https://github.com/encoredev/postgres-image) docker image for local development,
CI/CD, and for databases hosted on Encore Cloud.

This docker image ships with many popular PostgreSQL extensions pre-installed.
In particular, [pgvector](https://github.com/pgvector/pgvector) and [PostGIS](https://postgis.net) are available.

See [the full list of available extensions](/docs/ts/primitives/databases-extensions).

## Troubleshooting

When you run your application locally with `encore run`, Encore will provision local databases using Docker.
If this fails with a database error, it can often be resolved if you restart the Encore daemon using `encore daemon` and then try `encore run` again.

If this does not resolve the issue, here are steps to resolve common errors:

**Error: sqldb: unknown database**

This error is often caused by a problem with the initial migration file, such as incorrect naming or location.

- Verify that you've [created the migration file](#defining-the-database-schema) correctly, then try `encore run` again.

**Error: could not connect to the database**

When you can't connect to the database in your local environment, there's likely an issue with Docker:

- Make sure that you have [Docker](https://docker.com) installed and running, then try `encore run` again.
- If this fails, restart the Encore daemon by running `encore daemon`, then try `encore run` again.

**Error: Creating PostgreSQL database cluster Failed**

This means Encore was not able to create the database. Often this is due to a problem with Docker.

- Check if you have permission to access Docker by running `docker images`.
- Set the correct permissions with `sudo usermod -aG docker $USER` (Learn more in the [Docker documentation](https://docs.docker.com/engine/install/linux-postinstall/))
- Then log out and log back in so that your group membership is refreshed.

**Error: unable to save docker image**

This error is often caused by a problem with Docker.

- Make sure that you have [Docker](https://docker.com) installed and running.
- In Docker, open **Settings > Advanced** and make sure that the setting `Allow the default Docker socket to be used` is checked.
- If it still fails, restart the Encore daemon by running `encore daemon`, then try `encore run` again.

**Error: unable to add CA to cert pool**

This error is commonly caused by the presence of the file `$HOME/.postgresql/root.crt` on the filesystem.
When this file is present the PostgreSQL client library will assume the database server has that root certificate,
which will cause the above error.

- Remove or rename the file, then try `encore run` again.
