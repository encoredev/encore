---
seotitle: Using SQL databases for your backend application
seodesc: Learn how to use SQL databases for your backend application. See how to provision, migrate, and query PostgreSQL databases using Go and Encore.
title: Using SQL databases
subtitle: Provisioning, migrating, querying
---

Encore treats SQL databases as logical resources and natively supports **PostreSQL** databases.
To start using a database you only need to [define the schema](#defining-a-database-schema) by creating a migration file. Encore takes care of [provisioning the database](/docs/primitives/databases#provisioning-databases), running new schema migrations during deploys, and connecting to it.

## Defining a database schema

Database schemas are defined by creating *migration files* in a directory named `migrations`
within an Encore service package. Each migration file is named `<number>_<name>.up.sql`, where
`<number>` is a sequence number for ordering the migrations and `<name>` is a
descriptive name of what the migration does.

On disk it might look like this:

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

Each migration runs in order and expresses the change in the database schema
from the previous migration.

**The file name format is important:** Migration files must be sequentially named, starting with `1_` and counting up for each migration. Each file name must also end with `.up.sql`.

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
When you [define a database schema](#defining-a-database-schema), Encore will provision the database at your next deployment.

Encore provisions databases in an appropriate way depending on the environment.
When running locally, Encore creates a database cluster using [Docker](https://www.docker.com/).
In the cloud, it depends on the [environment type](/docs/deploy/environments#environment-types):

- In `production` environments, the database is provisioned through the Managed SQL Database
  service offered by the chosen cloud provider.
- In `development` environments, the database is provisioned as a Kubernetes deployment
  with a persistent disk attached.

See exactly what is provisioned for each cloud provider, and each environment type, in the [infrastructure documentation](/docs/deploy/infra).

## Inserting data into databases

Once you have defined a database schema, you can start inserting data into the database.
Import `encore.dev/storage/sqldb` in your service package (or any sub-packages within the service).
The interface is similar to that of the Go standard library's
`database/sql` package, learn more in the [package docs](https://pkg.go.dev/encore.dev/storage/sqldb).

One way of inserting data is with a helper function that uses the package function `sqldb.Exec`. For example, to insert a single todo item using the example schema above, we can use the following helper function `insert`:

```go
// insert inserts a todo item into the database.
func insert(ctx context.Context, id, title string, done bool) error {
	_, err := sqldb.Exec(ctx, `
		INSERT INTO todo_item (id, title, done)
		VALUES ($1, $2, $3)
	`, id, title, done)
	return err
}
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
err := sqldb.QueryRow(ctx, `
    SELECT id, title, done
    FROM todo_item
    LIMIT 1
`).Scan(&item.ID, &item.Title, &item.Done)
```

If `sqldb.QueryRow` does not find a matching row, it reports an error that can be checked against
by importing the standard library `errors` package and calling `errors.Is(err, sqldb.ErrNoRows)`.

Learn more in the [package docs](https://pkg.go.dev/encore.dev/storage/sqldb).

## Connecting to databases

It's often useful to be able to connect to the database from outside the backend application.
For example for scripts, ad-hoc querying, or dumping data for analysis.

The Encore CLI comes with built-in support for this:

* Use `encore db shell [--env=<name>] <service-name>` to open a [psql](https://www.postgresql.org/docs/current/app-psql.html)
  shell to the database for `<service-name>` in the given environment.
  Leaving out `--env` defaults to the local development environment.

* Use `encore db proxy [--env=<name>]` to create a local proxy that forwards any incoming connection
  to the database in the given environment.
  Leaving out `--env` defaults to the local development environment.

See `encore help db` for more information on database management commands.

## Handling migration errors

When Encore applies database migrations, there's always a possibility the migrations don't apply cleanly.

This can happen for many reasons:
- There's a problem with the SQL syntax in the migration
- You tried to add a `UNIQUE` constraint but the values in the table aren't actually unique
- The existing database schema didn't look like you thought it did, so the database object you tried to change doesn't actually exist
- ... and so on

If that happens, Encore rolls back the migration and reports an error.
Once you've fixed the problem, the migration automatically re-runs
on the next `encore run` (for local development) and next deploy (in cloud environments).

### Tracking migrations

Encore tracks which migrations have been applied using a `schema_migrations` table:

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

When running migrations Encore compares the latest migration in the code base
with the latest applied migration in the `schema_migrations` table, and runs the
ones that haven't been applied yet.

If needed, you can also manually roll back or roll forward migrations.

#### Roll back

To roll back to the previous migration:

1. Use `encore db shell <service-name>` to log in to the database
2. Apply the necessary changes to the schema to revert it back to the previous migration version
3. Execute the query `UPDATE schema_migrations SET version = <target version>;`

#### Roll forward

To roll forward to the new migration:

1. Use `encore db shell <service-name>` to log in to the database
2. Apply the necessary changes to the schema to fix the migration failure and bring the schema up to date with the new migration
3. Execute the query `UPDATE schema_migrations SET version = <target version>;`

## Integration testing

When running tests Encore automatically creates a test-only database cluster that's optimized for
running tests quickly with an in-memory filesystem, and wires up the tests to use that cluster.
This is supported both locally as well as when deploying to the cloud using Encore's CI/CD system.

The test cluster is automatically recreated on every test run.
To inspect the test databases, use `encore db shell --test <database-name>`.
The `--test` flag also works with the other database management commands.

## Troubleshooting

#### Application won't run

When you run your application locally with `encore run`, Encore will parse and compile your application, and provision the necessary infrastructure including databases. If this fails with a database error, there are a few common causes.

** Error: sqldb: unknown database **

This error is often caused by a problem with the initial migration file, such as incorrect naming or location.

- Verify that you've [created the migration file](/docs/develop/databases#defining-a-database-schema) correctly, then try `encore run` again.

** Error: could not connect to the database **

When you can't connect to the database in your local environment, there's likely an issue with Docker:

- Make sure that you have [Docker](https://docker.com) installed and running, then try `encore run` again.
- If this fails, restart the Encore daemon by running `encore daemon`, then try `encore run` again.

** Error: Creating PostgreSQL database cluster Failed **

This means Encore was not able to create the database. Often this is due to a problem with Docker.

- Check if you have permission to access Docker by running `docker images`.
- Set the correct permissions with `sudo usermod -aG docker $USER` (Learn more in the [Docker documentation](https://docs.docker.com/engine/install/linux-postinstall/))
- Then log out and log back in so that your group membership is refreshed.
