---
seotitle: Using SQL databases for your backend application
seodesc: Learn how to use SQL databases for your backend application. See how to provision, migrate, and query PostgreSQL databases using Go and Encore.
title: Using SQL databases
subtitle: Provisioning, migrating, querying
infobox: {
  title: "SQL Databases",
  import: "encore.dev/storage/sqldb",
  example_link: "/docs/tutorials/uptime"
}
lang: go
---

Encore treats SQL databases as logical resources and natively supports **PostgreSQL** databases.

## Creating a database

To create a database, import `encore.dev/storage/sqldb` and call `sqldb.NewDatabase`, assigning the result to a package-level variable.
Databases must be created from within an [Encore service](/docs/go/primitives/services).

For example:

```
-- todo/db.go --
package todo

// Create the todo database and assign it to the "tododb" variable
var tododb = sqldb.NewDatabase("todo", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// Then, query the database using db.QueryRow, db.Exec, etc.
-- todo/migrations/1_create_table.up.sql --
CREATE TABLE todo_item (
  id BIGSERIAL PRIMARY KEY,
  title TEXT NOT NULL,
  done BOOLEAN NOT NULL DEFAULT false
  -- etc...
);
```

As seen above, the `sqldb.DatabaseConfig` specifies the directory containing the database migration files, which is how you define the database schema.
See the [Defining the database schema](#defining-the-database-schema) section below for more details.

With this code in place, Encore will automatically create the database using [Docker](https://docker.com) when you run the command `encore run` in your local environment. Make sure Docker is installed and running on your machine before running `encore run`.

<Callout type="info">

If your application is already running when you define a new database, you will need to stop and restart `encore run`. This is necessary for Encore to create the new database using Docker.

</Callout>

<GitHubLink
    href="https://github.com/encoredev/examples/tree/main/sql-database"
    desc="Simple PostgreSQL example application."
/>

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

Migration files are created in a `migrations` directory within an Encore service package. Each file is named `<number>_<name>.up.sql`, where `<number>` is a sequence number for ordering and `<name>` describes the migration.

**Example Directory Structure:**

```
/my-app
├── encore.app                       // ... and other top-level project files
│
└── todo                             // todo service (a Go package)
    ├── migrations                   // todo service db migrations (directory)
    │   ├── 1_create_table.up.sql    // todo service db migration
    │   └── 2_add_field.up.sql       // todo service db migration
    ├── todo.go                      // todo service code
    └── todo_test.go                 // tests for todo service
```

## Inserting data into databases

Once you have created the database using `var mydb = sqldb.NewDatabase(...)` you can start inserting data into the database
by calling methods on the `mydb` variable.

The interface is similar to that of the Go standard library's `database/sql` package.
Learn more in the [package docs](https://pkg.go.dev/encore.dev/storage/sqldb).

One way of inserting data is with a helper function that uses the package function `sqldb.Exec`.
For example, to insert a single todo item using the example schema above, we can use the following helper function `insert`:

```
-- todo/insert.go --
// insert inserts a todo item into the database.
func insert(ctx context.Context, id, title string, done bool) error {
	_, err := tododb.Exec(ctx, `
		INSERT INTO todo_item (id, title, done)
		VALUES ($1, $2, $3)
	`, id, title, done)
	return err
}
-- todo/db.go --
package todo

// Create the todo database and assign it to the "tododb" variable
var tododb = sqldb.NewDatabase("todo", sqldb.DatabaseConfig{
  Migrations: "./migrations",
})

// Then, query the database using db.QueryRow, db.Exec, etc.
-- todo/migrations/1_create_table.up.sql --
CREATE TABLE todo_item (
  id BIGSERIAL PRIMARY KEY,
  title TEXT NOT NULL,
  done BOOLEAN NOT NULL DEFAULT false
  -- etc...
);
```

## Querying databases

To query a database in your application, you similarly need to import `encore.dev/storage/sqldb` in your service package or sub-package.

For example, to read a single todo item in the example schema above, we can use `sqldb.QueryRow`:

```go
var item struct {
    ID int64
    Title string
    Done bool
}
err := tododb.QueryRow(ctx, `
    SELECT id, title, done
    FROM todo_item
    LIMIT 1
`).Scan(&item.ID, &item.Title, &item.Done)
```

If `QueryRow` does not find a matching row, it reports an error that can be checked against
by importing the standard library `errors` package and calling `errors.Is(err, sqldb.ErrNoRows)`.

Learn more in the [package docs](https://pkg.go.dev/encore.dev/storage/sqldb).

## Provisioning databases

Encore automatically provisions databases to match what your application requires.
When you [define a database](#creating-a-database), Encore will provision the database at your next deployment.

Encore provisions databases in an appropriate way depending on the environment.
When running locally, Encore creates a database cluster using [Docker](https://www.docker.com/).
In the cloud, it depends on the [environment type](/docs/platform/deploy/environments#environment-types):

- In `production` environments, the database is provisioned through the Managed SQL Database
  service offered by the chosen cloud provider.
- In `development` environments, the database is provisioned as a Kubernetes deployment
  with a persistent disk attached.

See exactly what is provisioned for each cloud provider, and each environment type, in the [infrastructure documentation](/docs/platform/infrastructure/infra).

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

For cloud environments on AWS/GCP you can view database user credentials (created by Encore when provisioning databases) via the Encore Cloud dashboard:

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
