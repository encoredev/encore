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

## Creating a database

To create a database, import `encore.dev/storage/sqldb` and call `new SQLDatabase`, assigning the result to a top-level variable.

For example:

```typescript
-- todo/todo.ts --
import { SQLDatabase } from "encore.dev/storage/sqldb";

// Create the todo database and assign it to the "todoDB" variable
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

With this code in place Encore will automatically create the database when starting `encore run` (locally)
or on the next deployment (in the cloud). Encore automatically injects the appropriate configuration to authenticate
and connect to the database, so once the application starts up the database is ready to be used.

## Defining the database schema

Database schemas are defined by creating *migration files* in a directory named `migrations`
within an Encore service package. Each migration file is named `<number>_<name>.up.sql`, where
`<number>` is a sequence number for ordering the migrations and `<name>` is a
descriptive name of what the migration does.

On disk it might look like this:

```
/my-app
├── encore.app                       // ... and other top-level project files
│
└── todo                             // todo service
    ├── migrations                   // database migrations (directory)
    │   ├── 1_create_table.up.sql    // database migration file
    │   └── 2_add_field.up.sql       // database migration file
    ├── todo.ts                      // todo service code
    └── todo.test.ts                 // tests for todo service
```

Each migration runs in order and expresses the change in the database schema
from the previous migration.

**The file name format is important.** Migration files must start with a number followed by `_`, and increasing for each migration.
Each file name must also end with `.up.sql`.

The simplest naming convention is to start from `1` and counting up:

* `1_first_migration.up.sql`
* `2_second.up.sql`
* `3_migration_name_goes_here.up.sql`
* ... and so on.

It's also possible to prefix the migration files with leading zeroes to have them ordered nicer
in the editor (like `0001_migration.up.sql`).

The first migration usually defines the initial table structure. For example,
a `todo` service might start out by creating `todo/migrations/1_create_table.up.sql` with
the following contents:

```sql
CREATE TABLE todo_item (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    done BOOLEAN NOT NULL DEFAULT false
);
```

## Provisioning databases

Encore automatically provisions databases to match what your application requires.
When you [define a database](#creating-a-database), Encore will provision the database at your next deployment.

Encore provisions databases in an appropriate way depending on the environment.
When running locally, Encore creates a database cluster using [Docker](https://www.docker.com/).
In the cloud, it depends on the [environment type](/docs/deploy/environments#environment-types):

- In `production` environments, the database is provisioned through the Managed SQL Database
  service offered by the chosen cloud provider.
- In `development` environments, the database is provisioned as a Kubernetes deployment
  with a persistent disk attached.

See exactly what is provisioned for each cloud provider, and each environment type, in the [infrastructure documentation](/docs/deploy/infra).

## Using databases

Once you have created the database using `const db = new SQLDatabase(...)` you can start querying and inserting data into the database by calling methods on the `db` variable.

### Querying data

To query data, use the `db.query` or `db.queryRow` methods. `db.query` returns
an asynchronous iterator, yielding rows one by one as they are streamed from the database. `queryRow` returns a single row, or `null` if the query yields no rows.

Both APIs operate using JavaScript template strings, allowing easy use of
placeholder parameters while preventing the possibility of SQL Injection vulnerabilities.

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
  const row = await db.query`SELECT title FROM todo_item WHERE id = ${id}`;
  return row?.title;
}
```

### Inserting data

To insert data, or to make database queryies that don't return any rows, use `db.exec`.

For example:

```ts
await db.exec`
  INSERT INTO todo_item (title, done)
  VALUES (${title}, false)
`;
```

## Connecting to databases

It's often useful to be able to connect to the database from outside the backend application.
For example for scripts, ad-hoc querying, or dumping data for analysis.

### Using the Encore CLI

Encore's CLI comes with built-in support for connecting to databases:

* `encore db shell <database-name> [--env=<name>]` opens a [psql](https://www.postgresql.org/docs/current/app-psql.html)
  shell to the database named `<database-name>` in the given environment. Leaving out `--env` defaults to the local development environment.

* `encore db conn-uri <database-name> [--env=<name>]` outputs a connection string for the database named `<database-name>`.
  When specifying a cloud environment, the connection string is temporary. Leaving out `--env` defaults to the local development environment.

* `encore db proxy [--env=<name>]` sets up a local proxy that forwards any incoming connection
  to the databases in the specified environment.
  Leaving out `--env` defaults to the local development environment.

See `encore help db` for more information on database management commands.

### Using database user credentials

For cloud environments you can view database user credentials (created by Encore when provisioning databases) via the Cloud Dashboard:

* Open your app in the [Cloud Dashboard](https://app.encore.dev), navigate to the **Infrastructure** page for the appropriate environment, and locate the `USERS` section within the relevant **Database Cluster**.

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

## PostgreSQL Extensions

Encore uses the [encoredotdev/postgres](https://github.com/encoredev/postgres-image) docker image for local development,
CI/CD, and for databases hosted on Encore Cloud.

This docker image ships with many popular PostgreSQL extensions pre-installed.
In particular, [pgvector](https://github.com/pgvector/pgvector) and [PostGIS](https://postgis.net) are available.

See [the full list of available extensions](/docs/primitives/databases/extensions).

## Troubleshooting

### Application won't run

When you run your application locally with `encore run`, Encore will parse and compile your application, and provision the necessary infrastructure including databases. If this fails with a database error, there are a few common causes.

** Error: sqldb: unknown database **

This error is often caused by a problem with the initial migration file, such as incorrect naming or location.

- Verify that you've [created the migration file](/docs/ts/primitives/databases#defining-the-database-schema) correctly, then try `encore run` again.

** Error: could not connect to the database **

When you can't connect to the database in your local environment, there's likely an issue with Docker:

- Make sure that you have [Docker](https://docker.com) installed and running, then try `encore run` again.
- If this fails, restart the Encore daemon by running `encore daemon`, then try `encore run` again.

** Error: Creating PostgreSQL database cluster Failed **

This means Encore was not able to create the database. Often this is due to a problem with Docker.

- Check if you have permission to access Docker by running `docker images`.
- Set the correct permissions with `sudo usermod -aG docker $USER` (Learn more in the [Docker documentation](https://docs.docker.com/engine/install/linux-postinstall/))
- Then log out and log back in so that your group membership is refreshed.

** Error: unable to add CA to cert pool **

This error is commonly caused by the presence of the file `$HOME/.postgresql/root.crt` on the filesystem.
When this file is present the PostgreSQL client library will assume the database server has that root certificate,
which will cause the above error.

- Remove or rename the file, then try `encore run` again.
